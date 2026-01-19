package mcp

import (
	"fmt"
	"sync"
)

// Tool はMCPツールを定義
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
	Handler     ToolHandler
}

// ToolHandler はツールのハンドラ関数
type ToolHandler func(args map[string]any) (*ToolResult, error)

// ToolResult はツール実行結果
type ToolResult struct {
	Content []ContentBlock
	IsError bool
}

// ContentBlock はコンテンツブロック
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// SDKMCPServer はインプロセスMCPサーバー
type SDKMCPServer struct {
	Name    string
	Version string
	tools   map[string]Tool
	mu      sync.RWMutex
}

// NewSDKMCPServer は新しいSDKMCPServerを作成する
func NewSDKMCPServer(name, version string) *SDKMCPServer {
	return &SDKMCPServer{
		Name:    name,
		Version: version,
		tools:   make(map[string]Tool),
	}
}

// AddTool はツールを追加する
func (s *SDKMCPServer) AddTool(tool Tool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[tool.Name] = tool
}

// RemoveTool はツールを削除する
func (s *SDKMCPServer) RemoveTool(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tools, name)
}

// GetTool はツールを取得する
func (s *SDKMCPServer) GetTool(name string) (Tool, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tool, ok := s.tools[name]
	return tool, ok
}

// ListTools は全てのツールをリストする
func (s *SDKMCPServer) ListTools() []ToolInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ToolInfo, 0, len(s.tools))
	for _, tool := range s.tools {
		result = append(result, ToolInfo{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}
	return result
}

// ToolInfo はツール情報（ハンドラなし）
type ToolInfo struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// HandleCall はツール呼び出しを処理する
func (s *SDKMCPServer) HandleCall(toolName string, args map[string]any) (*ToolResult, error) {
	s.mu.RLock()
	tool, ok := s.tools[toolName]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("tool not found: %s", toolName)
	}

	if tool.Handler == nil {
		return nil, fmt.Errorf("tool handler not set: %s", toolName)
	}

	return tool.Handler(args)
}

// HandleMessage はMCPメッセージを処理する
func (s *SDKMCPServer) HandleMessage(msg *Message) (*Response, error) {
	switch msg.Method {
	case "tools/list":
		tools := s.ListTools()
		return &Response{
			ID:     msg.ID,
			Result: map[string]any{"tools": tools},
		}, nil

	case "tools/call":
		params, ok := msg.Params.(map[string]any)
		if !ok {
			return &Response{
				ID:    msg.ID,
				Error: &ResponseError{Code: -32600, Message: "invalid params"},
			}, nil
		}

		toolName, _ := params["name"].(string)
		args, _ := params["arguments"].(map[string]any)

		result, err := s.HandleCall(toolName, args)
		if err != nil {
			return &Response{
				ID:    msg.ID,
				Error: &ResponseError{Code: -32000, Message: err.Error()},
			}, nil
		}

		return &Response{
			ID: msg.ID,
			Result: map[string]any{
				"content":  result.Content,
				"isError": result.IsError,
			},
		}, nil

	default:
		return &Response{
			ID:    msg.ID,
			Error: &ResponseError{Code: -32601, Message: "method not found"},
		}, nil
	}
}

// Message はMCPメッセージ（JSON-RPC 2.0）
// リクエスト、レスポンス、通知すべてを表現する
type Message struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id,omitempty"`     // request/response
	Method  string         `json:"method,omitempty"` // request/notification
	Params  any            `json:"params,omitempty"` // request
	Result  any            `json:"result,omitempty"` // response
	Error   *ResponseError `json:"error,omitempty"`  // response
}

// Response はMCPレスポンス
type Response struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id"`
	Result  any            `json:"result,omitempty"`
	Error   *ResponseError `json:"error,omitempty"`
}

// ResponseError はエラーレスポンス
type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}
