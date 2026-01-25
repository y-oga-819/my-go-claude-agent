package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/y-oga-819/my-go-claude-agent/internal/hooks"
	"github.com/y-oga-819/my-go-claude-agent/internal/protocol"
	"github.com/y-oga-819/my-go-claude-agent/internal/transport"
)

var (
	// ErrSessionIDNotReady はセッションIDがまだ取得できていない場合のエラー
	ErrSessionIDNotReady = errors.New("session ID not ready: waiting for first message from CLI")
)

// Client はClaude CLIとの双方向ストリーミング通信を管理する
type Client struct {
	opts        *Options
	transport   transport.Transport
	protocol    *protocol.ProtocolHandler
	hookManager *hooks.Manager

	// sessionID はatomic.Pointerで管理（ロックフリー）
	// Connect()時のデッドロックを回避するため、c.muとは独立して管理
	sessionID atomic.Pointer[string]

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

	c := &Client{
		opts:        opts,
		hookManager: hooks.NewManager(),
		msgChan:     make(chan protocol.Message, 100),
		errChan:     make(chan error, 10),
		closeChan:   make(chan struct{}),
	}

	// フックを登録
	c.registerHooks()

	return c
}

// Connect はCLIに接続し、双方向ストリーミングを開始する
// 注意: SessionID()は最初のメッセージを受信するまでErrSessionIDNotReadyを返す
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

		// セッション設定
		Resume:                  c.opts.Resume,
		ForkSession:             c.opts.ForkSession,
		Continue:                c.opts.Continue,
		EnableFileCheckpointing: c.opts.EnableFileCheckpointing,
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

	// セッションIDを取得（複数のレスポンス形式に対応）
	c.extractSessionIDFromResponse(resp)

	return nil
}

// extractSessionIDFromResponse はControlResponseからsessionIDを抽出する
func (c *Client) extractSessionIDFromResponse(resp *protocol.ControlResponse) {
	// 既にsessionIDが設定されている場合はスキップ（ロックフリー）
	if c.sessionID.Load() != nil {
		return
	}

	// Response.Responseがmap[string]anyの場合
	if respData, ok := resp.Response.Response.(map[string]any); ok {
		if sid, ok := respData["session_id"].(string); ok && sid != "" {
			// CompareAndSwapで初回のみ設定（競合安全）
			c.sessionID.CompareAndSwap(nil, &sid)
		}
	}
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

			// ResultMessageからsessionIDを抽出
			c.extractSessionIDFromRawMessage(rawMsg)

			if err := c.protocol.HandleIncoming(ctx, rawMsg); err != nil {
				c.errChan <- err
			}
		}
	}
}

