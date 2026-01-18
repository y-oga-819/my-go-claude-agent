# Step 5: フック・権限管理実装

## 目的

ツール実行前後のフック機能と、詳細な権限管理を実装する。

## 成果物

- `internal/hooks/hooks.go` - フック定義
- `internal/hooks/matcher.go` - ツールマッチャー
- `internal/permission/permission.go` - 権限管理

## 主要な実装

### 5.1 フックシステム

```go
// HookEvent はフックイベントの種類
type HookEvent string

const (
    HookPreToolUse        HookEvent = "PreToolUse"
    HookPostToolUse       HookEvent = "PostToolUse"
    HookUserPromptSubmit  HookEvent = "UserPromptSubmit"
    HookStop              HookEvent = "Stop"
    HookSubagentStop      HookEvent = "SubagentStop"
    HookPreCompact        HookEvent = "PreCompact"
)

// HookMatcher はフックの適用条件を定義
type HookMatcher struct {
    Event   HookEvent
    Matcher string  // ツール名パターン（正規表現、nilで全て）
    Timeout float64 // 秒単位
}

// HookCallback はフックのコールバック関数
type HookCallback func(input HookInput) (*HookOutput, error)

// HookInput はフックへの入力
type HookInput struct {
    HookEventName  string
    SessionID      string
    TranscriptPath string
    CWD            string
    ToolName       string
    ToolInput      map[string]any
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

type HookSpecificOutput struct {
    HookEventName           string
    PermissionDecision      string // "allow", "deny", "ask"
    PermissionDecisionReason string
    UpdatedInput            map[string]any
}
```

### 5.2 権限管理

```go
// PermissionManager は権限判定を管理
type PermissionManager struct {
    mode       PermissionMode
    rules      []PermissionRule
    canUseTool CanUseToolFunc
}

// PermissionRule は権限ルール
type PermissionRule struct {
    ToolName    string
    RuleContent string
    Behavior    string // "allow", "deny", "ask"
}

// Evaluate はツール使用の許可を判定
func (pm *PermissionManager) Evaluate(
    ctx context.Context,
    toolName string,
    input map[string]any,
    permContext *ToolPermissionContext,
) (*PermissionResult, error)
```

### 5.3 使用例

```go
client := claude.NewClient(&claude.Options{
    Hooks: &claude.HookConfig{
        PreToolUse: []claude.HookMatcher{
            {
                Matcher: "Bash",
                Callback: func(input claude.HookInput) (*claude.HookOutput, error) {
                    // rm -rf を含むコマンドをブロック
                    if strings.Contains(input.ToolInput["command"].(string), "rm -rf") {
                        return &claude.HookOutput{
                            Continue: false,
                            Decision: "block",
                            Reason:   "Dangerous command blocked",
                        }, nil
                    }
                    return &claude.HookOutput{Continue: true}, nil
                },
            },
        },
    },
    CanUseTool: func(ctx context.Context, toolName string, input map[string]any, pc *claude.ToolPermissionContext) (*claude.PermissionResult, error) {
        // カスタム権限ロジック
        return &claude.PermissionResult{Allow: true}, nil
    },
})
```

## 完了条件

- [ ] フックが正しく呼び出される
- [ ] ツールマッチャーが正規表現で動作する
- [ ] canUseToolコールバックが呼び出される
- [ ] 権限ルールが評価される
