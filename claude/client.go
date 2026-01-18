package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/y-oga-819/my-go-claude-agent/internal/protocol"
	"github.com/y-oga-819/my-go-claude-agent/internal/transport"
)

// Client はClaude CLIとの双方向ストリーミング通信を管理する
type Client struct {
	opts      *Options
	transport transport.Transport
	protocol  *protocol.ProtocolHandler

	sessionID string

	msgChan   chan protocol.Message
	errChan   chan error
	closeChan chan struct{}

	mu     sync.RWMutex
	closed bool
}

// Stream は双方向ストリーミングの状態を表す
type Stream struct {
	client *Client
}

// NewClient は新しいClientを作成する
func NewClient(opts *Options) *Client {
	if opts == nil {
		opts = &Options{}
	}

	return &Client{
		opts:      opts,
		msgChan:   make(chan protocol.Message, 100),
		errChan:   make(chan error, 10),
		closeChan: make(chan struct{}),
	}
}

// Connect はCLIに接続し、双方向ストリーミングを開始する
func (c *Client) Connect(ctx context.Context) (*Stream, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	// Transport設定
	config := transport.Config{
		CLIPath:       c.opts.CLIPath,
		CWD:           c.opts.CWD,
		StreamingMode: true, // 双方向ストリーミングモード
	}

	c.transport = transport.NewSubprocessTransport(config)

	// 接続
	if err := c.transport.Connect(ctx); err != nil {
		return nil, &SDKError{Op: "connect", Err: ErrCLIConnection, Details: err.Error()}
	}

	// プロトコルハンドラを作成
	c.protocol = protocol.NewProtocolHandler(c.transport)

	// canUseToolコールバックを設定
	if c.opts.CanUseTool != nil {
		c.protocol.SetCanUseToolCallback(func(ctx context.Context, req *protocol.CanUseToolRequest) (*protocol.CanUseToolResponse, error) {
			permCtx := &ToolPermissionContext{
				SessionID:             req.SessionID,
				PermissionSuggestions: convertPermissionSuggestions(req.PermissionSuggestions),
				BlockedPath:           req.BlockedPath,
			}

			result, err := c.opts.CanUseTool(ctx, req.ToolName, req.Input, permCtx)
			if err != nil {
				return nil, err
			}

			return &protocol.CanUseToolResponse{
				Allow:              result.Allow,
				UpdatedInput:       result.UpdatedInput,
				UpdatedPermissions: convertPermissionUpdates(result.UpdatedPermissions),
				Message:            result.Message,
				Interrupt:          result.Interrupt,
			}, nil
		})
	}

	// メッセージ受信ループを開始
	go c.receiveLoop(ctx)

	// 初期化リクエストを送信
	if err := c.initialize(ctx); err != nil {
		c.transport.Close()
		return nil, err
	}

	return &Stream{client: c}, nil
}

func (c *Client) initialize(ctx context.Context) error {
	initReq := protocol.InitializeRequest{
		Subtype:            "initialize",
		SystemPrompt:       c.opts.SystemPrompt,
		AppendSystemPrompt: c.opts.AppendSystemPrompt,
		Model:              c.opts.Model,
		MaxTurns:           c.opts.MaxTurns,
		MaxBudgetUSD:       c.opts.MaxBudgetUSD,
		AllowedTools:       c.opts.AllowedTools,
		DisallowedTools:    c.opts.DisallowedTools,
	}

	if c.opts.PermissionMode != "" {
		initReq.PermissionMode = string(c.opts.PermissionMode)
	}

	resp, err := c.protocol.SendControlRequest(ctx, initReq)
	if err != nil {
		return &SDKError{Op: "initialize", Err: err}
	}

	if resp.Response.Subtype == "error" {
		return &SDKError{Op: "initialize", Err: fmt.Errorf("initialization failed"), Details: resp.Response.Error}
	}

	// セッションIDを取得
	if respData, ok := resp.Response.Response.(map[string]any); ok {
		if sid, ok := respData["session_id"].(string); ok {
			c.sessionID = sid
		}
	}

	return nil
}

func (c *Client) receiveLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.closeChan:
			return

		case err := <-c.transport.Errors():
			if err != nil {
				c.errChan <- err
			}

		case rawMsg, ok := <-c.transport.Messages():
			if !ok {
				return
			}

			if err := c.protocol.HandleIncoming(ctx, rawMsg); err != nil {
				c.errChan <- err
			}
		}
	}
}

