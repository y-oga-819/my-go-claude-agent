package protocol

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/y-oga-819/my-go-claude-agent/internal/transport"
)

// mockTransport はテスト用のモックTransport
type mockTransport struct {
	writtenData [][]byte
	msgChan     chan transport.RawMessage
	errChan     chan error
	mu          sync.Mutex
	connected   bool
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		writtenData: make([][]byte, 0),
		msgChan:     make(chan transport.RawMessage, 100),
		errChan:     make(chan error, 10),
		connected:   true,
	}
}

func (m *mockTransport) Connect(ctx context.Context) error {
	m.connected = true
	return nil
}

func (m *mockTransport) Write(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writtenData = append(m.writtenData, data)
	return nil
}

func (m *mockTransport) Messages() <-chan transport.RawMessage {
	return m.msgChan
}

func (m *mockTransport) Errors() <-chan error {
	return m.errChan
}

func (m *mockTransport) EndInput() error {
	return nil
}

func (m *mockTransport) Close() error {
	m.connected = false
	return nil
}

func (m *mockTransport) IsConnected() bool {
	return m.connected
}

func (m *mockTransport) GetProcessStatus() *transport.ProcessStatus {
	return nil
}

func (m *mockTransport) getWrittenData() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writtenData
}

func (m *mockTransport) sendMessage(msgType string, data map[string]any) {
	data["type"] = msgType
	raw, _ := json.Marshal(data)
	m.msgChan <- transport.RawMessage{
		Type: msgType,
		Data: data,
		Raw:  raw,
	}
}

func TestNewProtocolHandler(t *testing.T) {
	mt := newMockTransport()
	h := NewProtocolHandler(mt)

	if h == nil {
		t.Fatal("NewProtocolHandler returned nil")
	}
	if h.transport != mt {
		t.Error("transport not set correctly")
	}
	if h.pendingRequests == nil {
		t.Error("pendingRequests should be initialized")
	}
}

func TestProtocolHandler_SetCallbacks(t *testing.T) {
	mt := newMockTransport()
	h := NewProtocolHandler(mt)

	// SetCanUseToolCallback
	h.SetCanUseToolCallback(func(ctx context.Context, req *CanUseToolRequest) (*CanUseToolResponse, error) {
		return &CanUseToolResponse{Allow: true}, nil
	})

	// コールバックが設定されていることを確認（直接アクセスはできないのでハンドリングで確認）

	// SetMCPMessageCallback
	h.SetMCPMessageCallback(func(ctx context.Context, req *MCPMessageRequest) (*MCPMessageResponse, error) {
		return &MCPMessageResponse{}, nil
	})

	// AddHookCallback
	h.AddHookCallback("pre_tool_use", func(ctx context.Context, req *HookCallbackRequest) (*HookCallbackResponse, error) {
		return &HookCallbackResponse{Continue: true}, nil
	})
}

