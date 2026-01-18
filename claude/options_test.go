package claude

import (
	"context"
	"testing"
)

func TestOptions_Defaults(t *testing.T) {
	opts := Options{}

	// デフォルト値（ゼロ値）の確認
	if opts.CLIPath != "" {
		t.Errorf("CLIPath should be empty by default, got %q", opts.CLIPath)
	}
	if opts.MaxTurns != 0 {
		t.Errorf("MaxTurns should be 0 by default, got %d", opts.MaxTurns)
	}
	if opts.PermissionMode != "" {
		t.Errorf("PermissionMode should be empty by default, got %q", opts.PermissionMode)
	}
}

func TestOptions_WithValues(t *testing.T) {
	opts := Options{
		CLIPath:        "/usr/local/bin/claude",
		CWD:            "/home/user/project",
		SystemPrompt:   "You are a helpful assistant",
		Model:          "claude-3-opus",
		MaxTurns:       10,
		MaxBudgetUSD:   5.0,
		PermissionMode: PermissionModeAcceptEdits,
		AllowedTools:   []string{"Read", "Write"},
		MCPServers: map[string]MCPServerConfig{
			"filesystem": {
				Type:    "stdio",
				Command: "mcp-server-filesystem",
				Args:    []string{"/tmp"},
			},
		},
	}

	if opts.CLIPath != "/usr/local/bin/claude" {
		t.Errorf("CLIPath = %q, want %q", opts.CLIPath, "/usr/local/bin/claude")
	}
	if opts.MaxTurns != 10 {
		t.Errorf("MaxTurns = %d, want %d", opts.MaxTurns, 10)
	}
	if len(opts.AllowedTools) != 2 {
		t.Errorf("len(AllowedTools) = %d, want %d", len(opts.AllowedTools), 2)
	}
	if len(opts.MCPServers) != 1 {
		t.Errorf("len(MCPServers) = %d, want %d", len(opts.MCPServers), 1)
	}
}

func TestPermissionMode_Constants(t *testing.T) {
	tests := []struct {
		mode     PermissionMode
		expected string
	}{
		{PermissionModeDefault, "default"},
		{PermissionModeAcceptEdits, "acceptEdits"},
		{PermissionModePlan, "plan"},
		{PermissionModeBypassPermissions, "bypassPermissions"},
	}

	for _, tt := range tests {
		if string(tt.mode) != tt.expected {
			t.Errorf("PermissionMode = %q, want %q", tt.mode, tt.expected)
		}
	}
}

func TestCanUseToolFunc(t *testing.T) {
	// CanUseToolFunc型がコンパイルできることを確認
	var canUseTool CanUseToolFunc = func(
		ctx context.Context,
		toolName string,
		input map[string]any,
		permCtx *ToolPermissionContext,
	) (*PermissionResult, error) {
		return &PermissionResult{
			Allow: toolName == "Read",
		}, nil
	}

	result, err := canUseTool(context.Background(), "Read", nil, nil)
	if err != nil {
		t.Fatalf("canUseTool returned error: %v", err)
	}
	if !result.Allow {
		t.Error("canUseTool should allow 'Read' tool")
	}

	result, err = canUseTool(context.Background(), "Write", nil, nil)
	if err != nil {
		t.Fatalf("canUseTool returned error: %v", err)
	}
	if result.Allow {
		t.Error("canUseTool should deny 'Write' tool")
	}
}

func TestMCPServerConfig(t *testing.T) {
	// stdio型
	stdioConfig := MCPServerConfig{
		Type:    "stdio",
		Command: "mcp-server",
		Args:    []string{"--verbose"},
		Env:     map[string]string{"DEBUG": "true"},
	}
	if stdioConfig.Type != "stdio" {
		t.Errorf("Type = %q, want %q", stdioConfig.Type, "stdio")
	}

	// http型
	httpConfig := MCPServerConfig{
		Type:    "http",
		URL:     "http://localhost:8080",
		Headers: map[string]string{"Authorization": "Bearer token"},
	}
	if httpConfig.Type != "http" {
		t.Errorf("Type = %q, want %q", httpConfig.Type, "http")
	}
}

func TestHookConfig(t *testing.T) {
	hooks := &HookConfig{
		PreToolUse: []HookEntry{
			{
				Matcher: ToolMatcher{ToolName: "Bash"},
				Timeout: 5000,
			},
		},
		PostToolUse: []HookEntry{
			{
				Matcher: ToolMatcher{ToolName: ".*"},
				Timeout: 1000,
			},
		},
	}

	if len(hooks.PreToolUse) != 1 {
		t.Errorf("len(PreToolUse) = %d, want %d", len(hooks.PreToolUse), 1)
	}
	if hooks.PreToolUse[0].Timeout != 5000 {
		t.Errorf("PreToolUse[0].Timeout = %d, want %d", hooks.PreToolUse[0].Timeout, 5000)
	}
}
