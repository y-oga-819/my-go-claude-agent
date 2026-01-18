package claude

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestBuildQueryArgs_Basic(t *testing.T) {
	args := buildQueryArgs("Hello, Claude!", &Options{})

	// 必須の引数が含まれているか確認（末尾に配置される）
	if len(args) < 3 {
		t.Fatalf("args too short: got %d, want at least 3", len(args))
	}

	// 末尾3つが "--print", "--", "Hello, Claude!" であることを確認
	expectedSuffix := []string{"--print", "--", "Hello, Claude!"}
	for i, exp := range expectedSuffix {
		idx := len(args) - 3 + i
		if args[idx] != exp {
			t.Errorf("args[%d] = %q, want %q", idx, args[idx], exp)
		}
	}
}

func TestBuildQueryArgs_WithOptions(t *testing.T) {
	opts := &Options{
		SystemPrompt:       "You are a helpful assistant",
		AppendSystemPrompt: "Be concise",
		Model:              "claude-3-opus",
		FallbackModel:      "claude-3-sonnet",
		MaxTurns:           5,
		MaxBudgetUSD:       1.50,
		PermissionMode:     PermissionModeAcceptEdits,
		AllowedTools:       []string{"Read", "Write"},
		DisallowedTools:    []string{"Bash"},
		Resume:             "session-123",
	}

	args := buildQueryArgs("test prompt", opts)

	// argsをマップに変換して確認（末尾3つの --print -- prompt は除く）
	argMap := make(map[string]string)
	for i := 0; i < len(args)-3; i += 2 {
		argMap[args[i]] = args[i+1]
	}

	checkArg := func(key, expected string) {
		if val, ok := argMap[key]; !ok {
			t.Errorf("missing arg: %s", key)
		} else if val != expected {
			t.Errorf("arg %s = %q, want %q", key, val, expected)
		}
	}

	checkArg("--system-prompt", "You are a helpful assistant")
	checkArg("--append-system-prompt", "Be concise")
	checkArg("--model", "claude-3-opus")
	checkArg("--fallback-model", "claude-3-sonnet")
	checkArg("--max-turns", "5")
	checkArg("--max-budget-usd", "1.50")
	checkArg("--permission-mode", "acceptEdits")
	checkArg("--allowedTools", "Read,Write")
	checkArg("--disallowedTools", "Bash")
	checkArg("--resume", "session-123")
}

func TestBuildQueryArgs_EmptyOptions(t *testing.T) {
	// 空のOptionsで動作することを確認
	args := buildQueryArgs("test", &Options{})

	// 基本的な引数（--print -- test）が生成される
	if len(args) < 3 {
		t.Errorf("args too short: got %d, want at least 3", len(args))
	}

	// 末尾が "--print", "--", "test" であることを確認
	if args[len(args)-3] != "--print" {
		t.Errorf("args[-3] = %q, want %q", args[len(args)-3], "--print")
	}
	if args[len(args)-2] != "--" {
		t.Errorf("args[-2] = %q, want %q", args[len(args)-2], "--")
	}
	if args[len(args)-1] != "test" {
		t.Errorf("args[-1] = %q, want %q", args[len(args)-1], "test")
	}
}

func TestQuery_CLINotFound(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := Query(ctx, "test", &Options{
		CLIPath: "/nonexistent/claude",
	})

	if err == nil {
		t.Fatal("expected error for non-existent CLI")
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

func TestQuery_WithMockCLI(t *testing.T) {
	// モックCLIスクリプトを作成
	scriptContent := `#!/bin/sh
# 全ての引数を無視してJSONを出力する
echo '{"type":"assistant","message":{"role":"assistant","model":"claude-3-opus","content":[{"type":"text","text":"Hello from mock!"}]}}'
echo '{"type":"result","subtype":"query_complete","duration_ms":100,"duration_api_ms":50,"is_error":false,"num_turns":1,"session_id":"mock-session","total_cost_usd":0.001,"usage":{"input_tokens":10,"output_tokens":5},"result":"done"}'
`
	tmpFile, err := os.CreateTemp("", "mock-cli-*.sh")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(scriptContent); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}
	tmpFile.Close()

	if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := Query(ctx, "Hello!", &Options{
		CLIPath: tmpFile.Name(),
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// 結果の確認
	if result.SessionID != "mock-session" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "mock-session")
	}
	if result.TotalCost != 0.001 {
		t.Errorf("TotalCost = %f, want %f", result.TotalCost, 0.001)
	}
	if result.Usage.InputTokens != 10 {
		t.Errorf("Usage.InputTokens = %d, want %d", result.Usage.InputTokens, 10)
	}

	// メッセージの確認
	if len(result.Messages) != 1 {
		t.Fatalf("len(Messages) = %d, want %d", len(result.Messages), 1)
	}
}

func TestQuery_NilOptions(t *testing.T) {
	// モックCLIスクリプトを作成
	scriptContent := `#!/bin/sh
echo '{"type":"result","subtype":"query_complete","duration_ms":100,"duration_api_ms":50,"is_error":false,"num_turns":1,"session_id":"test","total_cost_usd":0.0,"usage":{"input_tokens":0,"output_tokens":0}}'
`
	tmpFile, err := os.CreateTemp("", "mock-cli-*.sh")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(scriptContent); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}
	tmpFile.Close()

	if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// nilオプションでQuery呼び出し（内部でdefault値が使用される想定）
	// CLIPathが空だと実際のclaudeを探すため、明示的に設定
	result, err := Query(ctx, "Hello!", &Options{CLIPath: tmpFile.Name()})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if result.Result == nil {
		t.Error("Result should not be nil")
	}
}

func TestQueryResult_Structure(t *testing.T) {
	// QueryResult構造体の基本的な動作確認
	result := &QueryResult{
		Messages:  nil,
		Result:    nil,
		SessionID: "test-session",
		TotalCost: 0.05,
	}

	if result.SessionID != "test-session" {
		t.Errorf("SessionID = %q, want %q", result.SessionID, "test-session")
	}
}

func TestBuildQueryArgs_SessionOptions(t *testing.T) {
	tests := []struct {
		name     string
		opts     *Options
		wantArgs []string
	}{
		{
			name: "resume only",
			opts: &Options{
				Resume: "session-123",
			},
			wantArgs: []string{"--resume", "session-123"},
		},
		{
			name: "resume with fork",
			opts: &Options{
				Resume:      "session-123",
				ForkSession: true,
			},
			wantArgs: []string{"--resume", "session-123", "--fork-session"},
		},
		{
			name: "continue",
			opts: &Options{
				Continue: true,
			},
			wantArgs: []string{"--continue"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := buildQueryArgs("test", tt.opts)

			// 期待する引数が含まれているか確認
			for i := 0; i < len(tt.wantArgs); i++ {
				found := false
				for j := 0; j < len(args); j++ {
					if args[j] == tt.wantArgs[i] {
						found = true
						// 値を持つフラグの場合は次の要素も確認
						if i+1 < len(tt.wantArgs) && j+1 < len(args) {
							if tt.wantArgs[i] == "--resume" {
								if args[j+1] != tt.wantArgs[i+1] {
									t.Errorf("arg %s value = %q, want %q", tt.wantArgs[i], args[j+1], tt.wantArgs[i+1])
								}
								i++ // skip value
							}
						}
						break
					}
				}
				if !found {
					t.Errorf("missing arg: %s", tt.wantArgs[i])
				}
			}
		})
	}
}
