package mcp

import (
	"context"
	"fmt"
	"testing"
)

func TestNewSDKMCPServer(t *testing.T) {
	s := NewSDKMCPServer("calc", "1.0.0")
	if s == nil {
		t.Fatal("NewSDKMCPServer returned nil")
	}
	if s.Name != "calc" {
		t.Errorf("Name = %q, want %q", s.Name, "calc")
	}
	if s.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", s.Version, "1.0.0")
	}
}

func TestSDKMCPServer_AddTool(t *testing.T) {
	s := NewSDKMCPServer("calc", "1.0.0")

	s.AddTool(Tool{
		Name:        "add",
		Description: "Add two numbers",
		Handler: func(args map[string]any) (*ToolResult, error) {
			return &ToolResult{}, nil
		},
	})

	tool, ok := s.GetTool("add")
	if !ok {
		t.Fatal("tool not found")
	}
	if tool.Name != "add" {
		t.Errorf("Name = %q, want %q", tool.Name, "add")
	}
}

func TestSDKMCPServer_RemoveTool(t *testing.T) {
	s := NewSDKMCPServer("calc", "1.0.0")

	s.AddTool(Tool{Name: "add"})
	s.RemoveTool("add")

	_, ok := s.GetTool("add")
	if ok {
		t.Error("tool should be removed")
	}
}

func TestSDKMCPServer_ListTools(t *testing.T) {
	s := NewSDKMCPServer("calc", "1.0.0")

	s.AddTool(Tool{Name: "add", Description: "Add numbers"})
	s.AddTool(Tool{Name: "sub", Description: "Subtract numbers"})

	tools := s.ListTools()
	if len(tools) != 2 {
		t.Errorf("len(tools) = %d, want 2", len(tools))
	}
}

func TestSDKMCPServer_HandleCall(t *testing.T) {
	s := NewSDKMCPServer("calc", "1.0.0")

	s.AddTool(Tool{
		Name: "add",
		Handler: func(args map[string]any) (*ToolResult, error) {
			a := args["a"].(float64)
			b := args["b"].(float64)
			return &ToolResult{
				Content: []ContentBlock{
					{Type: "text", Text: fmt.Sprintf("%.0f", a+b)},
				},
			}, nil
		},
	})

	result, err := s.HandleCall("add", map[string]any{"a": 1.0, "b": 2.0})
	if err != nil {
		t.Fatalf("HandleCall failed: %v", err)
	}

	if len(result.Content) != 1 {
		t.Fatalf("len(Content) = %d, want 1", len(result.Content))
	}
	if result.Content[0].Text != "3" {
		t.Errorf("Text = %q, want %q", result.Content[0].Text, "3")
	}
}

func TestSDKMCPServer_HandleCall_NotFound(t *testing.T) {
	s := NewSDKMCPServer("calc", "1.0.0")

	_, err := s.HandleCall("unknown", nil)
	if err == nil {
		t.Error("expected error for unknown tool")
	}
}

func TestSDKMCPServer_HandleMessage_ToolsList(t *testing.T) {
	s := NewSDKMCPServer("calc", "1.0.0")
	s.AddTool(Tool{Name: "add", Description: "Add"})

	msg := &Message{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}

	resp, err := s.HandleMessage(msg)
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}
	if resp.ID != 1 {
		t.Errorf("ID = %v, want 1", resp.ID)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatal("result should be map")
	}
	tools, ok := result["tools"].([]ToolInfo)
	if !ok {
		t.Fatal("tools should be []ToolInfo")
	}
	if len(tools) != 1 {
		t.Errorf("len(tools) = %d, want 1", len(tools))
	}
}

func TestSDKMCPServer_HandleMessage_ToolsCall(t *testing.T) {
	s := NewSDKMCPServer("calc", "1.0.0")
	s.AddTool(Tool{
		Name: "add",
		Handler: func(args map[string]any) (*ToolResult, error) {
			return &ToolResult{
				Content: []ContentBlock{{Type: "text", Text: "result"}},
			}, nil
		},
	})

	msg := &Message{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: map[string]any{
			"name":      "add",
			"arguments": map[string]any{},
		},
	}

	resp, err := s.HandleMessage(msg)
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}
}

func TestSDKMCPServer_HandleMessage_MethodNotFound(t *testing.T) {
	s := NewSDKMCPServer("calc", "1.0.0")

	msg := &Message{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "unknown/method",
	}

	resp, err := s.HandleMessage(msg)
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}

	if resp.Error == nil {
		t.Error("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("Code = %d, want -32601", resp.Error.Code)
	}
}

func TestServerConfig_ToMap(t *testing.T) {
	// Stdio
	stdio := &ServerConfig{
		Type:    TransportStdio,
		Command: "python",
		Args:    []string{"-m", "mcp_server"},
		Env:     map[string]string{"DEBUG": "true"},
	}

	m := stdio.ToMap()
	if m["type"] != "stdio" {
		t.Errorf("type = %v, want stdio", m["type"])
	}
	if m["command"] != "python" {
		t.Errorf("command = %v, want python", m["command"])
	}

	// SSE
	sse := &ServerConfig{
		Type:    TransportSSE,
		URL:     "http://localhost:8080",
		Headers: map[string]string{"Authorization": "Bearer token"},
	}

	m = sse.ToMap()
	if m["type"] != "sse" {
		t.Errorf("type = %v, want sse", m["type"])
	}
	if m["url"] != "http://localhost:8080" {
		t.Errorf("url = %v, want http://localhost:8080", m["url"])
	}
}

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.servers == nil {
		t.Error("servers should be initialized")
	}
	if m.sdkServers == nil {
		t.Error("sdkServers should be initialized")
	}
}

