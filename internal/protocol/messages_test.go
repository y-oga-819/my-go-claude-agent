package protocol

import (
	"encoding/json"
	"testing"
)

func TestUserMessage_JSON(t *testing.T) {
	sessionID := "session-123"
	msg := UserMessage{
		Type: "user",
		Message: UserContent{
			Role:    "user",
			Content: "Hello, Claude!",
		},
		SessionID: sessionID,
	}

	// Marshal
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// Unmarshal
	var decoded UserMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Type != "user" {
		t.Errorf("Type = %q, want %q", decoded.Type, "user")
	}
	if decoded.Message.Role != "user" {
		t.Errorf("Message.Role = %q, want %q", decoded.Message.Role, "user")
	}
	if decoded.SessionID != sessionID {
		t.Errorf("SessionID = %q, want %q", decoded.SessionID, sessionID)
	}
}

func TestAssistantMessage_JSON(t *testing.T) {
	msg := AssistantMessage{
		Type: "assistant",
		Message: AssistantBody{
			Role:  "assistant",
			Model: "claude-3-opus",
			Content: []ContentBlock{
				{Type: "text", Text: "Hello!"},
			},
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded AssistantMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Type != "assistant" {
		t.Errorf("Type = %q, want %q", decoded.Type, "assistant")
	}
	if decoded.Message.Model != "claude-3-opus" {
		t.Errorf("Message.Model = %q, want %q", decoded.Message.Model, "claude-3-opus")
	}
	if len(decoded.Message.Content) != 1 {
		t.Fatalf("len(Content) = %d, want %d", len(decoded.Message.Content), 1)
	}
	if decoded.Message.Content[0].Text != "Hello!" {
		t.Errorf("Content[0].Text = %q, want %q", decoded.Message.Content[0].Text, "Hello!")
	}
}

func TestContentBlock_ToolUse_JSON(t *testing.T) {
	block := ContentBlock{
		Type: "tool_use",
		ID:   "toolu_123",
		Name: "Read",
		Input: map[string]any{
			"file_path": "/tmp/test.txt",
		},
	}

	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded ContentBlock
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Type != "tool_use" {
		t.Errorf("Type = %q, want %q", decoded.Type, "tool_use")
	}
	if decoded.ID != "toolu_123" {
		t.Errorf("ID = %q, want %q", decoded.ID, "toolu_123")
	}
	if decoded.Name != "Read" {
		t.Errorf("Name = %q, want %q", decoded.Name, "Read")
	}
	if decoded.Input["file_path"] != "/tmp/test.txt" {
		t.Errorf("Input[file_path] = %q, want %q", decoded.Input["file_path"], "/tmp/test.txt")
	}
}

func TestResultMessage_JSON(t *testing.T) {
	msg := ResultMessage{
		Type:          "result",
		Subtype:       "query_complete",
		DurationMs:    1234,
		DurationAPIMs: 1000,
		IsError:       false,
		NumTurns:      3,
		SessionID:     "session-123",
		TotalCostUSD:  0.05,
		Usage: Usage{
			InputTokens:  100,
			OutputTokens: 50,
		},
		Result: "Task completed successfully",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded ResultMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Type != "result" {
		t.Errorf("Type = %q, want %q", decoded.Type, "result")
	}
	if decoded.NumTurns != 3 {
		t.Errorf("NumTurns = %d, want %d", decoded.NumTurns, 3)
	}
	if decoded.Usage.InputTokens != 100 {
		t.Errorf("Usage.InputTokens = %d, want %d", decoded.Usage.InputTokens, 100)
	}
}

func TestControlRequest_JSON(t *testing.T) {
	msg := ControlRequest{
		Type:      "control_request",
		RequestID: "req-123",
		Request: map[string]any{
			"type":      "can_use_tool",
			"tool_name": "Bash",
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded ControlRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Type != "control_request" {
		t.Errorf("Type = %q, want %q", decoded.Type, "control_request")
	}
	if decoded.RequestID != "req-123" {
		t.Errorf("RequestID = %q, want %q", decoded.RequestID, "req-123")
	}
}

func TestControlResponse_JSON(t *testing.T) {
	msg := ControlResponse{
		Type: "control_response",
		Response: ControlResponseBody{
			Subtype:   "success",
			RequestID: "req-123",
			Response: map[string]any{
				"allow": true,
			},
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded ControlResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Type != "control_response" {
		t.Errorf("Type = %q, want %q", decoded.Type, "control_response")
	}
	if decoded.Response.Subtype != "success" {
		t.Errorf("Response.Subtype = %q, want %q", decoded.Response.Subtype, "success")
	}
}

func TestMessageInterface(t *testing.T) {
	// 各メッセージ型がMessageインターフェースを実装していることを確認
	var _ Message = &UserMessage{Type: "user"}
	var _ Message = &AssistantMessage{Type: "assistant"}
	var _ Message = &SystemMessage{Type: "system"}
	var _ Message = &ResultMessage{Type: "result"}
	var _ Message = &ControlRequest{Type: "control_request"}
	var _ Message = &ControlResponse{Type: "control_response"}

	// MessageType()の戻り値を確認
	messages := []Message{
		&UserMessage{Type: "user"},
		&AssistantMessage{Type: "assistant"},
		&SystemMessage{Type: "system"},
		&ResultMessage{Type: "result"},
		&ControlRequest{Type: "control_request"},
		&ControlResponse{Type: "control_response"},
	}

	expectedTypes := []string{"user", "assistant", "system", "result", "control_request", "control_response"}
	for i, msg := range messages {
		if msg.MessageType() != expectedTypes[i] {
			t.Errorf("MessageType() = %q, want %q", msg.MessageType(), expectedTypes[i])
		}
	}
}

func TestAssistantMessage_WithError(t *testing.T) {
	errMsg := "Something went wrong"
	msg := AssistantMessage{
		Type: "assistant",
		Message: AssistantBody{
			Role:    "assistant",
			Model:   "claude-3-opus",
			Content: []ContentBlock{},
			Error:   &errMsg,
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded AssistantMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Message.Error == nil {
		t.Fatal("Message.Error should not be nil")
	}
	if *decoded.Message.Error != errMsg {
		t.Errorf("Message.Error = %q, want %q", *decoded.Message.Error, errMsg)
	}
}

func TestUserMessage_WithParentToolUseID(t *testing.T) {
	parentID := "toolu_parent_123"
	msg := UserMessage{
		Type: "user",
		Message: UserContent{
			Role:    "user",
			Content: "Tool result",
		},
		ParentToolUseID: &parentID,
		SessionID:       "session-123",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded UserMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.ParentToolUseID == nil {
		t.Fatal("ParentToolUseID should not be nil")
	}
	if *decoded.ParentToolUseID != parentID {
		t.Errorf("ParentToolUseID = %q, want %q", *decoded.ParentToolUseID, parentID)
	}
}

func TestUserMessage_WithUUID(t *testing.T) {
	msg := UserMessage{
		Type: "user",
		Message: UserContent{
			Role:    "user",
			Content: "Hello",
		},
		SessionID: "session-123",
		UUID:      "uuid-456-789",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded UserMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.UUID != "uuid-456-789" {
		t.Errorf("UUID = %q, want %q", decoded.UUID, "uuid-456-789")
	}
}

func TestUserMessage_UUIDOmitEmpty(t *testing.T) {
	msg := UserMessage{
		Type: "user",
		Message: UserContent{
			Role:    "user",
			Content: "Hello",
		},
		SessionID: "session-123",
		// UUID is empty
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// UUIDが空の場合はJSONに含まれないことを確認
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if _, exists := raw["uuid"]; exists {
		t.Error("uuid should not be present when empty")
	}
}
