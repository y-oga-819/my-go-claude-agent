package permission

import (
	"context"
	"regexp"
	"sync"
)

// Mode は権限モードを表す
type Mode string

const (
	ModeDefault           Mode = "default"
	ModeAcceptEdits       Mode = "acceptEdits"
	ModePlan              Mode = "plan"
	ModeBypassPermissions Mode = "bypassPermissions"
)

// Behavior は権限の振る舞いを表す
type Behavior string

const (
	BehaviorAllow Behavior = "allow"
	BehaviorDeny  Behavior = "deny"
	BehaviorAsk   Behavior = "ask"
)

// Rule は権限ルール
type Rule struct {
	ToolName    string   // ツール名（正規表現対応）
	RuleContent string   // ルール内容の説明
	Behavior    Behavior // "allow", "deny", "ask"
	regex       *regexp.Regexp
}

// ToolPermissionContext はツール権限判定時のコンテキスト情報
type ToolPermissionContext struct {
	SessionID             string
	PermissionSuggestions []PermissionSuggestion
	BlockedPath           string
}

// PermissionSuggestion は権限の提案
type PermissionSuggestion struct {
	Tool   string
	Prompt string
}

// Result はツール使用許可の結果
type Result struct {
	Allow              bool
	UpdatedInput       map[string]any
	UpdatedPermissions []PermissionUpdate
	Message            string // deny時
	Interrupt          bool   // deny時
}

// PermissionUpdate は権限の更新
type PermissionUpdate struct {
	Tool   string
	Prompt string
}

// CanUseToolFunc はツール使用可否を判定するコールバック関数の型
type CanUseToolFunc func(
	ctx context.Context,
	toolName string,
	input map[string]any,
	context *ToolPermissionContext,
) (*Result, error)

// Manager は権限判定を管理
type Manager struct {
	mode       Mode
	rules      []Rule
	canUseTool CanUseToolFunc
	mu         sync.RWMutex
}

// NewManager は新しいManagerを作成する
func NewManager(mode Mode) *Manager {
	return &Manager{
		mode:  mode,
		rules: make([]Rule, 0),
	}
}

// SetMode は権限モードを設定する
func (m *Manager) SetMode(mode Mode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mode = mode
}

// GetMode は現在の権限モードを取得する
func (m *Manager) GetMode() Mode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.mode
}

// SetCanUseToolCallback はツール使用許可コールバックを設定する
func (m *Manager) SetCanUseToolCallback(cb CanUseToolFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.canUseTool = cb
}

// AddRule は権限ルールを追加する
func (m *Manager) AddRule(rule Rule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 正規表現のコンパイル
	if rule.ToolName != "" {
		re, err := regexp.Compile(rule.ToolName)
		if err != nil {
			return err
		}
		rule.regex = re
	}

	m.rules = append(m.rules, rule)
	return nil
}

// ClearRules は全てのルールをクリアする
func (m *Manager) ClearRules() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules = make([]Rule, 0)
}

// Evaluate はツール使用の許可を判定する
func (m *Manager) Evaluate(
	ctx context.Context,
	toolName string,
	input map[string]any,
	permContext *ToolPermissionContext,
) (*Result, error) {
	m.mu.RLock()
	mode := m.mode
	rules := m.rules
	cb := m.canUseTool
	m.mu.RUnlock()

	// bypassPermissionsモードの場合は常に許可
	if mode == ModeBypassPermissions {
		return &Result{Allow: true}, nil
	}

	// ルールによる判定
	for _, rule := range rules {
		if rule.matches(toolName) {
			switch rule.Behavior {
			case BehaviorAllow:
				return &Result{Allow: true}, nil
			case BehaviorDeny:
				return &Result{
					Allow:   false,
					Message: "Denied by rule: " + rule.RuleContent,
				}, nil
			case BehaviorAsk:
				// askの場合はコールバックに委ねる
				break
			}
		}
	}

	// コールバックが設定されている場合は呼び出す
	if cb != nil {
		return cb(ctx, toolName, input, permContext)
	}

	// デフォルト動作: モードに応じて判定
	switch mode {
	case ModeAcceptEdits:
		// acceptEditsモードでは編集系ツールを許可
		if isEditTool(toolName) {
			return &Result{Allow: true}, nil
		}
	case ModePlan:
		// planモードでは読み取り系ツールのみ許可
		if isReadOnlyTool(toolName) {
			return &Result{Allow: true}, nil
		}
		return &Result{Allow: false, Message: "Plan mode: write operations not allowed"}, nil
	}

	// デフォルトで許可（実際のCLIで権限確認される）
	return &Result{Allow: true}, nil
}

func (r *Rule) matches(toolName string) bool {
	if r.regex != nil {
		return r.regex.MatchString(toolName)
	}
	return r.ToolName == toolName
}

// isEditTool は編集系ツールかを判定する
func isEditTool(toolName string) bool {
	editTools := map[string]bool{
		"Edit":         true,
		"Write":        true,
		"NotebookEdit": true,
	}
	return editTools[toolName]
}

// isReadOnlyTool は読み取り専用ツールかを判定する
func isReadOnlyTool(toolName string) bool {
	readOnlyTools := map[string]bool{
		"Read":      true,
		"Glob":      true,
		"Grep":      true,
		"LSP":       true,
		"WebFetch":  true,
		"WebSearch": true,
	}
	return readOnlyTools[toolName]
}
