package claude

import (
	"context"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient(nil)
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.opts == nil {
		t.Error("opts should not be nil")
	}
}

func TestNewClient_WithOptions(t *testing.T) {
	opts := &Options{
		CLIPath:      "/custom/claude",
		Model:        "claude-3-opus",
		MaxTurns:     10,
		SystemPrompt: "You are a helpful assistant",
	}

	client := NewClient(opts)
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.opts.CLIPath != "/custom/claude" {
		t.Errorf("CLIPath = %q, want %q", client.opts.CLIPath, "/custom/claude")
	}
	if client.opts.Model != "claude-3-opus" {
		t.Errorf("Model = %q, want %q", client.opts.Model, "claude-3-opus")
	}
}

func TestClient_Close(t *testing.T) {
	client := NewClient(nil)

	// 接続していない状態でもCloseは成功する
	if err := client.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// 複数回Closeしてもエラーにならない
	if err := client.Close(); err != nil {
		t.Errorf("second Close failed: %v", err)
	}
}

func TestClient_Send_NotConnected(t *testing.T) {
	client := NewClient(nil)

	ctx := context.Background()
	err := client.Send(ctx, "Hello")

	if err == nil {
		t.Error("Send should fail when not connected")
	}
}

func TestClient_Interrupt_NotConnected(t *testing.T) {
	client := NewClient(nil)

	ctx := context.Background()
	err := client.Interrupt(ctx)

	if err == nil {
		t.Error("Interrupt should fail when not connected")
	}
}

func TestClient_SessionID_NotConnected(t *testing.T) {
	client := NewClient(nil)

	sessionID := client.SessionID()
	if sessionID != "" {
		t.Errorf("SessionID should be empty when not connected, got %q", sessionID)
	}
}

func TestClient_Connect_CLINotFound(t *testing.T) {
	client := NewClient(&Options{
		CLIPath: "/nonexistent/claude",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Connect(ctx)
	if err == nil {
		t.Error("Connect should fail for non-existent CLI")
	}

	// SDKErrorにラップされていることを確認
	sdkErr, ok := err.(*SDKError)
	if !ok {
		t.Fatalf("expected *SDKError, got %T", err)
	}

	if sdkErr.Op != "connect" {
		t.Errorf("Op = %q, want %q", sdkErr.Op, "connect")
	}
}

func TestStream_Methods(t *testing.T) {
	// Streamがclientのメソッドを正しく委譲することを確認
	client := NewClient(nil)
	stream := &Stream{client: client}

	// SessionID
	if stream.SessionID() != "" {
		t.Error("SessionID should be empty")
	}

	// Close
	if err := stream.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestConvertPermissionSuggestions(t *testing.T) {
	// nil入力
	result := convertPermissionSuggestions(nil)
	if result != nil {
		t.Error("should return nil for nil input")
	}

	// 変換テスト（内部関数なのでパッケージ内からアクセス可能）
}

func TestConvertPermissionUpdates(t *testing.T) {
	// nil入力
	result := convertPermissionUpdates(nil)
	if result != nil {
		t.Error("should return nil for nil input")
	}
}

func TestClient_Messages_Errors_Channels(t *testing.T) {
	// 接続していない状態ではnilを返す可能性がある
	// これはprotocolがnilのため
	// 実際の使用では接続後にのみアクセスすることを想定
}

func TestClient_WithHooks(t *testing.T) {
	opts := &Options{
		Hooks: &HookConfig{
			UserPromptSubmit: []HookEntry{
				{
					Type: HookTypeCallback,
					Callback: func(ctx context.Context, input *HookInput) (*HookOutput, error) {
						return &HookOutput{Continue: true}, nil
					},
				},
			},
		},
	}

	client := NewClient(opts)
	if client.hookManager == nil {
		t.Fatal("hookManager should be initialized")
	}

	// フックが登録されていることを確認
	hooks := client.HookManager()
	if hooks == nil {
		t.Fatal("HookManager() should not return nil")
	}
}

func TestClient_WithCommandHook(t *testing.T) {
	opts := &Options{
		Hooks: &HookConfig{
			PreToolUse: []HookEntry{
				{
					Type:    HookTypeCommand,
					Matcher: "Bash",
					Command: "echo 'test'",
					Timeout: 5 * time.Second,
				},
			},
		},
	}

	client := NewClient(opts)
	if client.hookManager == nil {
		t.Fatal("hookManager should be initialized")
	}
}

func TestClient_WithSessionOptions(t *testing.T) {
	opts := &Options{
		Resume:                  "session-123",
		ForkSession:             true,
		Continue:                false,
		EnableFileCheckpointing: true,
	}

	client := NewClient(opts)
	if client.opts.Resume != "session-123" {
		t.Errorf("Resume = %q, want %q", client.opts.Resume, "session-123")
	}
	if !client.opts.ForkSession {
		t.Error("ForkSession should be true")
	}
	if client.opts.Continue {
		t.Error("Continue should be false")
	}
	if !client.opts.EnableFileCheckpointing {
		t.Error("EnableFileCheckpointing should be true")
	}
}

func TestClient_RewindFiles_NotConnected(t *testing.T) {
	client := NewClient(nil)

	ctx := context.Background()
	err := client.RewindFiles(ctx, "msg-uuid-123")

	if err == nil {
		t.Error("RewindFiles should fail when not connected")
	}
}
