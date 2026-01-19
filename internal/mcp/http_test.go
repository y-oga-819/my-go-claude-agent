package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewHTTPTransport(t *testing.T) {
	config := &ServerConfig{
		Type:    TransportHTTP,
		URL:     "http://localhost:8080/mcp",
		Headers: map[string]string{"Authorization": "Bearer token"},
	}

	transport := NewHTTPTransport(config)
	if transport == nil {
		t.Fatal("NewHTTPTransport returned nil")
	}
	if transport.url != "http://localhost:8080/mcp" {
		t.Errorf("url = %q, want %q", transport.url, "http://localhost:8080/mcp")
	}
	if transport.headers["Authorization"] != "Bearer token" {
		t.Errorf("headers[Authorization] = %q, want %q", transport.headers["Authorization"], "Bearer token")
	}
}

func TestHTTPTransport_ConnectAndClose(t *testing.T) {
	// テスト用HTTPサーバー
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := &ServerConfig{
		Type: TransportHTTP,
		URL:  server.URL,
	}

	transport := NewHTTPTransport(config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := transport.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if !transport.IsConnected() {
		t.Error("should be connected")
	}

	err = transport.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if transport.IsConnected() {
		t.Error("should be disconnected after Close")
	}
}

func TestHTTPTransport_SendAndReceive(t *testing.T) {
	var mu sync.Mutex
	var receivedReq *Message

	// テスト用HTTPサーバー
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// リクエストを解析
		var req Message
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		mu.Lock()
		receivedReq = &req
		mu.Unlock()

		// レスポンスを返す
		resp := &Message{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"status": "ok",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := &ServerConfig{
		Type: TransportHTTP,
		URL:  server.URL,
	}

	transport := NewHTTPTransport(config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := transport.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer transport.Close()

	// メッセージ送信
	msg := &Message{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test",
		Params:  map[string]any{"key": "value"},
	}

	err = transport.Send(msg)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// レスポンス受信
	resp, err := transport.Receive()
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}

	if resp.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %q, want %q", resp.JSONRPC, "2.0")
	}

	// サーバーが受信したリクエストを確認
	mu.Lock()
	if receivedReq == nil {
		t.Error("server did not receive request")
	} else if receivedReq.Method != "test" {
		t.Errorf("receivedReq.Method = %q, want %q", receivedReq.Method, "test")
	}
	mu.Unlock()
}

func TestHTTPTransport_SSE(t *testing.T) {
	// SSEストリームを返すサーバー
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Accept ヘッダーをチェック
		accept := r.Header.Get("Accept")
		if strings.Contains(accept, "text/event-stream") && r.Method == http.MethodGet {
			// SSEストリームを開始
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.WriteHeader(http.StatusOK)

			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "streaming not supported", http.StatusInternalServerError)
				return
			}

			// イベントを送信
			msg := &Message{
				JSONRPC: "2.0",
				Method:  "notification/test",
			}
			data, _ := json.Marshal(msg)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			return
		}

		// 通常のPOSTリクエスト
		if r.Method == http.MethodPost {
			var req Message
			json.NewDecoder(r.Body).Decode(&req)

			resp := &Message{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  map[string]any{"status": "ok"},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
	}))
	defer server.Close()

	config := &ServerConfig{
		Type: TransportSSE,
		URL:  server.URL,
	}

	transport := NewHTTPTransport(config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := transport.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer transport.Close()

	// SSEからの通知を受信（タイムアウト付き）
	receiveCh := make(chan *Message, 1)
	errCh := make(chan error, 1)

	go func() {
		msg, err := transport.Receive()
		if err != nil {
			errCh <- err
			return
		}
		receiveCh <- msg
	}()

	select {
	case msg := <-receiveCh:
		if msg.Method != "notification/test" {
			t.Errorf("Method = %q, want %q", msg.Method, "notification/test")
		}
	case err := <-errCh:
		t.Fatalf("Receive failed: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for SSE message")
	}
}

func TestHTTPTransport_Headers(t *testing.T) {
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(&Message{JSONRPC: "2.0", ID: 1, Result: map[string]any{}})
	}))
	defer server.Close()

	config := &ServerConfig{
		Type: TransportHTTP,
		URL:  server.URL,
		Headers: map[string]string{
			"Authorization": "Bearer test-token",
			"X-Custom":      "custom-value",
		},
	}

	transport := NewHTTPTransport(config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer transport.Close()

	if err := transport.Send(&Message{JSONRPC: "2.0", ID: 1, Method: "test"}); err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if _, err := transport.Receive(); err != nil {
		t.Fatalf("Receive failed: %v", err)
	}

	if receivedHeaders.Get("Authorization") != "Bearer test-token" {
		t.Errorf("Authorization = %q, want %q", receivedHeaders.Get("Authorization"), "Bearer test-token")
	}
	if receivedHeaders.Get("X-Custom") != "custom-value" {
		t.Errorf("X-Custom = %q, want %q", receivedHeaders.Get("X-Custom"), "custom-value")
	}
}

func TestHTTPTransport_SendBeforeConnect(t *testing.T) {
	config := &ServerConfig{
		Type: TransportHTTP,
		URL:  "http://localhost:8080",
	}

	transport := NewHTTPTransport(config)

	msg := &Message{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test",
	}

	err := transport.Send(msg)
	if err == nil {
		t.Error("expected error when sending before connect")
	}
}
