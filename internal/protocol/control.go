package protocol

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/y-oga-819/my-go-claude-agent/internal/transport"
)

const (
	DefaultControlTimeout = 30 * time.Second
)

// ProtocolHandler は制御プロトコルを管理する
type ProtocolHandler struct {
	transport transport.Transport

	// SDK → CLI のリクエスト管理
	pendingRequests map[string]chan *ControlResponse
	requestCounter  uint64

	// CLI → SDK のコールバック
	canUseToolCallback CanUseToolCallback
	hookCallbacks      map[string][]HookCallback
	mcpMessageCallback MCPMessageCallback

	mu sync.RWMutex

	// メッセージ出力チャネル
	msgChan chan Message
	errChan chan error
}

// CanUseToolCallback はツール使用許可確認のコールバック
type CanUseToolCallback func(ctx context.Context, req *CanUseToolRequest) (*CanUseToolResponse, error)

// HookCallback はフックコールバック
type HookCallback func(ctx context.Context, req *HookCallbackRequest) (*HookCallbackResponse, error)

// MCPMessageCallback はMCPメッセージのコールバック
type MCPMessageCallback func(ctx context.Context, req *MCPMessageRequest) (*MCPMessageResponse, error)

// CanUseToolRequest はツール使用許可リクエスト
type CanUseToolRequest struct {
	ToolName              string                `json:"tool_name"`
	Input                 map[string]any        `json:"input"`
	SessionID             string                `json:"session_id,omitempty"`
	PermissionSuggestions []PermissionSuggestion `json:"permission_suggestions,omitempty"`
	BlockedPath           string                `json:"blocked_path,omitempty"`
}

// PermissionSuggestion は権限の提案
type PermissionSuggestion struct {
	Tool   string `json:"tool"`
	Prompt string `json:"prompt"`
}

// CanUseToolResponse はツール使用許可レスポンス
type CanUseToolResponse struct {
	Allow              bool               `json:"allow"`
	UpdatedInput       map[string]any     `json:"updated_input,omitempty"`
	UpdatedPermissions []PermissionUpdate `json:"updated_permissions,omitempty"`
	Message            string             `json:"message,omitempty"`
	Interrupt          bool               `json:"interrupt,omitempty"`
}

// PermissionUpdate は権限の更新
type PermissionUpdate struct {
	Tool   string `json:"tool"`
	Prompt string `json:"prompt"`
}

