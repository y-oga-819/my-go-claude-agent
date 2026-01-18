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
	PreToolUse       []HookEntry
	PostToolUse      []HookEntry
	UserPromptSubmit []HookEntry
	Notification     []HookEntry
	Stop             []HookEntry
	SubagentStop     []HookEntry
	PreCompact       []HookEntry
}

// HookType はフックの種類
type HookType string

const (
	HookTypeCallback HookType = "callback" // Goコールバック
	HookTypeCommand  HookType = "command"  // シェルコマンド
)

// HookEntry はフックエントリを表す
type HookEntry struct {
	Type     HookType      // フックの種類（デフォルト: callback）
	Matcher  string        // ツール名パターン（正規表現対応）
	Command  string        // Type=command時のシェルコマンド
	Callback HookCallback  // Type=callback時のコールバック関数
	Timeout  time.Duration // タイムアウト（デフォルト: 60秒）
}

// HookCallback はフックのコールバック関数の型
type HookCallback func(ctx context.Context, input *HookInput) (*HookOutput, error)

// HookInput はフックへの入力
type HookInput struct {
	HookEventName  string
	SessionID      string
	TranscriptPath string
	CWD            string
	ToolName       string
	ToolInput      map[string]any
	ToolOutput     map[string]any
}

// HookOutput はフックからの出力
type HookOutput struct {
	Continue           bool
	StopReason         string
	SuppressOutput     bool
	Decision           string // "block" for explicit block
	SystemMessage      string
	Reason             string
	HookSpecificOutput *HookSpecificOutput
}

// HookSpecificOutput はフック固有の出力
type HookSpecificOutput struct {
	HookEventName            string
	PermissionDecision       string // "allow", "deny", "ask"
	PermissionDecisionReason string
	UpdatedInput             map[string]any
	AdditionalContext        string
}