// extractSessionIDFromRawMessage はRawMessageからsessionIDを抽出し、未設定の場合に設定する
// ロックフリー設計: atomic.Pointerを使用してデッドロックを回避
func (c *Client) extractSessionIDFromRawMessage(rawMsg transport.RawMessage) {
	// 既にsessionIDが設定されている場合はスキップ（ロックフリー）
	if c.sessionID.Load() != nil {
		return
	}

	// resultメッセージからsession_idを取得
	if rawMsg.Type == "result" {
		if sid, ok := rawMsg.Data["session_id"].(string); ok && sid != "" {
			// CompareAndSwapで初回のみ設定（競合安全）
			c.sessionID.CompareAndSwap(nil, &sid)
			return
		}
	}

	// systemメッセージからsession_idを取得
	if rawMsg.Type == "system" {
		if data, ok := rawMsg.Data["data"].(map[string]any); ok {
			if sid, ok := data["session_id"].(string); ok && sid != "" {
				// CompareAndSwapで初回のみ設定（競合安全）
				c.sessionID.CompareAndSwap(nil, &sid)
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

	// sessionIDを取得（ロックフリー）
	sessionID := c.getSessionIDString()

	// UserPromptSubmitフックをトリガー
	hookInput := &hooks.Input{
		SessionID:     sessionID,
		HookEventName: string(hooks.EventUserPromptSubmit),
		CWD:           c.opts.CWD,
	}
	output, err := c.hookManager.Trigger(ctx, hooks.EventUserPromptSubmit, hookInput)
	if err != nil {
		return fmt.Errorf("hook error: %w", err)
	}
	if !output.Continue {
		return fmt.Errorf("blocked by hook: %s", output.Reason)
	}

	msg := protocol.UserMessage{
		Type: "user",
		Message: protocol.UserContent{
			Role:    "user",
			Content: content,
		},
		SessionID: sessionID,
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

	// sessionIDを取得（ロックフリー）
	sessionID := c.getSessionIDString()

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
		"session_id":         sessionID,
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
// セッションIDはCLIからの最初のメッセージ受信後に取得可能になる
// まだ取得できていない場合はErrSessionIDNotReadyを返す
func (c *Client) SessionID() (string, error) {
	// ロックフリー: atomic.Pointerを使用
	sid := c.sessionID.Load()
	if sid == nil {
		return "", ErrSessionIDNotReady
	}
	return *sid, nil
}

// SessionIDReady はセッションIDが取得可能かどうかを返す
func (c *Client) SessionIDReady() bool {
	// ロックフリー: atomic.Pointerを使用
	return c.sessionID.Load() != nil
}

// getSessionIDString はsessionIDの文字列値を返す（未設定の場合は空文字列）
// 内部ヘルパー関数（ロックフリー）
func (c *Client) getSessionIDString() string {
	sid := c.sessionID.Load()
	if sid == nil {
		return ""
	}
	return *sid
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
// セッションIDはCLIからの最初のメッセージ受信後に取得可能になる
// まだ取得できていない場合はErrSessionIDNotReadyを返す
func (s *Stream) SessionID() (string, error) {
	return s.client.SessionID()
}

// SessionIDReady はセッションIDが取得可能かどうかを返す
func (s *Stream) SessionIDReady() bool {
	return s.client.SessionIDReady()
}

// registerHooks はOptionsからフックを登録する
func (c *Client) registerHooks() {
	if c.opts.Hooks == nil {
		return
	}

	// PreToolUse
	for _, entry := range c.opts.Hooks.PreToolUse {
		c.hookManager.Register(hooks.EventPreToolUse, convertHookEntry(entry))
	}

	// PostToolUse
	for _, entry := range c.opts.Hooks.PostToolUse {
		c.hookManager.Register(hooks.EventPostToolUse, convertHookEntry(entry))
	}

	// UserPromptSubmit
	for _, entry := range c.opts.Hooks.UserPromptSubmit {
		c.hookManager.Register(hooks.EventUserPromptSubmit, convertHookEntry(entry))
	}

	// Notification
	for _, entry := range c.opts.Hooks.Notification {
		c.hookManager.Register(hooks.EventNotification, convertHookEntry(entry))
	}

	// Stop
	for _, entry := range c.opts.Hooks.Stop {
		c.hookManager.Register(hooks.EventStop, convertHookEntry(entry))
	}

	// SubagentStop
	for _, entry := range c.opts.Hooks.SubagentStop {
		c.hookManager.Register(hooks.EventSubagentStop, convertHookEntry(entry))
	}

	// PreCompact
	for _, entry := range c.opts.Hooks.PreCompact {
		c.hookManager.Register(hooks.EventPreCompact, convertHookEntry(entry))
	}
}

// convertHookEntry はclaude.HookEntryをhooks.Entryに変換する
func convertHookEntry(entry HookEntry) hooks.Entry {
	hooksEntry := hooks.Entry{
		Timeout: entry.Timeout,
	}

	// マッチャーを設定
	if entry.Matcher != "" {
		hooksEntry.Matcher = hooks.NewMatcher(entry.Matcher)
	}

	// タイプに応じて設定
	switch entry.Type {
	case HookTypeCommand:
		hooksEntry.Type = hooks.HookTypeCommand
		hooksEntry.Command = entry.Command
	default:
		hooksEntry.Type = hooks.HookTypeCallback
		if entry.Callback != nil {
			hooksEntry.Callback = func(ctx context.Context, input *hooks.Input) (*hooks.Output, error) {
				hookInput := &HookInput{
					HookEventName:  input.HookEventName,
					SessionID:      input.SessionID,
					TranscriptPath: input.TranscriptPath,
					CWD:            input.CWD,
					ToolName:       input.ToolName,
					ToolInput:      input.ToolInput,
					ToolOutput:     input.ToolOutput,
				}

				hookOutput, err := entry.Callback(ctx, hookInput)
				if err != nil {
					return nil, err
				}

				output := &hooks.Output{
					Continue:       hookOutput.Continue,
					StopReason:     hookOutput.StopReason,
					SuppressOutput: hookOutput.SuppressOutput,
					Decision:       hookOutput.Decision,
					SystemMessage:  hookOutput.SystemMessage,
					Reason:         hookOutput.Reason,
				}

				if hookOutput.HookSpecificOutput != nil {
					output.HookSpecificOutput = &hooks.SpecificOutput{
						HookEventName:            hookOutput.HookSpecificOutput.HookEventName,
						PermissionDecision:       hookOutput.HookSpecificOutput.PermissionDecision,
						PermissionDecisionReason: hookOutput.HookSpecificOutput.PermissionDecisionReason,
						UpdatedInput:             hookOutput.HookSpecificOutput.UpdatedInput,
						AdditionalContext:        hookOutput.HookSpecificOutput.AdditionalContext,
					}
				}

				return output, nil
			}
		}
	}

	return hooksEntry
}

// TriggerHook はフックをトリガーする
func (c *Client) TriggerHook(ctx context.Context, event hooks.Event, input *hooks.Input) (*hooks.Output, error) {
	return c.hookManager.Trigger(ctx, event, input)
}

// HookManager はフックマネージャーを返す
func (c *Client) HookManager() *hooks.Manager {
	return c.hookManager
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