// HookCallbackRequest はフックコールバックリクエスト
type HookCallbackRequest struct {
	HookType  string         `json:"hook_type"`
	ToolName  string         `json:"tool_name,omitempty"`
	Input     map[string]any `json:"input,omitempty"`
	Output    map[string]any `json:"output,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
}

// HookCallbackResponse はフックコールバックレスポンス
type HookCallbackResponse struct {
	Continue bool   `json:"continue"`
	Message  string `json:"message,omitempty"`
}

// MCPMessageRequest はMCPメッセージリクエスト
type MCPMessageRequest struct {
	ServerName string         `json:"server_name"`
	Message    map[string]any `json:"message"`
}

// MCPMessageResponse はMCPメッセージレスポンス
type MCPMessageResponse struct {
	Message map[string]any `json:"message"`
}

// InitializeRequest は初期化リクエスト
type InitializeRequest struct {
	Subtype            string            `json:"subtype"` // "initialize"
	SystemPrompt       string            `json:"system_prompt,omitempty"`
	AppendSystemPrompt string            `json:"append_system_prompt,omitempty"`
	MCPServers         map[string]any    `json:"mcp_servers,omitempty"`
	AllowedTools       []string          `json:"allowed_tools,omitempty"`
	DisallowedTools    []string          `json:"disallowed_tools,omitempty"`
	PermissionMode     string            `json:"permission_mode,omitempty"`
	Model              string            `json:"model,omitempty"`
	MaxTurns           int               `json:"max_turns,omitempty"`
	MaxBudgetUSD       float64           `json:"max_budget_usd,omitempty"`
	Options            map[string]string `json:"options,omitempty"`
}

// InitializeResponse は初期化レスポンス
type InitializeResponse struct {
	SessionID string `json:"session_id"`
}

// InterruptRequest は中断リクエスト
type InterruptRequest struct {
	Subtype string `json:"subtype"` // "interrupt"
}

// RewindFilesRequest はファイル巻き戻しリクエスト
type RewindFilesRequest struct {
	Subtype       string `json:"subtype"`         // "rewind_files"
	UserMessageID string `json:"user_message_id"` // 巻き戻し先のユーザーメッセージID
}

// NewProtocolHandler は新しいProtocolHandlerを作成する
func NewProtocolHandler(t transport.Transport) *ProtocolHandler {
	return &ProtocolHandler{
		transport:       t,
		pendingRequests: make(map[string]chan *ControlResponse),
		hookCallbacks:   make(map[string][]HookCallback),
		msgChan:         make(chan Message, 100),
		errChan:         make(chan error, 10),
	}
}

// SetCanUseToolCallback はツール使用許可コールバックを設定する
func (h *ProtocolHandler) SetCanUseToolCallback(cb CanUseToolCallback) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.canUseToolCallback = cb
}

// SetMCPMessageCallback はMCPメッセージコールバックを設定する
func (h *ProtocolHandler) SetMCPMessageCallback(cb MCPMessageCallback) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.mcpMessageCallback = cb
}

// AddHookCallback はフックコールバックを追加する
func (h *ProtocolHandler) AddHookCallback(hookType string, cb HookCallback) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.hookCallbacks[hookType] = append(h.hookCallbacks[hookType], cb)
}

// Messages はメッセージチャネルを返す
func (h *ProtocolHandler) Messages() <-chan Message {
	return h.msgChan
}

// Errors はエラーチャネルを返す
func (h *ProtocolHandler) Errors() <-chan error {
	return h.errChan
}

// SendControlRequest は制御リクエストを送信し、レスポンスを待つ
func (h *ProtocolHandler) SendControlRequest(ctx context.Context, req any) (*ControlResponse, error) {
	return h.SendControlRequestWithTimeout(ctx, req, DefaultControlTimeout)
}

// SendControlRequestWithTimeout はタイムアウト付きで制御リクエストを送信する
func (h *ProtocolHandler) SendControlRequestWithTimeout(ctx context.Context, req any, timeout time.Duration) (*ControlResponse, error) {
	// リクエストIDを生成
	id := h.generateRequestID()

	// レスポンスチャネルを作成
	respChan := make(chan *ControlResponse, 1)

	h.mu.Lock()
	h.pendingRequests[id] = respChan
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.pendingRequests, id)
		h.mu.Unlock()
	}()

	// リクエストを送信
	ctrlReq := ControlRequest{
		Type:      "control_request",
		RequestID: id,
		Request:   req,
	}

	data, err := json.Marshal(ctrlReq)
	if err != nil {
		return nil, fmt.Errorf("marshal control request: %w", err)
	}

	if err := h.transport.Write(data); err != nil {
		return nil, fmt.Errorf("write control request: %w", err)
	}

	// レスポンスを待つ
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case resp := <-respChan:
		return resp, nil
	case <-timeoutCtx.Done():
		return nil, fmt.Errorf("control request timeout: %w", timeoutCtx.Err())
	}
}

// HandleIncoming は受信したメッセージを処理する
func (h *ProtocolHandler) HandleIncoming(ctx context.Context, raw transport.RawMessage) error {
	msg, err := ParseMessage(raw.Data)
	if err != nil {
		return fmt.Errorf("parse message: %w", err)
	}

	switch m := msg.(type) {
	case *ControlRequest:
		return h.handleControlRequest(ctx, m)

	case *ControlResponse:
		return h.handleControlResponse(m)

	case *AssistantMessage, *SystemMessage, *UserMessage, *ResultMessage:
		select {
		case h.msgChan <- msg:
		default:
			// チャネルがいっぱいの場合は古いメッセージを破棄
			select {
			case <-h.msgChan:
			default:
			}
			h.msgChan <- msg
		}

	default:
		// 未知のメッセージ型はそのまま転送
		select {
		case h.msgChan <- msg:
		default:
		}
	}

	return nil
}

func (h *ProtocolHandler) handleControlRequest(ctx context.Context, req *ControlRequest) error {
	// リクエストの内容をパース
	reqData, ok := req.Request.(map[string]any)
	if !ok {
		return h.sendControlError(req.RequestID, "invalid request format")
	}

	subtype, _ := reqData["subtype"].(string)

	switch subtype {
	case "can_use_tool":
		return h.handleCanUseTool(ctx, req.RequestID, reqData)

	case "hook_callback":
		return h.handleHookCallback(ctx, req.RequestID, reqData)

	case "mcp_message":
		return h.handleMCPMessage(ctx, req.RequestID, reqData)

	default:
		// 未知のリクエストは成功レスポンスを返す
		return h.sendControlSuccess(req.RequestID, nil)
	}
}

func (h *ProtocolHandler) handleCanUseTool(ctx context.Context, requestID string, reqData map[string]any) error {
	h.mu.RLock()
	cb := h.canUseToolCallback
	h.mu.RUnlock()

	if cb == nil {
		// コールバックが設定されていない場合はデフォルトで許可
		return h.sendControlSuccess(requestID, &CanUseToolResponse{Allow: true})
	}

	// リクエストをパース
	reqJSON, err := json.Marshal(reqData)
	if err != nil {
		return h.sendControlError(requestID, "marshal request: "+err.Error())
	}

	var toolReq CanUseToolRequest
	if err := json.Unmarshal(reqJSON, &toolReq); err != nil {
		return h.sendControlError(requestID, "unmarshal request: "+err.Error())
	}

	// コールバックを呼び出し
	resp, err := cb(ctx, &toolReq)
	if err != nil {
		return h.sendControlError(requestID, err.Error())
	}

	return h.sendControlSuccess(requestID, resp)
}

func (h *ProtocolHandler) handleHookCallback(ctx context.Context, requestID string, reqData map[string]any) error {
	hookType, _ := reqData["hook_type"].(string)

	h.mu.RLock()
	callbacks := h.hookCallbacks[hookType]
	h.mu.RUnlock()

	if len(callbacks) == 0 {
		// コールバックが設定されていない場合は続行
		return h.sendControlSuccess(requestID, &HookCallbackResponse{Continue: true})
	}

	// リクエストをパース
	reqJSON, err := json.Marshal(reqData)
	if err != nil {
		return h.sendControlError(requestID, "marshal request: "+err.Error())
	}

	var hookReq HookCallbackRequest
	if err := json.Unmarshal(reqJSON, &hookReq); err != nil {
		return h.sendControlError(requestID, "unmarshal request: "+err.Error())
	}

	// 全てのコールバックを呼び出し
	for _, cb := range callbacks {
		resp, err := cb(ctx, &hookReq)
		if err != nil {
			return h.sendControlError(requestID, err.Error())
		}
		if !resp.Continue {
			return h.sendControlSuccess(requestID, resp)
		}
	}

	return h.sendControlSuccess(requestID, &HookCallbackResponse{Continue: true})
}

func (h *ProtocolHandler) handleMCPMessage(ctx context.Context, requestID string, reqData map[string]any) error {
	h.mu.RLock()
	cb := h.mcpMessageCallback
	h.mu.RUnlock()

	if cb == nil {
		return h.sendControlError(requestID, "no MCP message callback")
	}

	// リクエストをパース
	reqJSON, err := json.Marshal(reqData)
	if err != nil {
		return h.sendControlError(requestID, "marshal request: "+err.Error())
	}

	var mcpReq MCPMessageRequest
	if err := json.Unmarshal(reqJSON, &mcpReq); err != nil {
		return h.sendControlError(requestID, "unmarshal request: "+err.Error())
	}

	// コールバックを呼び出し
	resp, err := cb(ctx, &mcpReq)
	if err != nil {
		return h.sendControlError(requestID, err.Error())
	}

	return h.sendControlSuccess(requestID, resp)
}

func (h *ProtocolHandler) handleControlResponse(resp *ControlResponse) error {
	h.mu.RLock()
	ch, ok := h.pendingRequests[resp.Response.RequestID]
	h.mu.RUnlock()

	if !ok {
		// 対応するリクエストが見つからない（タイムアウト済みなど）
		return nil
	}

	select {
	case ch <- resp:
	default:
		// チャネルがいっぱいの場合は無視
	}

	return nil
}

func (h *ProtocolHandler) sendControlSuccess(requestID string, response any) error {
	resp := ControlResponse{
		Type: "control_response",
		Response: ControlResponseBody{
			Subtype:   "success",
			RequestID: requestID,
			Response:  response,
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal control response: %w", err)
	}

	return h.transport.Write(data)
}

func (h *ProtocolHandler) sendControlError(requestID string, errMsg string) error {
	resp := ControlResponse{
		Type: "control_response",
		Response: ControlResponseBody{
			Subtype:   "error",
			RequestID: requestID,
			Error:     errMsg,
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal control response: %w", err)
	}

	return h.transport.Write(data)
}

func (h *ProtocolHandler) generateRequestID() string {
	id := atomic.AddUint64(&h.requestCounter, 1)
	return fmt.Sprintf("sdk-%d", id)
}

// Close はハンドラをクローズする
func (h *ProtocolHandler) Close() {
	close(h.msgChan)
	close(h.errChan)
}
