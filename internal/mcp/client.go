package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

// MCPClient は外部MCPサーバーとの通信を管理する
type MCPClient struct {
	name      string
	transport Transport

	serverInfo   *ServerInfo
	capabilities *Capabilities

	pendingReqs map[any]chan *Message
	reqID       atomic.Int64

	mu        sync.RWMutex
	connected bool

	stopChan chan struct{}
	msgChan  chan *Message
}

// ServerInfo はサーバー情報
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Capabilities はサーバー能力
type Capabilities struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
}

// ToolsCapability はツール機能の能力
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability はリソース機能の能力
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptsCapability はプロンプト機能の能力
type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// NewMCPClient は新しいMCPClientを作成する
func NewMCPClient(name string, transport Transport) *MCPClient {
	return &MCPClient{
		name:        name,
		transport:   transport,
		pendingReqs: make(map[any]chan *Message),
		stopChan:    make(chan struct{}),
		msgChan:     make(chan *Message, 100),
	}
}

// Connect は接続と初期化を行う
func (c *MCPClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.connected {
		c.mu.Unlock()
		return errors.New("already connected")
	}
	c.mu.Unlock()

	// トランスポート接続
	if err := c.transport.Connect(ctx); err != nil {
		return fmt.Errorf("transport connect failed: %w", err)
	}

	// メッセージ受信ループを開始
	go c.receiveLoop()

	// 初期化ハンドシェイク
	if err := c.initialize(ctx); err != nil {
		c.transport.Close()
		return fmt.Errorf("initialize failed: %w", err)
	}

	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()

	return nil
}

// receiveLoop はメッセージ受信ループ
func (c *MCPClient) receiveLoop() {
	for {
		select {
		case <-c.stopChan:
			return
		default:
		}

		msg, err := c.transport.Receive()
		if err != nil {
			// 接続が閉じられた場合は終了
			return
		}

		// レスポンスの場合、対応するリクエストに通知
		if msg.ID != nil && msg.Method == "" {
			c.mu.Lock()
			ch, ok := c.pendingReqs[normalizeID(msg.ID)]
			if ok {
				delete(c.pendingReqs, normalizeID(msg.ID))
			}
			c.mu.Unlock()

			if ok {
				select {
				case ch <- msg:
				default:
				}
			}
		} else {
			// 通知などは別チャネルに送る
			select {
			case c.msgChan <- msg:
			default:
			}
		}
	}
}

// normalizeID はIDを正規化する
func normalizeID(id any) any {
	switch v := id.(type) {
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return i
		}
		return v.String()
	case float64:
		return int64(v)
	default:
		return id
	}
}

// initialize は初期化ハンドシェイクを行う
func (c *MCPClient) initialize(ctx context.Context) error {
	// 1. initialize request送信
	initReq := &Message{
		JSONRPC: "2.0",
		ID:      c.nextID(),
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "go-claude-agent",
				"version": "1.0.0",
			},
		},
	}

	resp, err := c.request(ctx, initReq)
	if err != nil {
		return fmt.Errorf("initialize request failed: %w", err)
	}

	// 2. サーバー情報を保存
	if err := c.parseInitializeResult(resp); err != nil {
		return fmt.Errorf("failed to parse initialize result: %w", err)
	}

	// 3. initialized notification送信
	if err := c.transport.Send(&Message{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}); err != nil {
		return fmt.Errorf("failed to send initialized notification: %w", err)
	}

	return nil
}

// parseInitializeResult はinitialize resultを解析する
func (c *MCPClient) parseInitializeResult(msg *Message) error {
	result, ok := msg.Result.(map[string]any)
	if !ok {
		return errors.New("invalid result type")
	}

	// サーバー情報
	if serverInfo, ok := result["serverInfo"].(map[string]any); ok {
		c.serverInfo = &ServerInfo{}
		if name, ok := serverInfo["name"].(string); ok {
			c.serverInfo.Name = name
		}
		if version, ok := serverInfo["version"].(string); ok {
			c.serverInfo.Version = version
		}
	}

	// 能力
	if capabilities, ok := result["capabilities"].(map[string]any); ok {
		c.capabilities = &Capabilities{}
		if tools, ok := capabilities["tools"].(map[string]any); ok {
			c.capabilities.Tools = &ToolsCapability{}
			if listChanged, ok := tools["listChanged"].(bool); ok {
				c.capabilities.Tools.ListChanged = listChanged
			}
		}
	}

	return nil
}

