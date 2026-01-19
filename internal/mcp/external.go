package mcp

import (
	"context"
	"fmt"
	"sync"
)

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
	servers    map[string]*ServerConfig
	sdkServers map[string]*SDKMCPServer
	clients    map[string]*MCPClient // 接続中のクライアント
	mu         sync.RWMutex
}

// NewManager は新しいManagerを作成する
func NewManager() *Manager {
	return &Manager{
		servers:    make(map[string]*ServerConfig),
		sdkServers: make(map[string]*SDKMCPServer),
		clients:    make(map[string]*MCPClient),
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

// ConnectServer は外部サーバーに接続する
func (m *Manager) ConnectServer(ctx context.Context, name string) error {
	m.mu.Lock()
	config, ok := m.servers[name]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("server not found: %s", name)
	}
	m.mu.Unlock()

	var transport Transport
	switch config.Type {
	case TransportStdio:
		transport = NewStdioTransport(config)
	case TransportHTTP, TransportSSE:
		transport = NewHTTPTransport(config)
	default:
		return fmt.Errorf("unsupported transport type: %s", config.Type)
	}

	client := NewMCPClient(name, transport)
	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	m.mu.Lock()
	m.clients[name] = client
	m.mu.Unlock()

	return nil
}

// DisconnectServer はサーバーから切断する
func (m *Manager) DisconnectServer(name string) error {
	m.mu.Lock()
	client, ok := m.clients[name]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("client not connected: %s", name)
	}
	delete(m.clients, name)
	m.mu.Unlock()

	return client.Close()
}

// GetClient は接続中のクライアントを取得する
func (m *Manager) GetClient(name string) (*MCPClient, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	client, ok := m.clients[name]
	return client, ok
}

// CallTool はツールを呼び出す（SDK/外部両対応）
func (m *Manager) CallTool(ctx context.Context, serverName, toolName string, args map[string]any) (*ToolResult, error) {
	// SDKサーバーを優先
	m.mu.RLock()
	sdkServer, sdkOk := m.sdkServers[serverName]
	client, clientOk := m.clients[serverName]
	m.mu.RUnlock()

	if sdkOk {
		return sdkServer.HandleCall(toolName, args)
	}

	if clientOk {
		return client.CallTool(ctx, toolName, args)
	}

	return nil, fmt.Errorf("server not found: %s", serverName)
}

// DisconnectAll は全ての接続を切断する
func (m *Manager) DisconnectAll() error {
	m.mu.Lock()
	clients := make([]*MCPClient, 0, len(m.clients))
	for _, c := range m.clients {
		clients = append(clients, c)
	}
	m.clients = make(map[string]*MCPClient)
	m.mu.Unlock()

	var lastErr error
	for _, c := range clients {
		if err := c.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
