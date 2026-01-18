package claude

import (
	"context"
	"time"
)

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
	PermissionMode  PermissionMode
	AllowedTools    []string
	DisallowedTools []string

	// セッション設定
	Resume                  string // 再開するセッションID
	ForkSession             bool   // trueで分岐、falseで継続
	Continue                bool   // 直前のセッションを継続
	EnableFileCheckpointing bool   // ファイルチェックポイントを有効化

	// MCP設定
	MCPServers map[string]MCPServerConfig

	// フック設定
	Hooks *HookConfig

	// タイムアウト設定
	Timeout *TimeoutConfig

	// リトライ設定
	Retry *RetryConfig

	// コールバック
	CanUseTool CanUseToolFunc
}

// TimeoutConfig はタイムアウトの設定
type TimeoutConfig struct {
	// 接続タイムアウト（CLIプロセス起動）
	Connect time.Duration
	// 単一リクエストのタイムアウト
	Request time.Duration
	// 全体タイムアウト（セッション全体）
	Total time.Duration
	// 制御リクエストタイムアウト
	Control time.Duration
}

// DefaultTimeoutConfig はデフォルトのタイムアウト設定を返す
func DefaultTimeoutConfig() *TimeoutConfig {
	return &TimeoutConfig{
		Connect: 30 * time.Second,
		Request: 5 * time.Minute,
		Total:   30 * time.Minute,
		Control: 30 * time.Second,
	}
}

// GetTimeout は指定された種類のタイムアウトを取得する
func (o *Options) GetTimeout(kind string) time.Duration {
	if o.Timeout == nil {
		def := DefaultTimeoutConfig()
		switch kind {
		case "connect":
			return def.Connect
		case "request":
			return def.Request
		case "total":
			return def.Total
		case "control":
			return def.Control
		default:
			return def.Request
		}
	}

	switch kind {
	case "connect":
		if o.Timeout.Connect > 0 {
			return o.Timeout.Connect
		}
		return DefaultTimeoutConfig().Connect
	case "request":
		if o.Timeout.Request > 0 {
			return o.Timeout.Request
		}
		return DefaultTimeoutConfig().Request
	case "total":
		if o.Timeout.Total > 0 {
			return o.Timeout.Total
		}
		return DefaultTimeoutConfig().Total
	case "control":
		if o.Timeout.Control > 0 {
			return o.Timeout.Control
		}
		return DefaultTimeoutConfig().Control
	default:
		return DefaultTimeoutConfig().Request
	}
}

// GetRetryConfig はリトライ設定を取得する
func (o *Options) GetRetryConfig() *RetryConfig {
	if o.Retry != nil {
		return o.Retry
	}
	return nil // デフォルトではリトライしない
}

// PermissionMode は権限モードを表す
type PermissionMode string

const (
	PermissionModeDefault           PermissionMode = "default"
	PermissionModeAcceptEdits       PermissionMode = "acceptEdits"
	PermissionModePlan              PermissionMode = "plan"
	PermissionModeBypassPermissions PermissionMode = "bypassPermissions"
)

// MCPServerConfig はMCPサーバーの設定を表す
type MCPServerConfig struct {
	Type    string            // "stdio", "sse", "http"
	Command string            // stdio用
	Args    []string          // stdio用
	URL     string            // sse/http用
	Headers map[string]string // sse/http用
	Env     map[string]string
}

// CanUseToolFunc はツール使用可否を判定するコールバック関数の型
type CanUseToolFunc func(
	ctx context.Context,
	toolName string,
	input map[string]any,
	context *ToolPermissionContext,
) (*PermissionResult, error)

// ToolPermissionContext はツール権限判定時のコンテキスト情報
type ToolPermissionContext struct {
	SessionID             string
	PermissionSuggestions []PermissionSuggestion
	BlockedPath           string
}

// PermissionSuggestion は権限の提案を表す
type PermissionSuggestion struct {
	Tool   string `json:"tool"`
	Prompt string `json:"prompt"`
}

// PermissionUpdate は権限の更新を表す
type PermissionUpdate struct {
	Tool   string `json:"tool"`
	Prompt string `json:"prompt"`
}

// PermissionResult はツール使用許可の結果を表す
type PermissionResult struct {
	Allow              bool
	UpdatedInput       map[string]any
	UpdatedPermissions []PermissionUpdate
	Message            string // deny時
	Interrupt          bool   // deny時
}

// HookConfig はフックの設定を表す
type HookConfig struct {
	PreToolUse        []HookEntry
	PostToolUse       []HookEntry
	UserPromptSubmit  []HookEntry
	Notification      []HookEntry
	Stop              []HookEntry
}

// HookEntry はフックエントリを表す
type HookEntry struct {
	Matcher ToolMatcher
	Timeout int // ミリ秒
}

// ToolMatcher はツールマッチングの条件を表す
type ToolMatcher struct {
	ToolName string // 正規表現対応
}
