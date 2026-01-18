package protocol

import (
	"encoding/json"
	"fmt"
)

// ParseMessage は生のJSONデータをメッセージ型に変換する
func ParseMessage(data map[string]any) (Message, error) {
	msgType, ok := data["type"].(string)
	if !ok {
		return nil, fmt.Errorf("missing message type")
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	switch msgType {
	case "user":
		var msg UserMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			return nil, fmt.Errorf("parse user message: %w", err)
		}
		return &msg, nil

	case "assistant":
		var msg AssistantMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			return nil, fmt.Errorf("parse assistant message: %w", err)
		}
		return &msg, nil

	case "system":
		var msg SystemMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			return nil, fmt.Errorf("parse system message: %w", err)
		}
		return &msg, nil

	case "result":
		var msg ResultMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			return nil, fmt.Errorf("parse result message: %w", err)
		}
		return &msg, nil

	case "control_request":
		var msg ControlRequest
		if err := json.Unmarshal(raw, &msg); err != nil {
			return nil, fmt.Errorf("parse control request: %w", err)
		}
		return &msg, nil

	case "control_response":
		var msg ControlResponse
		if err := json.Unmarshal(raw, &msg); err != nil {
			return nil, fmt.Errorf("parse control response: %w", err)
		}
		return &msg, nil

	default:
		// 未知のメッセージ型は汎用マップで返す
		return &GenericMessage{Type: msgType, Data: data}, nil
	}
}

// GenericMessage は未知のメッセージ型を表す
type GenericMessage struct {
	Type string
	Data map[string]any
}

// MessageType はメッセージタイプを返す
func (m *GenericMessage) MessageType() string { return m.Type }
