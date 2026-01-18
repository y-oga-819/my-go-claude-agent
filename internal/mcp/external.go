package mcp

// TransportType はMCPトランスポートの種類
type TransportType string

const (
	TransportStdio TransportType = "stdio"
	TransportSSE   TransportType = "sse"
	TransportHTTP  TransportType = "http"
)

// ServerConfig は外部MCPサーバーの設定
type ServerConfig struct {
	Type    TransportType     `json:"type"`
	Command string            `json:"command,omitempty"`  // stdio用
	Args    []string          `json:"args,omitempty"`     // stdio用
	URL     string            `json:"url,omitempty"`      // sse/http用
	Headers map[string]string `json:"headers,omitempty"`  // sse/http用
	Env     map[string]string `json:"env,omitempty"`
}

// ToMap はServerConfigをmap[string]anyに変換する
// CLIへの設定渡しに使用
func (c *ServerConfig) ToMap() map[string]any {
	result := map[string]any{
		"type": string(c.Type),
	}

	switch c.Type {
	case TransportStdio:
		if c.Command != "" {
			result["command"] = c.Command
		}
		if len(c.Args) > 0 {
			result["args"] = c.Args
		}
	case TransportSSE, TransportHTTP:
		if c.URL != "" {
			result["url"] = c.URL
		}
		if len(c.Headers) > 0 {
			result["headers"] = c.Headers
		}
	}

	if len(c.Env) > 0 {
		result["env"] = c.Env
	}

	return result
}

// Manager は外部MCPサーバーを管理する
type Manager struct {
	servers map[string]*ServerConfig
	sdkServers map[string]*SDKMCPServer
}

// NewManager は新しいManagerを作成する
func NewManager() *Manager {
	return &Manager{
		servers:    make(map[string]*ServerConfig),
		sdkServers: make(map[string]*SDKMCPServer),
	}
}

// AddExternalServer は外部MCPサーバーを追加する
func (m *Manager) AddExternalServer(name string, config *ServerConfig) {
	m.servers[name] = config
}

// AddSDKServer はSDK MCPサーバーを追加する
func (m *Manager) AddSDKServer(name string, server *SDKMCPServer) {
	m.sdkServers[name] = server
}

// GetExternalServer は外部MCPサーバーを取得する
func (m *Manager) GetExternalServer(name string) (*ServerConfig, bool) {
	config, ok := m.servers[name]
	return config, ok
}

// GetSDKServer はSDK MCPサーバーを取得する
func (m *Manager) GetSDKServer(name string) (*SDKMCPServer, bool) {
	server, ok := m.sdkServers[name]
	return server, ok
}

// ListExternalServers は全ての外部MCPサーバーをリストする
func (m *Manager) ListExternalServers() map[string]*ServerConfig {
	result := make(map[string]*ServerConfig)
	for name, config := range m.servers {
		result[name] = config
	}
	return result
}

// ListSDKServers は全てのSDK MCPサーバーをリストする
func (m *Manager) ListSDKServers() map[string]*SDKMCPServer {
	result := make(map[string]*SDKMCPServer)
	for name, server := range m.sdkServers {
		result[name] = server
	}
	return result
}

// BuildCLIConfig はCLIに渡すMCP設定を構築する
func (m *Manager) BuildCLIConfig() map[string]any {
	result := make(map[string]any)

	// 外部サーバー
	for name, config := range m.servers {
		result[name] = config.ToMap()
	}

	// SDKサーバー（CLIにはSDKサーバーとして通知）
	for name := range m.sdkServers {
		result[name] = map[string]any{
			"type": "sdk",
		}
	}

	return result
}

// HandleMCPMessage はMCPメッセージを処理する
func (m *Manager) HandleMCPMessage(serverName string, msg *Message) (*Response, error) {
	// SDKサーバーを優先
	if server, ok := m.sdkServers[serverName]; ok {
		return server.HandleMessage(msg)
	}

	// 外部サーバーの場合はエラー（外部サーバーへの転送はCLIが行う）
	return &Response{
		ID:    msg.ID,
		Error: &ResponseError{Code: -32000, Message: "server not found or external server"},
	}, nil
}
