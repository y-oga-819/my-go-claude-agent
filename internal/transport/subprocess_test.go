package transport

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestNewSubprocessTransport(t *testing.T) {
	// デフォルト設定
	tr := NewSubprocessTransport(Config{})
	if tr.config.CLIPath != DefaultCLIPath {
		t.Errorf("CLIPath = %q, want %q", tr.config.CLIPath, DefaultCLIPath)
	}
	if tr.config.MaxBufferSize != DefaultMaxBufferSize {
		t.Errorf("MaxBufferSize = %d, want %d", tr.config.MaxBufferSize, DefaultMaxBufferSize)
	}

	// カスタム設定
	tr = NewSubprocessTransport(Config{
		CLIPath:       "/custom/claude",
		MaxBufferSize: 1024,
	})
	if tr.config.CLIPath != "/custom/claude" {
		t.Errorf("CLIPath = %q, want %q", tr.config.CLIPath, "/custom/claude")
	}
	if tr.config.MaxBufferSize != 1024 {
		t.Errorf("MaxBufferSize = %d, want %d", tr.config.MaxBufferSize, 1024)
	}
}

func TestSubprocessTransport_buildArgs(t *testing.T) {
	tests := []struct {
		name          string
		config        Config
		expectContain []string
	}{
		{
			name:   "default args",
			config: Config{},
			expectContain: []string{
				"--output-format", "stream-json",
				"--verbose",
			},
		},
		{
			name: "streaming mode",
			config: Config{
				StreamingMode: true,
			},
			expectContain: []string{
				"--output-format", "stream-json",
				"--verbose",
				"--input-format", "stream-json",
			},
		},
		{
			name: "with additional args",
			config: Config{
				Args: []string{"--print", "--", "hello"},
			},
			expectContain: []string{
				"--output-format", "stream-json",
				"--print", "--", "hello",
			},
		},
		{
			name: "with permission prompt tool",
			config: Config{
				PermissionPromptToolName: "stdio",
			},
			expectContain: []string{
				"--output-format", "stream-json",
				"--permission-prompt-tool", "stdio",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewSubprocessTransport(tt.config)
			args := tr.buildArgs()

			for _, expected := range tt.expectContain {
				found := false
				for _, arg := range args {
					if arg == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("args should contain %q, got %v", expected, args)
				}
			}
		})
	}
}

func TestSubprocessTransport_buildEnv(t *testing.T) {
	tr := NewSubprocessTransport(Config{
		Env: map[string]string{
			"CUSTOM_VAR": "custom_value",
		},
	})

	env := tr.buildEnv()

	// SDK固有の環境変数が含まれているか確認
	// 注: 環境変数は後から追加されるので、最後に出現する値を確認
	findLastEnv := func(key string) (string, bool) {
		var lastValue string
		found := false
		prefix := key + "="
		for _, e := range env {
			if len(e) > len(prefix) && e[:len(prefix)] == prefix {
				lastValue = e[len(prefix):]
				found = true
			}
		}
		return lastValue, found
	}

	// SDK固有の環境変数が追加されていることを確認
	if val, found := findLastEnv("CLAUDE_CODE_ENTRYPOINT"); !found {
		t.Error("env should contain CLAUDE_CODE_ENTRYPOINT")
	} else if val != "sdk-go" {
		t.Errorf("CLAUDE_CODE_ENTRYPOINT = %q, want %q", val, "sdk-go")
	}

	if val, found := findLastEnv("CLAUDE_AGENT_SDK_VERSION"); !found {
		t.Error("env should contain CLAUDE_AGENT_SDK_VERSION")
	} else if val != "0.1.0" {
		t.Errorf("CLAUDE_AGENT_SDK_VERSION = %q, want %q", val, "0.1.0")
	}

	if val, found := findLastEnv("CUSTOM_VAR"); !found {
		t.Error("env should contain CUSTOM_VAR")
	} else if val != "custom_value" {
		t.Errorf("CUSTOM_VAR = %q, want %q", val, "custom_value")
	}
}

func TestSubprocessTransport_InterfaceCompliance(t *testing.T) {
	// SubprocessTransportがTransportインターフェースを実装していることを確認
	var _ Transport = &SubprocessTransport{}
}

func TestSubprocessTransport_ConnectWithMockCLI(t *testing.T) {
	// モックCLIスクリプトを作成
	// 全ての引数を無視してJSONを出力する
	scriptContent := `#!/bin/sh
echo '{"type":"result","subtype":"query_complete"}'
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

	tr := NewSubprocessTransport(Config{
		CLIPath: tmpFile.Name(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := tr.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer tr.Close()

	if !tr.IsConnected() {
		t.Error("IsConnected() should return true after Connect")
	}

	// メッセージを受信
	select {
	case msg := <-tr.Messages():
		if msg.Type != "result" {
			t.Errorf("Type = %q, want %q", msg.Type, "result")
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for message")
	}
}

func TestSubprocessTransport_WriteBeforeConnect(t *testing.T) {
	tr := NewSubprocessTransport(Config{})

	err := tr.Write([]byte(`{"type":"test"}`))
	if err == nil {
		t.Error("Write should fail before Connect")
	}
}

func TestSubprocessTransport_CloseIdempotent(t *testing.T) {
	tr := NewSubprocessTransport(Config{
		CLIPath: "echo",
		Args:    []string{"test"},
	})

	ctx := context.Background()
	if err := tr.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// 複数回Closeしてもエラーにならない
	if err := tr.Close(); err != nil {
		t.Errorf("first Close failed: %v", err)
	}
	if err := tr.Close(); err != nil {
		t.Errorf("second Close failed: %v", err)
	}
}

func TestRawMessage_JSON(t *testing.T) {
	// RawMessageの基本的な動作確認
	raw := []byte(`{"type":"assistant","message":{"role":"assistant"}}`)
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	msg := RawMessage{
		Type: data["type"].(string),
		Data: data,
		Raw:  raw,
	}

	if msg.Type != "assistant" {
		t.Errorf("Type = %q, want %q", msg.Type, "assistant")
	}
	if string(msg.Raw) != string(raw) {
		t.Errorf("Raw mismatch")
	}
}

func TestSubprocessTransport_Channels(t *testing.T) {
	tr := NewSubprocessTransport(Config{})

	// チャネルが初期化されていることを確認
	if tr.Messages() == nil {
		t.Error("Messages() should not return nil")
	}
	if tr.Errors() == nil {
		t.Error("Errors() should not return nil")
	}
}

func TestSubprocessTransport_EndInput(t *testing.T) {
	tr := NewSubprocessTransport(Config{
		CLIPath: "cat",
	})

	ctx := context.Background()
	if err := tr.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer tr.Close()

	// EndInputでstdinを閉じる
	if err := tr.EndInput(); err != nil {
		t.Errorf("EndInput failed: %v", err)
	}

	// 2回目のEndInputはエラーにならない（stdinはnilになっている可能性）
	_ = tr.EndInput()
}

func TestConfig_Defaults(t *testing.T) {
	config := Config{}

	if config.CLIPath != "" {
		t.Errorf("CLIPath should be empty by default, got %q", config.CLIPath)
	}
	if config.MaxBufferSize != 0 {
		t.Errorf("MaxBufferSize should be 0 by default, got %d", config.MaxBufferSize)
	}
	if config.StreamingMode {
		t.Error("StreamingMode should be false by default")
	}
}