// Send はユーザーメッセージを送信する
func (c *Client) Send(ctx context.Context, content string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed || c.transport == nil {
		return fmt.Errorf("client is not connected")
	}

	msg := protocol.UserMessage{
		Type: "user",
		Message: protocol.UserContent{
			Role:    "user",
			Content: content,
		},
		SessionID: c.sessionID,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	return c.transport.Write(data)
}

// SendToolResult はツール実行結果を送信する
func (c *Client) SendToolResult(ctx context.Context, toolUseID string, result any, isError bool) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed || c.transport == nil {
		return fmt.Errorf("client is not connected")
	}

	msg := map[string]any{
		"type": "user",
		"message": map[string]any{
			"role": "user",
			"content": []map[string]any{
				{
					"type":        "tool_result",
					"tool_use_id": toolUseID,
					"content":     result,
					"is_error":    isError,
				},
			},
		},
		"session_id":         c.sessionID,
		"parent_tool_use_id": toolUseID,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	return c.transport.Write(data)
}

// Interrupt は実行を中断する
func (c *Client) Interrupt(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed || c.protocol == nil {
		return fmt.Errorf("client is not connected")
	}

	interruptReq := protocol.InterruptRequest{
		Subtype: "interrupt",
	}

	_, err := c.protocol.SendControlRequest(ctx, interruptReq)
	if err != nil {
		return &SDKError{Op: "interrupt", Err: err}
	}

	return nil
}

// RewindFiles はファイルを指定したチェックポイントに巻き戻す
func (c *Client) RewindFiles(ctx context.Context, userMessageID string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed || c.protocol == nil {
		return fmt.Errorf("client is not connected")
	}

	rewindReq := protocol.RewindFilesRequest{
		Subtype:       "rewind_files",
		UserMessageID: userMessageID,
	}

	resp, err := c.protocol.SendControlRequest(ctx, rewindReq)
	if err != nil {
		return &SDKError{Op: "rewind_files", Err: err}
	}

	if resp.Response.Subtype == "error" {
		return &SDKError{Op: "rewind_files", Err: fmt.Errorf("rewind failed"), Details: resp.Response.Error}
	}

	return nil
}

// Messages はメッセージチャネルを返す
func (c *Client) Messages() <-chan protocol.Message {
	return c.protocol.Messages()
}

// Errors はエラーチャネルを返す
func (c *Client) Errors() <-chan error {
	return c.errChan
}

// SessionID は現在のセッションIDを返す
func (c *Client) SessionID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessionID
}

// Close はクライアントをクローズする
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	close(c.closeChan)

	if c.protocol != nil {
		c.protocol.Close()
	}

	if c.transport != nil {
		return c.transport.Close()
	}

	return nil
}

// Stream methods

// Messages はストリームからのメッセージチャネルを返す
func (s *Stream) Messages() <-chan protocol.Message {
	return s.client.Messages()
}

// Errors はストリームからのエラーチャネルを返す
func (s *Stream) Errors() <-chan error {
	return s.client.Errors()
}

// Send はユーザーメッセージを送信する
func (s *Stream) Send(ctx context.Context, content string) error {
	return s.client.Send(ctx, content)
}

// SendToolResult はツール実行結果を送信する
func (s *Stream) SendToolResult(ctx context.Context, toolUseID string, result any, isError bool) error {
	return s.client.SendToolResult(ctx, toolUseID, result, isError)
}

// Interrupt は実行を中断する
func (s *Stream) Interrupt(ctx context.Context) error {
	return s.client.Interrupt(ctx)
}

// RewindFiles はファイルを指定したチェックポイントに巻き戻す
func (s *Stream) RewindFiles(ctx context.Context, userMessageID string) error {
	return s.client.RewindFiles(ctx, userMessageID)
}

// Close はストリームをクローズする
func (s *Stream) Close() error {
	return s.client.Close()
}

// SessionID は現在のセッションIDを返す
func (s *Stream) SessionID() string {
	return s.client.SessionID()
}

// Helper functions

func convertPermissionSuggestions(suggestions []protocol.PermissionSuggestion) []PermissionSuggestion {
	if suggestions == nil {
		return nil
	}
	result := make([]PermissionSuggestion, len(suggestions))
	for i, s := range suggestions {
		result[i] = PermissionSuggestion{
			Tool:   s.Tool,
			Prompt: s.Prompt,
		}
	}
	return result
}

func convertPermissionUpdates(updates []PermissionUpdate) []protocol.PermissionUpdate {
	if updates == nil {
		return nil
	}
	result := make([]protocol.PermissionUpdate, len(updates))
	for i, u := range updates {
		result[i] = protocol.PermissionUpdate{
			Tool:   u.Tool,
			Prompt: u.Prompt,
		}
	}
	return result
}
