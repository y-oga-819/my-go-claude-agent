package hooks

import (
	"context"
	"sync"
)

// Event はフックイベントの種類
type Event string

const (
	EventPreToolUse       Event = "PreToolUse"
	EventPostToolUse      Event = "PostToolUse"
	EventUserPromptSubmit Event = "UserPromptSubmit"
	EventStop             Event = "Stop"
	EventSubagentStop     Event = "SubagentStop"
	EventPreCompact       Event = "PreCompact"
	EventNotification     Event = "Notification"
)

// Input はフックへの入力
type Input struct {
	HookEventName  string
	SessionID      string
	TranscriptPath string
	CWD            string
	ToolName       string
	ToolInput      map[string]any
	ToolOutput     map[string]any // PostToolUse用
}

// Output はフックからの出力
type Output struct {
	Continue           bool
	StopReason         string
	SuppressOutput     bool
	Decision           string // "block" for explicit block
	SystemMessage      string
	Reason             string
	HookSpecificOutput *SpecificOutput
}

// SpecificOutput はフック固有の出力
type SpecificOutput struct {
	HookEventName            string
	PermissionDecision       string // "allow", "deny", "ask"
	PermissionDecisionReason string
	UpdatedInput             map[string]any
}

// Callback はフックのコールバック関数
type Callback func(ctx context.Context, input *Input) (*Output, error)

// Entry はフックエントリ
type Entry struct {
	Matcher  *Matcher
	Callback Callback
	Timeout  int // ミリ秒
}

// Manager はフックを管理する
type Manager struct {
	hooks map[Event][]Entry
	mu    sync.RWMutex
}

// NewManager は新しいManagerを作成する
func NewManager() *Manager {
	return &Manager{
		hooks: make(map[Event][]Entry),
	}
}

// Register はフックを登録する
func (m *Manager) Register(event Event, entry Entry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hooks[event] = append(m.hooks[event], entry)
}

// Trigger はフックをトリガーする
func (m *Manager) Trigger(ctx context.Context, event Event, input *Input) (*Output, error) {
	m.mu.RLock()
	entries := m.hooks[event]
	m.mu.RUnlock()

	if len(entries) == 0 {
		return &Output{Continue: true}, nil
	}

	input.HookEventName = string(event)

	for _, entry := range entries {
		// マッチャーが設定されている場合はマッチングを確認
		if entry.Matcher != nil && !entry.Matcher.Match(input.ToolName) {
			continue
		}

		output, err := entry.Callback(ctx, input)
		if err != nil {
			return nil, err
		}

		// 続行しない場合は即座に返す
		if !output.Continue {
			return output, nil
		}
	}

	return &Output{Continue: true}, nil
}

// GetHooks は登録されたフックを取得する
func (m *Manager) GetHooks(event Event) []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.hooks[event]
}

// Clear は全てのフックをクリアする
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hooks = make(map[Event][]Entry)
}
