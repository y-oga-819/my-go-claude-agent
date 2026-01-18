# Step 1: プロジェクト初期化・型定義

## 目的

プロジェクトの基盤を構築し、主要な型を定義する。

## 成果物

- `go.mod` - モジュール定義
- `claude/options.go` - オプション型
- `claude/errors.go` - エラー型
- `internal/protocol/messages.go` - メッセージ型

## 実装内容

### 1.1 go.mod

```go
module github.com/y-oga-819/my-go-claude-agent

go 1.23
```

### 1.2 オプション型 (`claude/options.go`)

```go
package claude

// Options はClaude SDKの設定を表す
type Options struct {
    // CLI設定
    CLIPath string // CLIのパス（デフォルト: "claude"）
    CWD     string // 作業ディレクトリ

    // プロンプト設定
    SystemPrompt       string
    AppendSystemPrompt string

    // モデル設定
    Model         string
    FallbackModel string

    // 制限設定
    MaxTurns     int
    MaxBudgetUSD float64

    // 権限設定
    PermissionMode   PermissionMode
    AllowedTools     []string
    DisallowedTools  []string

    // セッション設定
    Resume      string
    ForkSession bool

    // MCP設定
    MCPServers map[string]MCPServerConfig

    // フック設定
    Hooks *HookConfig

    // コールバック
    CanUseTool CanUseToolFunc
}

type PermissionMode string

const (
    PermissionModeDefault           PermissionMode = "default"
    PermissionModeAcceptEdits       PermissionMode = "acceptEdits"
    PermissionModePlan              PermissionMode = "plan"
    PermissionModeBypassPermissions PermissionMode = "bypassPermissions"
)

type MCPServerConfig struct {
    Type    string            // "stdio", "sse", "http"
    Command string            // stdio用
    Args    []string          // stdio用
    URL     string            // sse/http用
    Headers map[string]string // sse/http用
    Env     map[string]string
}

type CanUseToolFunc func(
    ctx context.Context,
    toolName string,
    input map[string]any,
    context *ToolPermissionContext,
) (*PermissionResult, error)

type ToolPermissionContext struct {
    SessionID          string
    PermissionSuggestions []PermissionSuggestion
    BlockedPath        string
}

type PermissionResult struct {
    Allow             bool
    UpdatedInput      map[string]any
    UpdatedPermissions []PermissionUpdate
    Message           string // deny時
    Interrupt         bool   // deny時
}
```

### 1.3 エラー型 (`claude/errors.go`)

```go
package claude

import "errors"

var (
    // CLIエラー
    ErrCLINotFound   = errors.New("claude CLI not found")
    ErrCLIConnection = errors.New("CLI connection error")
    ErrProcessExited = errors.New("CLI process exited unexpectedly")

    // プロトコルエラー
    ErrJSONDecode     = errors.New("JSON decode error")
    ErrMessageParse   = errors.New("message parse error")
    ErrControlTimeout = errors.New("control request timeout")
    ErrBufferOverflow = errors.New("JSON buffer overflow")

    // セッションエラー
    ErrSessionNotFound = errors.New("session not found")
)

// SDKError はエラーの詳細情報を含む
type SDKError struct {
    Op      string // 操作名
    Err     error  // 元エラー
    Details string // 追加情報
}

func (e *SDKError) Error() string {
    if e.Details != "" {
        return e.Op + ": " + e.Err.Error() + " (" + e.Details + ")"
    }
    return e.Op + ": " + e.Err.Error()
}

func (e *SDKError) Unwrap() error {
    return e.Err
}
```

### 1.4 メッセージ型 (`internal/protocol/messages.go`)

```go
package protocol

// Message はCLIとやり取りするメッセージの共通インターフェース
type Message interface {
    MessageType() string
}

// UserMessage はユーザーからのメッセージ
type UserMessage struct {
    Type            string      `json:"type"` // "user"
    Message         UserContent `json:"message"`
    ParentToolUseID *string     `json:"parent_tool_use_id,omitempty"`
    SessionID       string      `json:"session_id"`
}

type UserContent struct {
    Role    string `json:"role"` // "user"
    Content any    `json:"content"` // string or []ContentBlock
}

// AssistantMessage はアシスタントからのメッセージ
type AssistantMessage struct {
    Type            string         `json:"type"` // "assistant"
    Message         AssistantBody  `json:"message"`
    ParentToolUseID *string        `json:"parent_tool_use_id,omitempty"`
}

type AssistantBody struct {
    Role    string         `json:"role"` // "assistant"
    Model   string         `json:"model"`
    Content []ContentBlock `json:"content"`
    Error   *string        `json:"error,omitempty"`
}

// ContentBlock はメッセージ内のコンテンツブロック
type ContentBlock struct {
    Type string `json:"type"`
    // type別のフィールド
    Text      string          `json:"text,omitempty"`      // type: "text"
    Thinking  string          `json:"thinking,omitempty"`  // type: "thinking"
    Signature string          `json:"signature,omitempty"` // type: "thinking"
    ID        string          `json:"id,omitempty"`        // type: "tool_use"
    Name      string          `json:"name,omitempty"`      // type: "tool_use"
    Input     map[string]any  `json:"input,omitempty"`     // type: "tool_use"
}

// SystemMessage はシステムメッセージ
type SystemMessage struct {
    Type    string         `json:"type"` // "system"
    Subtype string         `json:"subtype"`
    Data    map[string]any `json:"data"`
}

// ResultMessage は結果メッセージ
type ResultMessage struct {
    Type             string         `json:"type"` // "result"
    Subtype          string         `json:"subtype"` // "query_complete"
    DurationMs       int64          `json:"duration_ms"`
    DurationAPIMs    int64          `json:"duration_api_ms"`
    IsError          bool           `json:"is_error"`
    NumTurns         int            `json:"num_turns"`
    SessionID        string         `json:"session_id"`
    TotalCostUSD     float64        `json:"total_cost_usd"`
    Usage            Usage          `json:"usage"`
    Result           string         `json:"result,omitempty"`
    StructuredOutput map[string]any `json:"structured_output,omitempty"`
}

type Usage struct {
    InputTokens       int `json:"input_tokens"`
    OutputTokens      int `json:"output_tokens"`
    CacheCreationTokens int `json:"cache_creation_input_tokens,omitempty"`
    CacheReadTokens   int `json:"cache_read_input_tokens,omitempty"`
}

// ControlRequest は制御リクエスト
type ControlRequest struct {
    Type      string `json:"type"` // "control_request"
    RequestID string `json:"request_id"`
    Request   any    `json:"request"`
}

// ControlResponse は制御レスポンス
type ControlResponse struct {
    Type     string              `json:"type"` // "control_response"
    Response ControlResponseBody `json:"response"`
}

type ControlResponseBody struct {
    Subtype   string `json:"subtype"` // "success" or "error"
    RequestID string `json:"request_id"`
    Response  any    `json:"response,omitempty"`
    Error     string `json:"error,omitempty"`
}
```

## テスト

- 型のJSON marshal/unmarshalテスト
- エラー型のUnwrapテスト

## 完了条件

- [ ] go.modが作成されている
- [ ] 全ての型がコンパイルできる
- [ ] 基本的なJSONシリアライズテストが通る
