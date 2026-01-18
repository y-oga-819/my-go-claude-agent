package protocol

import (
	"testing"
)

func TestParseMessage_AssistantMessage(t *testing.T) {
	data := map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"role":  "assistant",
			"model": "claude-3-opus",
			"content": []any{
				map[string]any{
					"type": "text",
					"text": "Hello!",
				},
			},
		},
	}

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	am, ok := msg.(*AssistantMessage)
	if !ok {
		t.Fatalf("expected *AssistantMessage, got %T", msg)
	}

	if am.Type != "assistant" {
		t.Errorf("Type = %q, want %q", am.Type, "assistant")
	}
	if am.Message.Model != "claude-3-opus" {
		t.Errorf("Message.Model = %q, want %q", am.Message.Model, "claude-3-opus")
	}
	if len(am.Message.Content) != 1 {
		t.Fatalf("len(Content) = %d, want %d", len(am.Message.Content), 1)
	}
	if am.Message.Content[0].Text != "Hello!" {
		t.Errorf("Content[0].Text = %q, want %q", am.Message.Content[0].Text, "Hello!")
	}
}

func TestParseMessage_ResultMessage(t *testing.T) {
	data := map[string]any{
		"type":           "result",
		"subtype":        "query_complete",
		"duration_ms":    float64(1234),
		"duration_api_ms": float64(1000),
		"is_error":       false,
		"num_turns":      float64(3),
		"session_id":     "session-123",
		"total_cost_usd": 0.05,
		"usage": map[string]any{
			"input_tokens":  float64(100),
			"output_tokens": float64(50),
		},
		"result": "Task completed",
	}

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	rm, ok := msg.(*ResultMessage)
	if !ok {
		t.Fatalf("expected *ResultMessage, got %T", msg)
	}

	if rm.Type != "result" {
		t.Errorf("Type = %q, want %q", rm.Type, "result")
	}
	if rm.SessionID != "session-123" {
		t.Errorf("SessionID = %q, want %q", rm.SessionID, "session-123")
	}
	if rm.TotalCostUSD != 0.05 {
		t.Errorf("TotalCostUSD = %f, want %f", rm.TotalCostUSD, 0.05)
	}
	if rm.Usage.InputTokens != 100 {
		t.Errorf("Usage.InputTokens = %d, want %d", rm.Usage.InputTokens, 100)
	}
}

func TestParseMessage_SystemMessage(t *testing.T) {
	data := map[string]any{
		"type":    "system",
		"subtype": "init",
		"data": map[string]any{
			"version": "2.0.0",
		},
	}

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	sm, ok := msg.(*SystemMessage)
	if !ok {
		t.Fatalf("expected *SystemMessage, got %T", msg)
	}

	if sm.Type != "system" {
		t.Errorf("Type = %q, want %q", sm.Type, "system")
	}
	if sm.Subtype != "init" {
		t.Errorf("Subtype = %q, want %q", sm.Subtype, "init")
	}
}

func TestParseMessage_UserMessage(t *testing.T) {
	data := map[string]any{
		"type": "user",
		"message": map[string]any{
			"role":    "user",
			"content": "Hello, Claude!",
		},
		"session_id": "session-123",
	}

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	um, ok := msg.(*UserMessage)
	if !ok {
		t.Fatalf("expected *UserMessage, got %T", msg)
	}

	if um.Type != "user" {
		t.Errorf("Type = %q, want %q", um.Type, "user")
	}
	if um.SessionID != "session-123" {
		t.Errorf("SessionID = %q, want %q", um.SessionID, "session-123")
	}
}

func TestParseMessage_ControlRequest(t *testing.T) {
	data := map[string]any{
		"type":       "control_request",
		"request_id": "req-123",
		"request": map[string]any{
			"type":      "can_use_tool",
			"tool_name": "Bash",
		},
	}

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	cr, ok := msg.(*ControlRequest)
	if !ok {
		t.Fatalf("expected *ControlRequest, got %T", msg)
	}

	if cr.Type != "control_request" {
		t.Errorf("Type = %q, want %q", cr.Type, "control_request")
	}
	if cr.RequestID != "req-123" {
		t.Errorf("RequestID = %q, want %q", cr.RequestID, "req-123")
	}
}

func TestParseMessage_ControlResponse(t *testing.T) {
	data := map[string]any{
		"type": "control_response",
		"response": map[string]any{
			"subtype":    "success",
			"request_id": "req-123",
			"response": map[string]any{
				"allow": true,
			},
		},
	}

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	cr, ok := msg.(*ControlResponse)
	if !ok {
		t.Fatalf("expected *ControlResponse, got %T", msg)
	}

	if cr.Type != "control_response" {
		t.Errorf("Type = %q, want %q", cr.Type, "control_response")
	}
	if cr.Response.Subtype != "success" {
		t.Errorf("Response.Subtype = %q, want %q", cr.Response.Subtype, "success")
	}
}

func TestParseMessage_UnknownType(t *testing.T) {
	data := map[string]any{
		"type":  "unknown_type",
		"field": "value",
	}

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage failed: %v", err)
	}

	gm, ok := msg.(*GenericMessage)
	if !ok {
		t.Fatalf("expected *GenericMessage, got %T", msg)
	}

	if gm.Type != "unknown_type" {
		t.Errorf("Type = %q, want %q", gm.Type, "unknown_type")
	}
	if gm.MessageType() != "unknown_type" {
		t.Errorf("MessageType() = %q, want %q", gm.MessageType(), "unknown_type")
	}
}

func TestParseMessage_MissingType(t *testing.T) {
	data := map[string]any{
		"field": "value",
	}

	_, err := ParseMessage(data)
	if err == nil {
		t.Error("expected error for missing type")
	}
}

func TestParseMessage_MessageTypeInterface(t *testing.T) {
	// 各メッセージ型がMessageインターフェースを満たすことを確認
	testCases := []struct {
		name string
		data map[string]any
	}{
		{
			name: "user",
			data: map[string]any{
				"type": "user",
				"message": map[string]any{
					"role":    "user",
					"content": "test",
				},
				"session_id": "test",
			},
		},
		{
			name: "assistant",
			data: map[string]any{
				"type": "assistant",
				"message": map[string]any{
					"role":    "assistant",
					"model":   "claude",
					"content": []any{},
				},
			},
		},
		{
			name: "system",
			data: map[string]any{
				"type":    "system",
				"subtype": "test",
				"data":    map[string]any{},
			},
		},
		{
			name: "result",
			data: map[string]any{
				"type":           "result",
				"subtype":        "query_complete",
				"session_id":     "test",
				"total_cost_usd": 0.0,
				"usage":          map[string]any{},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			msg, err := ParseMessage(tc.data)
			if err != nil {
				t.Fatalf("ParseMessage failed: %v", err)
			}

			if msg.MessageType() != tc.name {
				t.Errorf("MessageType() = %q, want %q", msg.MessageType(), tc.name)
			}
		})
	}
}
