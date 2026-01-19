package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// HTTPTransport はHTTPベースのトランスポート
type HTTPTransport struct {
	url       string
	headers   map[string]string
	sessionID string
	useSSE    bool

	client     *http.Client
	sseResp    *http.Response
	sseReader  *bufio.Reader
	msgChan    chan *Message
	cancelFunc context.CancelFunc

	mu     sync.Mutex
	closed bool
}

// NewHTTPTransport は新しいHTTPTransportを作成する
func NewHTTPTransport(config *ServerConfig) *HTTPTransport {
	return &HTTPTransport{
		url:     config.URL,
		headers: config.Headers,
		useSSE:  config.Type == TransportSSE,
		client:  &http.Client{},
		msgChan: make(chan *Message, 100),
		closed:  true,
	}
}

// Connect はHTTP接続を確立する
func (t *HTTPTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.closed {
		return errors.New("already connected")
	}

	t.closed = false

	// SSEモードの場合、SSE接続を開始
	if t.useSSE {
		sseCtx, cancel := context.WithCancel(context.Background())
		t.cancelFunc = cancel

		go t.startSSE(sseCtx)
	}

	return nil
}

// startSSE はSSE接続を開始する
func (t *HTTPTransport) startSSE(ctx context.Context) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.url, nil)
	if err != nil {
		return
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return
	}

	t.mu.Lock()
	t.sseResp = resp
	t.sseReader = bufio.NewReader(resp.Body)
	t.mu.Unlock()

	// SSEイベントを読み取るループ
	for {
		select {
		case <-ctx.Done():
			resp.Body.Close()
			return
		default:
		}

		line, err := t.sseReader.ReadString('\n')
		if err != nil {
			// EOFまたはその他のエラーで終了
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// SSEイベントの解析
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			var msg Message
			if err := json.Unmarshal([]byte(data), &msg); err != nil {
				continue
			}

			select {
			case t.msgChan <- &msg:
			default:
				// チャネルが満杯の場合は破棄
			}
		}
	}
}

// Send はメッセージを送信する
func (t *HTTPTransport) Send(msg *Message) error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return errors.New("not connected")
	}
	t.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, t.url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-06-18")

	// カスタムヘッダーを追加
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	// セッションIDがあれば追加
	t.mu.Lock()
	if t.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", t.sessionID)
	}
	t.mu.Unlock()

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// セッションIDを保存
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		t.mu.Lock()
		t.sessionID = sid
		t.mu.Unlock()
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	// レスポンスを解析してチャネルに送信
	var respMsg Message
	if err := json.NewDecoder(resp.Body).Decode(&respMsg); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	select {
	case t.msgChan <- &respMsg:
	default:
		// チャネルが満杯の場合は破棄（通常起こらない）
	}

	return nil
}

// Receive はメッセージを受信する
func (t *HTTPTransport) Receive() (*Message, error) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil, errors.New("not connected")
	}
	t.mu.Unlock()

	msg, ok := <-t.msgChan
	if !ok {
		return nil, io.EOF
	}

	return msg, nil
}

// Close は接続を閉じる
func (t *HTTPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	t.closed = true

	if t.cancelFunc != nil {
		t.cancelFunc()
	}

	if t.sseResp != nil {
		t.sseResp.Body.Close()
	}

	close(t.msgChan)

	return nil
}

// IsConnected は接続状態を返す
func (t *HTTPTransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return !t.closed
}