func TestProtocolHandler_HandleIncoming_AssistantMessage(t *testing.T) {
	mt := newMockTransport()
	h := NewProtocolHandler(mt)

	raw := transport.RawMessage{
		Type: "assistant",
		Data: map[string]any{
			"type": "assistant",
			"message": map[string]any{
				"role":    "assistant",
				"model":   "claude",
				"content": []any{},
			},
		},
	}

	ctx := context.Background()
	if err := h.HandleIncoming(ctx, raw); err != nil {
		t.Fatalf("HandleIncoming failed: %v", err)
	}

	// メッセージがチャネルに送信されていることを確認
	select {
	case msg := <-h.Messages():
		if msg.MessageType() != "assistant" {
			t.Errorf("MessageType() = %q, want %q", msg.MessageType(), "assistant")
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for message")
	}
}

func TestProtocolHandler_HandleIncoming_ControlRequest_CanUseTool(t *testing.T) {
	mt := newMockTransport()
	h := NewProtocolHandler(mt)

	// コールバックを設定
	h.SetCanUseToolCallback(func(ctx context.Context, req *CanUseToolRequest) (*CanUseToolResponse, error) {
		if req.ToolName != "Bash" {
			t.Errorf("ToolName = %q, want %q", req.ToolName, "Bash")
		}
		return &CanUseToolResponse{
			Allow:   true,
			Message: "allowed",
		}, nil
	})

	raw := transport.RawMessage{
		Type: "control_request",
		Data: map[string]any{
			"type":       "control_request",
			"request_id": "req-123",
			"request": map[string]any{
				"subtype":   "can_use_tool",
				"tool_name": "Bash",
				"input":     map[string]any{"command": "ls"},
			},
		},
	}

	ctx := context.Background()
	if err := h.HandleIncoming(ctx, raw); err != nil {
		t.Fatalf("HandleIncoming failed: %v", err)
	}

	// レスポンスが書き込まれていることを確認
	written := mt.getWrittenData()
	if len(written) != 1 {
		t.Fatalf("expected 1 written message, got %d", len(written))
	}

	var resp ControlResponse
	if err := json.Unmarshal(written[0], &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.Response.Subtype != "success" {
		t.Errorf("Response.Subtype = %q, want %q", resp.Response.Subtype, "success")
	}
	if resp.Response.RequestID != "req-123" {
		t.Errorf("Response.RequestID = %q, want %q", resp.Response.RequestID, "req-123")
	}
}

func TestProtocolHandler_HandleIncoming_ControlRequest_NoCallback(t *testing.T) {
	mt := newMockTransport()
	h := NewProtocolHandler(mt)

	// コールバックを設定しない

	raw := transport.RawMessage{
		Type: "control_request",
		Data: map[string]any{
			"type":       "control_request",
			"request_id": "req-456",
			"request": map[string]any{
				"subtype":   "can_use_tool",
				"tool_name": "Read",
			},
		},
	}

	ctx := context.Background()
	if err := h.HandleIncoming(ctx, raw); err != nil {
		t.Fatalf("HandleIncoming failed: %v", err)
	}

	// デフォルトで許可されることを確認
	written := mt.getWrittenData()
	if len(written) != 1 {
		t.Fatalf("expected 1 written message, got %d", len(written))
	}

	var resp ControlResponse
	if err := json.Unmarshal(written[0], &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.Response.Subtype != "success" {
		t.Errorf("Response.Subtype = %q, want %q", resp.Response.Subtype, "success")
	}
}

func TestProtocolHandler_HandleIncoming_ControlResponse(t *testing.T) {
	mt := newMockTransport()
	h := NewProtocolHandler(mt)

	// pending requestを作成
	respChan := make(chan *ControlResponse, 1)
	h.mu.Lock()
	h.pendingRequests["req-789"] = respChan
	h.mu.Unlock()

	raw := transport.RawMessage{
		Type: "control_response",
		Data: map[string]any{
			"type": "control_response",
			"response": map[string]any{
				"subtype":    "success",
				"request_id": "req-789",
				"response": map[string]any{
					"session_id": "session-123",
				},
			},
		},
	}

	ctx := context.Background()
	if err := h.HandleIncoming(ctx, raw); err != nil {
		t.Fatalf("HandleIncoming failed: %v", err)
	}

	// レスポンスがチャネルに送信されていることを確認
	select {
	case resp := <-respChan:
		if resp.Response.RequestID != "req-789" {
			t.Errorf("RequestID = %q, want %q", resp.Response.RequestID, "req-789")
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for response")
	}
}

func TestProtocolHandler_HandleIncoming_HookCallback(t *testing.T) {
	mt := newMockTransport()
	h := NewProtocolHandler(mt)

	// フックコールバックを設定
	hookCalled := false
	h.AddHookCallback("pre_tool_use", func(ctx context.Context, req *HookCallbackRequest) (*HookCallbackResponse, error) {
		hookCalled = true
		if req.HookType != "pre_tool_use" {
			t.Errorf("HookType = %q, want %q", req.HookType, "pre_tool_use")
		}
		return &HookCallbackResponse{Continue: true}, nil
	})

	raw := transport.RawMessage{
		Type: "control_request",
		Data: map[string]any{
			"type":       "control_request",
			"request_id": "hook-123",
			"request": map[string]any{
				"subtype":   "hook_callback",
				"hook_type": "pre_tool_use",
				"tool_name": "Bash",
			},
		},
	}

	ctx := context.Background()
	if err := h.HandleIncoming(ctx, raw); err != nil {
		t.Fatalf("HandleIncoming failed: %v", err)
	}

	if !hookCalled {
		t.Error("hook callback was not called")
	}
}

func TestCanUseToolRequest_JSON(t *testing.T) {
	req := CanUseToolRequest{
		ToolName:  "Bash",
		Input:     map[string]any{"command": "ls"},
		SessionID: "session-123",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded CanUseToolRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.ToolName != "Bash" {
		t.Errorf("ToolName = %q, want %q", decoded.ToolName, "Bash")
	}
}

func TestCanUseToolResponse_JSON(t *testing.T) {
	resp := CanUseToolResponse{
		Allow:   true,
		Message: "approved",
		UpdatedPermissions: []PermissionUpdate{
			{Tool: "Bash", Prompt: "run commands"},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded CanUseToolResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if !decoded.Allow {
		t.Error("Allow should be true")
	}
	if len(decoded.UpdatedPermissions) != 1 {
		t.Errorf("len(UpdatedPermissions) = %d, want 1", len(decoded.UpdatedPermissions))
	}
}

func TestInitializeRequest_JSON(t *testing.T) {
	req := InitializeRequest{
		Subtype:        "initialize",
		SystemPrompt:   "You are a helpful assistant",
		Model:          "claude-3-opus",
		MaxTurns:       10,
		MaxBudgetUSD:   5.0,
		PermissionMode: "acceptEdits",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded InitializeRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Subtype != "initialize" {
		t.Errorf("Subtype = %q, want %q", decoded.Subtype, "initialize")
	}
	if decoded.Model != "claude-3-opus" {
		t.Errorf("Model = %q, want %q", decoded.Model, "claude-3-opus")
	}
}

func TestInitializeRequest_SessionOptions(t *testing.T) {
	req := InitializeRequest{
		Subtype:                 "initialize",
		Resume:                  "session-123",
		ForkSession:             true,
		Continue:                false,
		EnableFileCheckpointing: true,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded InitializeRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Resume != "session-123" {
		t.Errorf("Resume = %q, want %q", decoded.Resume, "session-123")
	}
	if !decoded.ForkSession {
		t.Error("ForkSession should be true")
	}
	if decoded.Continue {
		t.Error("Continue should be false")
	}
	if !decoded.EnableFileCheckpointing {
		t.Error("EnableFileCheckpointing should be true")
	}
}

func TestInitializeRequest_SessionOptions_OmitEmpty(t *testing.T) {
	req := InitializeRequest{
		Subtype: "initialize",
		// Session options are empty
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// Empty session options should not be present
	if _, exists := raw["resume"]; exists {
		t.Error("resume should not be present when empty")
	}
	if _, exists := raw["fork_session"]; exists {
		t.Error("fork_session should not be present when false")
	}
}

func TestRewindFilesRequest_JSON(t *testing.T) {
	req := RewindFilesRequest{
		Subtype:       "rewind_files",
		UserMessageID: "msg-uuid-123",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded RewindFilesRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Subtype != "rewind_files" {
		t.Errorf("Subtype = %q, want %q", decoded.Subtype, "rewind_files")
	}
	if decoded.UserMessageID != "msg-uuid-123" {
		t.Errorf("UserMessageID = %q, want %q", decoded.UserMessageID, "msg-uuid-123")
	}
}

func TestProtocolHandler_generateRequestID(t *testing.T) {
	mt := newMockTransport()
	h := NewProtocolHandler(mt)

	id1 := h.generateRequestID()
	id2 := h.generateRequestID()
	id3 := h.generateRequestID()

	// IDがユニークであることを確認
	if id1 == id2 || id2 == id3 || id1 == id3 {
		t.Errorf("IDs should be unique: %s, %s, %s", id1, id2, id3)
	}

	// ID形式の確認
	if id1 != "sdk-1" {
		t.Errorf("id1 = %q, want %q", id1, "sdk-1")
	}
	if id2 != "sdk-2" {
		t.Errorf("id2 = %q, want %q", id2, "sdk-2")
	}
}

func TestProtocolHandler_HandleIncoming_AskUserQuestion(t *testing.T) {
	mt := newMockTransport()
	h := NewProtocolHandler(mt)

	// AskUserQuestionを処理するコールバックを設定
	h.SetCanUseToolCallback(func(ctx context.Context, req *CanUseToolRequest) (*CanUseToolResponse, error) {
		if req.ToolName != "AskUserQuestion" {
			t.Errorf("ToolName = %q, want %q", req.ToolName, "AskUserQuestion")
		}

		// inputからquestionsを取得
		questions, ok := req.Input["questions"].([]any)
		if !ok {
			t.Fatal("questions not found in input")
		}
		if len(questions) != 1 {
			t.Errorf("len(questions) = %d, want 1", len(questions))
		}

		// 回答を含めてUpdatedInputを返す
		return &CanUseToolResponse{
			Allow: true,
			UpdatedInput: map[string]any{
				"questions": req.Input["questions"],
				"answers": map[string]string{
					"How should I format the output?": "Summary",
				},
			},
		}, nil
	})

	raw := transport.RawMessage{
		Type: "control_request",
		Data: map[string]any{
			"type":       "control_request",
			"request_id": "ask-123",
			"request": map[string]any{
				"subtype":   "can_use_tool",
				"tool_name": "AskUserQuestion",
				"input": map[string]any{
					"questions": []any{
						map[string]any{
							"question": "How should I format the output?",
							"header":   "Format",
							"options": []any{
								map[string]any{"label": "Summary", "description": "Brief overview"},
								map[string]any{"label": "Detailed", "description": "Full explanation"},
							},
							"multiSelect": false,
						},
					},
				},
			},
		},
	}

	ctx := context.Background()
	if err := h.HandleIncoming(ctx, raw); err != nil {
		t.Fatalf("HandleIncoming failed: %v", err)
	}

	// レスポンスが書き込まれていることを確認
	written := mt.getWrittenData()
	if len(written) != 1 {
		t.Fatalf("expected 1 written message, got %d", len(written))
	}

	var resp ControlResponse
	if err := json.Unmarshal(written[0], &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.Response.Subtype != "success" {
		t.Errorf("Response.Subtype = %q, want %q", resp.Response.Subtype, "success")
	}

	// レスポンスにUpdatedInputが含まれていることを確認
	respData, ok := resp.Response.Response.(*CanUseToolResponse)
	if !ok {
		// JSONからデコードされた場合はmap[string]anyになる
		respMap, ok := resp.Response.Response.(map[string]any)
		if !ok {
			t.Fatalf("unexpected response type: %T", resp.Response.Response)
		}
		updatedInput, ok := respMap["updated_input"].(map[string]any)
		if !ok {
			t.Fatal("updated_input not found in response")
		}
		answers, ok := updatedInput["answers"].(map[string]any)
		if !ok {
			t.Fatal("answers not found in updated_input")
		}
		if answers["How should I format the output?"] != "Summary" {
			t.Errorf("answer = %v, want %q", answers["How should I format the output?"], "Summary")
		}
	} else {
		if respData.UpdatedInput == nil {
			t.Fatal("UpdatedInput should not be nil")
		}
		answers, ok := respData.UpdatedInput["answers"].(map[string]string)
		if !ok {
			t.Fatal("answers not found in UpdatedInput")
		}
		if answers["How should I format the output?"] != "Summary" {
			t.Errorf("answer = %q, want %q", answers["How should I format the output?"], "Summary")
		}
	}
}

func TestProtocolHandler_HandleIncoming_AskUserQuestion_MultiSelect(t *testing.T) {
	mt := newMockTransport()
	h := NewProtocolHandler(mt)

	// マルチセレクトのテスト
	h.SetCanUseToolCallback(func(ctx context.Context, req *CanUseToolRequest) (*CanUseToolResponse, error) {
		return &CanUseToolResponse{
			Allow: true,
			UpdatedInput: map[string]any{
				"questions": req.Input["questions"],
				"answers": map[string]string{
					"Which sections should I include?": "Introduction, Conclusion",
				},
			},
		}, nil
	})

	raw := transport.RawMessage{
		Type: "control_request",
		Data: map[string]any{
			"type":       "control_request",
			"request_id": "ask-multi-123",
			"request": map[string]any{
				"subtype":   "can_use_tool",
				"tool_name": "AskUserQuestion",
				"input": map[string]any{
					"questions": []any{
						map[string]any{
							"question": "Which sections should I include?",
							"header":   "Sections",
							"options": []any{
								map[string]any{"label": "Introduction", "description": "Opening context"},
								map[string]any{"label": "Conclusion", "description": "Final summary"},
							},
							"multiSelect": true,
						},
					},
				},
			},
		},
	}

	ctx := context.Background()
	if err := h.HandleIncoming(ctx, raw); err != nil {
		t.Fatalf("HandleIncoming failed: %v", err)
	}

	written := mt.getWrittenData()
	if len(written) != 1 {
		t.Fatalf("expected 1 written message, got %d", len(written))
	}

	var resp ControlResponse
	if err := json.Unmarshal(written[0], &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.Response.Subtype != "success" {
		t.Errorf("Response.Subtype = %q, want %q", resp.Response.Subtype, "success")
	}
}
