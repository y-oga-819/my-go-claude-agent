package hooks

import (
	"context"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.hooks == nil {
		t.Error("hooks map should be initialized")
	}
}

func TestManager_Register(t *testing.T) {
	m := NewManager()

	entry := Entry{
		Callback: func(ctx context.Context, input *Input) (*Output, error) {
			return &Output{Continue: true}, nil
		},
	}

	m.Register(EventPreToolUse, entry)

	hooks := m.GetHooks(EventPreToolUse)
	if len(hooks) != 1 {
		t.Errorf("len(hooks) = %d, want 1", len(hooks))
	}
}

func TestManager_Trigger_NoHooks(t *testing.T) {
	m := NewManager()

	ctx := context.Background()
	input := &Input{ToolName: "Bash"}

	output, err := m.Trigger(ctx, EventPreToolUse, input)
	if err != nil {
		t.Fatalf("Trigger failed: %v", err)
	}

	if !output.Continue {
		t.Error("Continue should be true when no hooks")
	}
}

func TestManager_Trigger_SingleHook(t *testing.T) {
	m := NewManager()

	called := false
	m.Register(EventPreToolUse, Entry{
		Callback: func(ctx context.Context, input *Input) (*Output, error) {
			called = true
			if input.ToolName != "Bash" {
				t.Errorf("ToolName = %q, want %q", input.ToolName, "Bash")
			}
			return &Output{Continue: true}, nil
		},
	})

	ctx := context.Background()
	input := &Input{ToolName: "Bash"}

	output, err := m.Trigger(ctx, EventPreToolUse, input)
	if err != nil {
		t.Fatalf("Trigger failed: %v", err)
	}

	if !called {
		t.Error("callback was not called")
	}
	if !output.Continue {
		t.Error("Continue should be true")
	}
}

func TestManager_Trigger_StopOnFalse(t *testing.T) {
	m := NewManager()

	firstCalled := false
	secondCalled := false

	m.Register(EventPreToolUse, Entry{
		Callback: func(ctx context.Context, input *Input) (*Output, error) {
			firstCalled = true
			return &Output{Continue: false, Reason: "blocked"}, nil
		},
	})

	m.Register(EventPreToolUse, Entry{
		Callback: func(ctx context.Context, input *Input) (*Output, error) {
			secondCalled = true
			return &Output{Continue: true}, nil
		},
	})

	ctx := context.Background()
	input := &Input{ToolName: "Bash"}

	output, err := m.Trigger(ctx, EventPreToolUse, input)
	if err != nil {
		t.Fatalf("Trigger failed: %v", err)
	}

	if !firstCalled {
		t.Error("first callback should be called")
	}
	if secondCalled {
		t.Error("second callback should not be called")
	}
	if output.Continue {
		t.Error("Continue should be false")
	}
	if output.Reason != "blocked" {
		t.Errorf("Reason = %q, want %q", output.Reason, "blocked")
	}
}

func TestManager_Trigger_WithMatcher(t *testing.T) {
	m := NewManager()

	bashCalled := false
	otherCalled := false

	m.Register(EventPreToolUse, Entry{
		Matcher: NewMatcher("Bash"),
		Callback: func(ctx context.Context, input *Input) (*Output, error) {
			bashCalled = true
			return &Output{Continue: true}, nil
		},
	})

	m.Register(EventPreToolUse, Entry{
		Matcher: NewMatcher("Read"),
		Callback: func(ctx context.Context, input *Input) (*Output, error) {
			otherCalled = true
			return &Output{Continue: true}, nil
		},
	})

	ctx := context.Background()
	input := &Input{ToolName: "Bash"}

	_, err := m.Trigger(ctx, EventPreToolUse, input)
	if err != nil {
		t.Fatalf("Trigger failed: %v", err)
	}

	if !bashCalled {
		t.Error("Bash callback should be called")
	}
	if otherCalled {
		t.Error("Read callback should not be called")
	}
}

func TestManager_Clear(t *testing.T) {
	m := NewManager()

	m.Register(EventPreToolUse, Entry{
		Callback: func(ctx context.Context, input *Input) (*Output, error) {
			return &Output{Continue: true}, nil
		},
	})

	m.Clear()

	hooks := m.GetHooks(EventPreToolUse)
	if len(hooks) != 0 {
		t.Errorf("len(hooks) = %d, want 0", len(hooks))
	}
}

func TestNewMatcher_Empty(t *testing.T) {
	m := NewMatcher("")

	// 空パターンは全てにマッチ
	if !m.Match("Bash") {
		t.Error("empty pattern should match any tool")
	}
	if !m.Match("Read") {
		t.Error("empty pattern should match any tool")
	}
}

func TestNewMatcher_Exact(t *testing.T) {
	m := NewMatcher("Bash")

	if !m.Match("Bash") {
		t.Error("should match exact name")
	}
	if m.Match("Read") {
		t.Error("should not match different name")
	}
	if m.Match("BashScript") {
		t.Error("should not match substring")
	}
}

func TestNewMatcher_Regex(t *testing.T) {
	m := NewMatcher("Bash.*")

	if !m.Match("Bash") {
		t.Error("should match Bash")
	}
	if !m.Match("BashScript") {
		t.Error("should match BashScript")
	}
	if m.Match("Read") {
		t.Error("should not match Read")
	}
}

