package protocol

// Message はCLIとやり取りするメッセージの共通インターフェース
type Message interface {
	MessageType() string
}

// UserMessage はユーザーからのメッセージ
type UserMessage struct {
	Type            string      `json:"type"` // "user"
	Message         UserContent `json:"message"`
	ParentToolUseID *string     `json:"parent_tool_use_id,omitempty"`
	SessionID       string      `json:"session_id"`
}

func (m *UserMessage) MessageType() string { return m.Type }

// UserContent はユーザーメッセージの内容
type UserContent struct {
	Role    string `json:"role"` // "user"
	Content any    `json:"content"` // string or []ContentBlock
}

// AssistantMessage はアシスタントからのメッセージ
type AssistantMessage struct {
	Type            string        `json:"type"` // "assistant"
	Message         AssistantBody `json:"message"`
	ParentToolUseID *string       `json:"parent_tool_use_id,omitempty"`
}

func (m *AssistantMessage) MessageType() string { return m.Type }

// AssistantBody はアシスタントメッセージの本体
type AssistantBody struct {
	Role    string         `json:"role"` // "assistant"
	Model   string         `json:"model"`
	Content []ContentBlock `json:"content"`
	Error   *string        `json:"error,omitempty"`
}

// ContentBlock はメッセージ内のコンテンツブロック
type ContentBlock struct {
	Type string `json:"type"`
	// type別のフィールド
	Text      string         `json:"text,omitempty"`      // type: "text"
	Thinking  string         `json:"thinking,omitempty"`  // type: "thinking"
	Signature string         `json:"signature,omitempty"` // type: "thinking"
	ID        string         `json:"id,omitempty"`        // type: "tool_use"
	Name      string         `json:"name,omitempty"`      // type: "tool_use"
	Input     map[string]any `json:"input,omitempty"`     // type: "tool_use"
}

// SystemMessage はシステムメッセージ
type SystemMessage struct {
	Type    string         `json:"type"` // "system"
	Subtype string         `json:"subtype"`
	Data    map[string]any `json:"data"`
}

func (m *SystemMessage) MessageType() string { return m.Type }

// ResultMessage は結果メッセージ
type ResultMessage struct {
	Type             string         `json:"type"`    // "result"
	Subtype          string         `json:"subtype"` // "query_complete"
	DurationMs       int64          `json:"duration_ms"`
	DurationAPIMs    int64          `json:"duration_api_ms"`
	IsError          bool           `json:"is_error"`
	NumTurns         int            `json:"num_turns"`
	SessionID        string         `json:"session_id"`
	TotalCostUSD     float64        `json:"total_cost_usd"`
	Usage            Usage          `json:"usage"`
	Result           string         `json:"result,omitempty"`
	StructuredOutput map[string]any `json:"structured_output,omitempty"`
}

func (m *ResultMessage) MessageType() string { return m.Type }

// Usage はトークン使用量を表す
type Usage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheCreationTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// ControlRequest は制御リクエスト
type ControlRequest struct {
	Type      string `json:"type"` // "control_request"
	RequestID string `json:"request_id"`
	Request   any    `json:"request"`
}

func (m *ControlRequest) MessageType() string { return m.Type }

// ControlResponse は制御レスポンス
type ControlResponse struct {
	Type     string              `json:"type"` // "control_response"
	Response ControlResponseBody `json:"response"`
}

func (m *ControlResponse) MessageType() string { return m.Type }

// ControlResponseBody は制御レスポンスの本体
type ControlResponseBody struct {
	Subtype   string `json:"subtype"` // "success" or "error"
	RequestID string `json:"request_id"`
	Response  any    `json:"response,omitempty"`
	Error     string `json:"error,omitempty"`
}