func TestManager_AddExternalServer(t *testing.T) {
	m := NewManager()

	config := &ServerConfig{
		Type:    TransportStdio,
		Command: "python",
	}
	m.AddExternalServer("test", config)

	retrieved, ok := m.GetExternalServer("test")
	if !ok {
		t.Fatal("server not found")
	}
	if retrieved.Command != "python" {
		t.Errorf("Command = %q, want %q", retrieved.Command, "python")
	}
}

func TestManager_AddSDKServer(t *testing.T) {
	m := NewManager()

	server := NewSDKMCPServer("calc", "1.0.0")
	m.AddSDKServer("calc", server)

	retrieved, ok := m.GetSDKServer("calc")
	if !ok {
		t.Fatal("server not found")
	}
	if retrieved.Name != "calc" {
		t.Errorf("Name = %q, want %q", retrieved.Name, "calc")
	}
}

func TestManager_BuildCLIConfig(t *testing.T) {
	m := NewManager()

	m.AddExternalServer("external", &ServerConfig{
		Type:    TransportStdio,
		Command: "python",
	})

	m.AddSDKServer("sdk", NewSDKMCPServer("sdk", "1.0.0"))

	config := m.BuildCLIConfig()

	// 外部サーバー
	ext, ok := config["external"].(map[string]any)
	if !ok {
		t.Fatal("external config should be map")
	}
	if ext["type"] != "stdio" {
		t.Errorf("external type = %v, want stdio", ext["type"])
	}

	// SDKサーバー
	sdk, ok := config["sdk"].(map[string]any)
	if !ok {
		t.Fatal("sdk config should be map")
	}
	if sdk["type"] != "sdk" {
		t.Errorf("sdk type = %v, want sdk", sdk["type"])
	}
}

func TestManager_HandleMCPMessage(t *testing.T) {
	m := NewManager()

	server := NewSDKMCPServer("calc", "1.0.0")
	server.AddTool(Tool{
		Name: "add",
		Handler: func(args map[string]any) (*ToolResult, error) {
			return &ToolResult{
				Content: []ContentBlock{{Type: "text", Text: "result"}},
			}, nil
		},
	})
	m.AddSDKServer("calc", server)

	msg := &Message{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}

	resp, err := m.HandleMCPMessage("calc", msg)
	if err != nil {
		t.Fatalf("HandleMCPMessage failed: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}
}

func TestManager_HandleMCPMessage_NotFound(t *testing.T) {
	m := NewManager()

	msg := &Message{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}

	resp, err := m.HandleMCPMessage("unknown", msg)
	if err != nil {
		t.Fatalf("HandleMCPMessage failed: %v", err)
	}

	if resp.Error == nil {
		t.Error("expected error for unknown server")
	}
}

func TestTransportTypeConstants(t *testing.T) {
	types := []TransportType{
		TransportStdio,
		TransportSSE,
		TransportHTTP,
	}

	expected := []string{"stdio", "sse", "http"}
	for i, tp := range types {
		if string(tp) != expected[i] {
			t.Errorf("type = %q, want %q", tp, expected[i])
		}
	}
}

func TestManager_GetClient_NotConnected(t *testing.T) {
	m := NewManager()

	_, ok := m.GetClient("unknown")
	if ok {
		t.Error("expected false for unknown client")
	}
}

func TestManager_DisconnectServer_NotConnected(t *testing.T) {
	m := NewManager()

	err := m.DisconnectServer("unknown")
	if err == nil {
		t.Error("expected error for unknown server")
	}
}

func TestManager_CallTool_SDKServer(t *testing.T) {
	m := NewManager()

	server := NewSDKMCPServer("calc", "1.0.0")
	server.AddTool(Tool{
		Name: "add",
		Handler: func(args map[string]any) (*ToolResult, error) {
			a := args["a"].(float64)
			b := args["b"].(float64)
			return &ToolResult{
				Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("%.0f", a+b)}},
			}, nil
		},
	})
	m.AddSDKServer("calc", server)

	result, err := m.CallTool(context.Background(), "calc", "add", map[string]any{"a": 1.0, "b": 2.0})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	if len(result.Content) != 1 {
		t.Fatalf("len(Content) = %d, want 1", len(result.Content))
	}
	if result.Content[0].Text != "3" {
		t.Errorf("Content[0].Text = %q, want %q", result.Content[0].Text, "3")
	}
}

func TestManager_CallTool_NotFound(t *testing.T) {
	m := NewManager()

	_, err := m.CallTool(context.Background(), "unknown", "tool", nil)
	if err == nil {
		t.Error("expected error for unknown server")
	}
}