func TestNewMatcher_RegexAll(t *testing.T) {
	m := NewMatcher(".*")

	if !m.Match("Bash") {
		t.Error("should match any tool")
	}
	if !m.Match("Read") {
		t.Error("should match any tool")
	}
}

func TestMatcher_Properties(t *testing.T) {
	exact := NewMatcher("Bash")
	if exact.Pattern() != "Bash" {
		t.Errorf("Pattern() = %q, want %q", exact.Pattern(), "Bash")
	}
	if !exact.IsExact() {
		t.Error("IsExact() should be true for exact match")
	}

	regex := NewMatcher("Bash.*")
	if regex.Pattern() != "Bash.*" {
		t.Errorf("Pattern() = %q, want %q", regex.Pattern(), "Bash.*")
	}
	if regex.IsExact() {
		t.Error("IsExact() should be false for regex")
	}
}

func TestEventConstants(t *testing.T) {
	events := []Event{
		EventPreToolUse,
		EventPostToolUse,
		EventUserPromptSubmit,
		EventStop,
		EventSubagentStop,
		EventPreCompact,
		EventNotification,
	}

	// イベント名が空でないことを確認
	for _, e := range events {
		if e == "" {
			t.Error("Event should not be empty")
		}
	}
}

func TestManager_Trigger_SetsHookEventName(t *testing.T) {
	m := NewManager()

	m.Register(EventPreToolUse, Entry{
		Callback: func(ctx context.Context, input *Input) (*Output, error) {
			if input.HookEventName != string(EventPreToolUse) {
				t.Errorf("HookEventName = %q, want %q", input.HookEventName, EventPreToolUse)
			}
			return &Output{Continue: true}, nil
		},
	})

	ctx := context.Background()
	input := &Input{ToolName: "Bash"}

	_, err := m.Trigger(ctx, EventPreToolUse, input)
	if err != nil {
		t.Fatalf("Trigger failed: %v", err)
	}
}

func TestManager_Trigger_CommandHook(t *testing.T) {
	m := NewManager()

	// コマンドフックを登録
	m.Register(EventPreToolUse, Entry{
		Type:    HookTypeCommand,
		Command: `echo '{"continue": true}'`,
	})

	ctx := context.Background()
	input := &Input{
		SessionID: "test-session",
		ToolName:  "Bash",
	}

	output, err := m.Trigger(ctx, EventPreToolUse, input)
	if err != nil {
		t.Fatalf("Trigger failed: %v", err)
	}
	if !output.Continue {
		t.Error("Continue should be true")
	}
}

func TestManager_Trigger_CommandHookBlock(t *testing.T) {
	m := NewManager()

	// ブロックするコマンドフックを登録
	m.Register(EventPreToolUse, Entry{
		Type:    HookTypeCommand,
		Command: `echo "blocked" >&2; exit 2`,
	})

	ctx := context.Background()
	input := &Input{
		SessionID: "test-session",
		ToolName:  "Bash",
	}

	output, err := m.Trigger(ctx, EventPreToolUse, input)
	if err != nil {
		t.Fatalf("Trigger failed: %v", err)
	}
	if output.Continue {
		t.Error("Continue should be false for blocking hook")
	}
}

func TestManager_Trigger_MixedHooks(t *testing.T) {
	m := NewManager()

	callbackCalled := false

	// コールバックフックを登録
	m.Register(EventPreToolUse, Entry{
		Type: HookTypeCallback,
		Callback: func(ctx context.Context, input *Input) (*Output, error) {
			callbackCalled = true
			return &Output{Continue: true}, nil
		},
	})

	// コマンドフックを登録
	m.Register(EventPreToolUse, Entry{
		Type:    HookTypeCommand,
		Command: `echo '{"continue": true}'`,
	})

	ctx := context.Background()
	input := &Input{
		SessionID: "test-session",
		ToolName:  "Bash",
	}

	output, err := m.Trigger(ctx, EventPreToolUse, input)
	if err != nil {
		t.Fatalf("Trigger failed: %v", err)
	}
	if !callbackCalled {
		t.Error("callback should be called")
	}
	if !output.Continue {
		t.Error("Continue should be true")
	}
}

func TestManager_Trigger_CommandHookWithMatcher(t *testing.T) {
	m := NewManager()

	// Bashにのみマッチするコマンドフック
	m.Register(EventPreToolUse, Entry{
		Type:    HookTypeCommand,
		Matcher: NewMatcher("Bash"),
		Command: `echo '{"continue": false, "reason": "Bash blocked"}'`,
	})

	ctx := context.Background()

	// Bashツールはブロックされる
	input := &Input{SessionID: "test-session", ToolName: "Bash"}
	output, err := m.Trigger(ctx, EventPreToolUse, input)
	if err != nil {
		t.Fatalf("Trigger failed: %v", err)
	}
	if output.Continue {
		t.Error("Bash should be blocked")
	}

	// Readツールは通過
	input2 := &Input{SessionID: "test-session", ToolName: "Read"}
	output2, err := m.Trigger(ctx, EventPreToolUse, input2)
	if err != nil {
		t.Fatalf("Trigger failed: %v", err)
	}
	if !output2.Continue {
		t.Error("Read should continue")
	}
}
