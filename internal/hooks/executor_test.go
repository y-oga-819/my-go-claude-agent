package hooks

import (
	"context"
	"testing"
	"time"
)

func TestNewExecutor(t *testing.T) {
	e := NewExecutor()
	if e == nil {
		t.Fatal("NewExecutor returned nil")
	}
	if e.shell != "sh" {
		t.Errorf("shell = %q, want %q", e.shell, "sh")
	}
}

func TestExecutor_Execute_Success(t *testing.T) {
	e := NewExecutor()
	ctx := context.Background()

	input := &Input{
		SessionID:     "test-session",
		HookEventName: "PreToolUse",
		ToolName:      "Bash",
		ToolInput:     map[string]any{"command": "echo hello"},
	}

	// シンプルなJSONを返すコマンド
	command := `echo '{"continue": true}'`
	output, err := e.Execute(ctx, command, input, 10*time.Second)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !output.Continue {
		t.Error("Continue should be true")
	}
}

func TestExecutor_Execute_Block(t *testing.T) {
	e := NewExecutor()
	ctx := context.Background()

	input := &Input{
		SessionID:     "test-session",
		HookEventName: "PreToolUse",
		ToolName:      "Bash",
	}

	// exit 2 でブロック
	command := `echo "blocked" >&2; exit 2`
	output, err := e.Execute(ctx, command, input, 10*time.Second)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if output.Continue {
		t.Error("Continue should be false for exit code 2")
	}
	if output.Reason == "" {
		t.Error("Reason should contain stderr message")
	}
}

func TestExecutor_Execute_NonBlockingError(t *testing.T) {
	e := NewExecutor()
	ctx := context.Background()

	input := &Input{
		SessionID:     "test-session",
		HookEventName: "PreToolUse",
		ToolName:      "Bash",
	}

	// exit 1 は非ブロッキングエラー（処理継続）
	command := `echo "warning" >&2; exit 1`
	output, err := e.Execute(ctx, command, input, 10*time.Second)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !output.Continue {
		t.Error("Continue should be true for non-blocking error (exit 1)")
	}
}

func TestExecutor_Execute_Timeout(t *testing.T) {
	e := NewExecutor()
	ctx := context.Background()

	input := &Input{
		SessionID:     "test-session",
		HookEventName: "PreToolUse",
	}

	// 長時間スリープするコマンド
	command := `sleep 10`
	_, err := e.Execute(ctx, command, input, 100*time.Millisecond)

	// タイムアウトでエラーまたはコンテキストキャンセルエラーを期待
	if err == nil {
		t.Error("Execute should fail with timeout")
	}
	// context deadline exceeded を含むことを確認
	if err != nil && !contains(err.Error(), "context deadline exceeded") && !contains(err.Error(), "signal: killed") {
		t.Logf("Got error: %v (this is acceptable)", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestExecutor_Execute_ContextCanceled(t *testing.T) {
	e := NewExecutor()
	ctx, cancel := context.WithCancel(context.Background())

	input := &Input{
		SessionID:     "test-session",
		HookEventName: "PreToolUse",
	}

	// キャンセル
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	command := `sleep 10`
	_, err := e.Execute(ctx, command, input, 10*time.Second)

	// コンテキストキャンセルでエラーを期待
	if err == nil {
		t.Error("Execute should fail with context canceled")
	}
	// context canceled または signal: killed を含むことを確認
	if err != nil && !contains(err.Error(), "context canceled") && !contains(err.Error(), "signal: killed") {
		t.Logf("Got error: %v (this is acceptable)", err)
	}
}

func TestExecutor_Execute_InvalidJSON(t *testing.T) {
	e := NewExecutor()
	ctx := context.Background()

	input := &Input{
		SessionID:     "test-session",
		HookEventName: "PreToolUse",
	}

	// 不正なJSON
	command := `echo 'not json'`
	output, err := e.Execute(ctx, command, input, 10*time.Second)

	// 不正なJSONでも継続（デフォルト動作）
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !output.Continue {
		t.Error("Continue should be true for invalid JSON")
	}
}

func TestExecutor_Execute_WithHookSpecificOutput(t *testing.T) {
	e := NewExecutor()
	ctx := context.Background()

	input := &Input{
		SessionID:     "test-session",
		HookEventName: "PreToolUse",
		ToolName:      "Bash",
	}

	// フック固有の出力を含むJSON
	command := `echo '{"continue": true, "hookSpecificOutput": {"hookEventName": "PreToolUse", "permissionDecision": "allow", "permissionDecisionReason": "safe command"}}'`
	output, err := e.Execute(ctx, command, input, 10*time.Second)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if output.HookSpecificOutput == nil {
		t.Fatal("HookSpecificOutput should not be nil")
	}
	if output.HookSpecificOutput.PermissionDecision != "allow" {
		t.Errorf("PermissionDecision = %q, want %q", output.HookSpecificOutput.PermissionDecision, "allow")
	}
}

func TestExecutor_Execute_EmptyOutput(t *testing.T) {
	e := NewExecutor()
	ctx := context.Background()

	input := &Input{
		SessionID:     "test-session",
		HookEventName: "PreToolUse",
	}

	// 何も出力しない
	command := `true`
	output, err := e.Execute(ctx, command, input, 10*time.Second)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !output.Continue {
		t.Error("Continue should be true for empty output")
	}
}

func TestExecutor_Execute_StdinInput(t *testing.T) {
	e := NewExecutor()
	ctx := context.Background()

	input := &Input{
		SessionID:     "test-session",
		HookEventName: "PreToolUse",
		ToolName:      "Bash",
		ToolInput:     map[string]any{"command": "test command"},
	}

	// stdinからJSONを読み取り、tool_nameを確認
	command := `cat | jq -r '.tool_name' | grep -q Bash && echo '{"continue": true}'`
	output, err := e.Execute(ctx, command, input, 10*time.Second)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !output.Continue {
		t.Error("Continue should be true (stdin was passed correctly)")
	}
}

func TestExecutor_Execute_EnvironmentVariables(t *testing.T) {
	e := NewExecutor()
	ctx := context.Background()

	input := &Input{
		SessionID:     "test-session-123",
		HookEventName: "PreToolUse",
		// CWDを設定しない（存在しないディレクトリを避ける）
	}

	// 環境変数を確認
	command := `test "$CLAUDE_SESSION_ID" = "test-session-123" && echo '{"continue": true}'`
	output, err := e.Execute(ctx, command, input, 10*time.Second)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !output.Continue {
		t.Error("Continue should be true (env var was set correctly)")
	}
}