// request はリクエストを送信してレスポンスを待つ
func (c *MCPClient) request(ctx context.Context, msg *Message) (*Message, error) {
	respChan := make(chan *Message, 1)

	c.mu.Lock()
	c.pendingReqs[normalizeID(msg.ID)] = respChan
	c.mu.Unlock()

	if err := c.transport.Send(msg); err != nil {
		c.mu.Lock()
		delete(c.pendingReqs, normalizeID(msg.ID))
		c.mu.Unlock()
		return nil, err
	}

	select {
	case resp := <-respChan:
		if resp.Error != nil {
			return nil, fmt.Errorf("RPC error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pendingReqs, normalizeID(msg.ID))
		c.mu.Unlock()
		return nil, ctx.Err()
	}
}

// nextID は次のリクエストIDを生成する
func (c *MCPClient) nextID() int64 {
	return c.reqID.Add(1)
}

// ListTools はツール一覧を取得する
func (c *MCPClient) ListTools(ctx context.Context) ([]ToolInfo, error) {
	c.mu.RLock()
	connected := c.connected
	c.mu.RUnlock()

	if !connected {
		return nil, errors.New("not connected")
	}

	req := &Message{
		JSONRPC: "2.0",
		ID:      c.nextID(),
		Method:  "tools/list",
	}

	resp, err := c.request(ctx, req)
	if err != nil {
		return nil, err
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		return nil, errors.New("invalid result type")
	}

	toolsRaw, ok := result["tools"].([]any)
	if !ok {
		return nil, errors.New("invalid tools type")
	}

	var tools []ToolInfo
	for _, t := range toolsRaw {
		toolMap, ok := t.(map[string]any)
		if !ok {
			continue
		}

		tool := ToolInfo{}
		if name, ok := toolMap["name"].(string); ok {
			tool.Name = name
		}
		if desc, ok := toolMap["description"].(string); ok {
			tool.Description = desc
		}
		if schema, ok := toolMap["inputSchema"].(map[string]any); ok {
			tool.InputSchema = schema
		}
		tools = append(tools, tool)
	}

	return tools, nil
}

// CallTool はツールを呼び出す
func (c *MCPClient) CallTool(ctx context.Context, name string, args map[string]any) (*ToolResult, error) {
	c.mu.RLock()
	connected := c.connected
	c.mu.RUnlock()

	if !connected {
		return nil, errors.New("not connected")
	}

	req := &Message{
		JSONRPC: "2.0",
		ID:      c.nextID(),
		Method:  "tools/call",
		Params: map[string]any{
			"name":      name,
			"arguments": args,
		},
	}

	resp, err := c.request(ctx, req)
	if err != nil {
		return nil, err
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		return nil, errors.New("invalid result type")
	}

	toolResult := &ToolResult{}

	if contentRaw, ok := result["content"].([]any); ok {
		for _, c := range contentRaw {
			contentMap, ok := c.(map[string]any)
			if !ok {
				continue
			}
			block := ContentBlock{}
			if t, ok := contentMap["type"].(string); ok {
				block.Type = t
			}
			if text, ok := contentMap["text"].(string); ok {
				block.Text = text
			}
			toolResult.Content = append(toolResult.Content, block)
		}
	}

	if isError, ok := result["isError"].(bool); ok {
		toolResult.IsError = isError
	}

	return toolResult, nil
}

// Close は接続を閉じる
func (c *MCPClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	c.connected = false
	close(c.stopChan)

	return c.transport.Close()
}

// GetServerInfo はサーバー情報を取得する
func (c *MCPClient) GetServerInfo() *ServerInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serverInfo
}

// GetCapabilities はサーバー能力を取得する
func (c *MCPClient) GetCapabilities() *Capabilities {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.capabilities
}
