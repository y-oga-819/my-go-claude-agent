package permission

import (
	"context"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager(ModeDefault)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.mode != ModeDefault {
		t.Errorf("mode = %q, want %q", m.mode, ModeDefault)
	}
}

func TestManager_SetMode(t *testing.T) {
	m := NewManager(ModeDefault)

	m.SetMode(ModeAcceptEdits)
	if m.GetMode() != ModeAcceptEdits {
		t.Errorf("mode = %q, want %q", m.GetMode(), ModeAcceptEdits)
	}
}

func TestManager_Evaluate_BypassPermissions(t *testing.T) {
	m := NewManager(ModeBypassPermissions)

	ctx := context.Background()
	result, err := m.Evaluate(ctx, "Bash", nil, nil)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if !result.Allow {
		t.Error("bypassPermissions mode should always allow")
	}
}

func TestManager_Evaluate_WithCallback(t *testing.T) {
	m := NewManager(ModeDefault)

	called := false
	m.SetCanUseToolCallback(func(
		ctx context.Context,
		toolName string,
		input map[string]any,
		permCtx *ToolPermissionContext,
	) (*Result, error) {
		called = true
		if toolName == "Bash" {
			return &Result{Allow: false, Message: "Bash not allowed"}, nil
		}
		return &Result{Allow: true}, nil
	})

	ctx := context.Background()

	// Bashは拒否
	result, err := m.Evaluate(ctx, "Bash", nil, nil)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if result.Allow {
		t.Error("Bash should be denied")
	}
	if !called {
		t.Error("callback was not called")
	}

	// Readは許可
	result, err = m.Evaluate(ctx, "Read", nil, nil)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !result.Allow {
		t.Error("Read should be allowed")
	}
}

func TestManager_AddRule_Allow(t *testing.T) {
	m := NewManager(ModeDefault)

	err := m.AddRule(Rule{
		ToolName:    "Bash",
		RuleContent: "Allow Bash",
		Behavior:    BehaviorAllow,
	})
	if err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	ctx := context.Background()
	result, err := m.Evaluate(ctx, "Bash", nil, nil)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if !result.Allow {
		t.Error("Bash should be allowed by rule")
	}
}

func TestManager_AddRule_Deny(t *testing.T) {
	m := NewManager(ModeDefault)

	err := m.AddRule(Rule{
		ToolName:    "Bash",
		RuleContent: "Deny dangerous commands",
		Behavior:    BehaviorDeny,
	})
	if err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	ctx := context.Background()
	result, err := m.Evaluate(ctx, "Bash", nil, nil)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if result.Allow {
		t.Error("Bash should be denied by rule")
	}
	if result.Message == "" {
		t.Error("Message should be set for denied")
	}
}

func TestManager_AddRule_Regex(t *testing.T) {
	m := NewManager(ModeDefault)

	err := m.AddRule(Rule{
		ToolName:    ".*Edit.*",
		RuleContent: "Allow edit tools",
		Behavior:    BehaviorAllow,
	})
	if err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	ctx := context.Background()

	// Edit にマッチ
	result, err := m.Evaluate(ctx, "Edit", nil, nil)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !result.Allow {
		t.Error("Edit should be allowed")
	}

	// NotebookEdit にマッチ
	result, err = m.Evaluate(ctx, "NotebookEdit", nil, nil)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !result.Allow {
		t.Error("NotebookEdit should be allowed")
	}
}

func TestManager_ClearRules(t *testing.T) {
	m := NewManager(ModeDefault)

	err := m.AddRule(Rule{
		ToolName: "Bash",
		Behavior: BehaviorDeny,
	})
	if err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	m.ClearRules()

	// ルールがクリアされた後はデフォルト動作
	ctx := context.Background()
	result, err := m.Evaluate(ctx, "Bash", nil, nil)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// デフォルトでは許可
	if !result.Allow {
		t.Error("should be allowed after clearing rules")
	}
}

func TestManager_Evaluate_PlanMode(t *testing.T) {
	m := NewManager(ModePlan)

	ctx := context.Background()

	// 読み取り専用ツールは許可
	result, err := m.Evaluate(ctx, "Read", nil, nil)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !result.Allow {
		t.Error("Read should be allowed in plan mode")
	}

	// 書き込みツールは拒否
	result, err = m.Evaluate(ctx, "Write", nil, nil)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if result.Allow {
		t.Error("Write should be denied in plan mode")
	}
}

func TestManager_Evaluate_AcceptEditsMode(t *testing.T) {
	m := NewManager(ModeAcceptEdits)

	ctx := context.Background()

	// 編集ツールは許可
	result, err := m.Evaluate(ctx, "Edit", nil, nil)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !result.Allow {
		t.Error("Edit should be allowed in acceptEdits mode")
	}

	result, err = m.Evaluate(ctx, "Write", nil, nil)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !result.Allow {
		t.Error("Write should be allowed in acceptEdits mode")
	}
}

func TestModeConstants(t *testing.T) {
	modes := []Mode{
		ModeDefault,
		ModeAcceptEdits,
		ModePlan,
		ModeBypassPermissions,
	}

	for _, mode := range modes {
		if mode == "" {
			t.Error("Mode should not be empty")
		}
	}
}

func TestBehaviorConstants(t *testing.T) {
	behaviors := []Behavior{
		BehaviorAllow,
		BehaviorDeny,
		BehaviorAsk,
	}

	for _, b := range behaviors {
		if b == "" {
			t.Error("Behavior should not be empty")
		}
	}
}

func TestIsEditTool(t *testing.T) {
	editTools := []string{"Edit", "Write", "NotebookEdit"}
	for _, tool := range editTools {
		if !isEditTool(tool) {
			t.Errorf("%s should be an edit tool", tool)
		}
	}

	nonEditTools := []string{"Read", "Glob", "Bash"}
	for _, tool := range nonEditTools {
		if isEditTool(tool) {
			t.Errorf("%s should not be an edit tool", tool)
		}
	}
}

func TestIsReadOnlyTool(t *testing.T) {
	readOnlyTools := []string{"Read", "Glob", "Grep", "LSP", "WebFetch", "WebSearch"}
	for _, tool := range readOnlyTools {
		if !isReadOnlyTool(tool) {
			t.Errorf("%s should be a read-only tool", tool)
		}
	}

	nonReadOnlyTools := []string{"Edit", "Write", "Bash"}
	for _, tool := range nonReadOnlyTools {
		if isReadOnlyTool(tool) {
			t.Errorf("%s should not be a read-only tool", tool)
		}
	}
}

func TestResult_Structure(t *testing.T) {
	result := &Result{
		Allow:   false,
		Message: "Not allowed",
		UpdatedPermissions: []PermissionUpdate{
			{Tool: "Bash", Prompt: "run commands"},
		},
		Interrupt: true,
	}

	if result.Allow {
		t.Error("Allow should be false")
	}
	if result.Message != "Not allowed" {
		t.Errorf("Message = %q, want %q", result.Message, "Not allowed")
	}
	if len(result.UpdatedPermissions) != 1 {
		t.Errorf("len(UpdatedPermissions) = %d, want 1", len(result.UpdatedPermissions))
	}
}
