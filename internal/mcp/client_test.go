package mcp

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"
)

// mockTransport はテスト用のモックトランスポート
// リクエストが送信されたら対応するレスポンスを返す
type mockTransport struct {
	mu         sync.Mutex
	connected  bool
	closed     bool
	sent       []*Message
	responses  map[int64]*Message // ID -> response
	responseCh chan *Message
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		responses:  make(map[int64]*Message),
		responseCh: make(chan *Message, 100),
	}
}

func (m *mockTransport) Connect(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = true
	m.closed = false
	return nil
}

func (m *mockTransport) Send(msg *Message) error {
	m.mu.Lock()
	m.sent = append(m.sent, msg)
	// リクエストに対応するレスポンスを探す
	id := normalizeIDForTest(msg.ID)
	resp, ok := m.responses[id]
	m.mu.Unlock()

	if ok && resp != nil {
		// レスポンスをチャネルに送信
		m.responseCh <- resp
	}
	return nil
}

func normalizeIDForTest(id any) int64 {
	switch v := id.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return i
		}
	}
	return 0
}

func (m *mockTransport) Receive() (*Message, error) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, io.EOF
	}
	m.mu.Unlock()

	select {
	case msg := <-m.responseCh:
		return msg, nil
	case <-time.After(10 * time.Second):
		return nil, context.DeadlineExceeded
	}
}

func (m *mockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = false
	m.closed = true
	return nil
}

func (m *mockTransport) IsConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.connected
}

func (m *mockTransport) setResponse(id int64, msg *Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[id] = msg
}

func (m *mockTransport) getSentMessages() []*Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*Message, len(m.sent))
	copy(result, m.sent)
	return result
}

func TestNewMCPClient(t *testing.T) {
	transport := newMockTransport()
	client := NewMCPClient("test", transport)
	if client == nil {
		t.Fatal("NewMCPClient returned nil")
	}
	if client.name != "test" {
		t.Errorf("name = %q, want %q", client.name, "test")
	}
}

func TestMCPClient_Connect(t *testing.T) {
	transport := newMockTransport()
	client := NewMCPClient("test", transport)

	// initialize responseを準備（ID=1）
	transport.setResponse(1, &Message{
		JSONRPC: "2.0",
		ID:      json.Number("1"),
		Result: map[string]any{
			"protocolVersion": "2025-06-18",
			"serverInfo": map[string]any{
				"name":    "test-server",
				"version": "1.0.0",
			},
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// initialize requestが送信されたことを確認
	sent := transport.getSentMessages()
	if len(sent) < 1 {
		t.Fatal("no messages sent")
	}

	initReq := sent[0]
	if initReq.Method != "initialize" {
		t.Errorf("Method = %q, want %q", initReq.Method, "initialize")
	}

	// initialized notificationが送信されたことを確認
	if len(sent) < 2 {
		t.Fatal("initialized notification not sent")
	}
	initNotif := sent[1]
	if initNotif.Method != "notifications/initialized" {
		t.Errorf("Method = %q, want %q", initNotif.Method, "notifications/initialized")
	}

	// サーバー情報が保存されたことを確認
	if client.serverInfo == nil {
		t.Error("serverInfo not set")
	} else if client.serverInfo.Name != "test-server" {
		t.Errorf("serverInfo.Name = %q, want %q", client.serverInfo.Name, "test-server")
	}
}

func TestMCPClient_ListTools(t *testing.T) {
	transport := newMockTransport()
	client := NewMCPClient("test", transport)

	// initialize response (ID=1)
	transport.setResponse(1, &Message{
		JSONRPC: "2.0",
		ID:      json.Number("1"),
		Result: map[string]any{
			"protocolVersion": "2025-06-18",
			"serverInfo":      map[string]any{"name": "test", "version": "1.0.0"},
			"capabilities":    map[string]any{},
		},
	})

	// tools/list response (ID=2)
	transport.setResponse(2, &Message{
		JSONRPC: "2.0",
		ID:      json.Number("2"),
		Result: map[string]any{
			"tools": []any{
				map[string]any{
					"name":        "add",
					"description": "Add two numbers",
					"inputSchema": map[string]any{"type": "object"},
				},
			},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	if len(tools) != 1 {
		t.Errorf("len(tools) = %d, want 1", len(tools))
	}
	if tools[0].Name != "add" {
		t.Errorf("tools[0].Name = %q, want %q", tools[0].Name, "add")
	}
}

func TestMCPClient_CallTool(t *testing.T) {
	transport := newMockTransport()
	client := NewMCPClient("test", transport)

	// initialize response (ID=1)
	transport.setResponse(1, &Message{
		JSONRPC: "2.0",
		ID:      json.Number("1"),
		Result: map[string]any{
			"protocolVersion": "2025-06-18",
			"serverInfo":      map[string]any{"name": "test", "version": "1.0.0"},
			"capabilities":    map[string]any{},
		},
	})

	// tools/call response (ID=2)
	transport.setResponse(2, &Message{
		JSONRPC: "2.0",
		ID:      json.Number("2"),
		Result: map[string]any{
			"content": []any{
				map[string]any{
					"type": "text",
					"text": "3",
				},
			},
			"isError": false,
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	result, err := client.CallTool(ctx, "add", map[string]any{"a": 1, "b": 2})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	if len(result.Content) != 1 {
		t.Errorf("len(Content) = %d, want 1", len(result.Content))
	}
	if result.Content[0].Text != "3" {
		t.Errorf("Content[0].Text = %q, want %q", result.Content[0].Text, "3")
	}
}

func TestMCPClient_Close(t *testing.T) {
	transport := newMockTransport()
	client := NewMCPClient("test", transport)

	// initialize response (ID=1)
	transport.setResponse(1, &Message{
		JSONRPC: "2.0",
		ID:      json.Number("1"),
		Result: map[string]any{
			"protocolVersion": "2025-06-18",
			"serverInfo":      map[string]any{"name": "test", "version": "1.0.0"},
			"capabilities":    map[string]any{},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	err = client.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if transport.IsConnected() {
		t.Error("transport should be disconnected")
	}
}
