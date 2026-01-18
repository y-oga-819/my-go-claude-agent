# Step 6: MCPサーバー統合

## 目的

外部MCPサーバーおよびインプロセス（SDK）MCPサーバーを統合する。

## 成果物

- `internal/mcp/server.go` - SDKMCPServer
- `internal/mcp/external.go` - 外部MCPサーバー管理

## 主要な実装

### 6.1 外部MCPサーバー設定

```go
// MCPServerConfig は外部MCPサーバーの設定
type MCPServerConfig struct {
    Type    MCPTransportType      // stdio, sse, http
    Command string                // stdio用
    Args    []string              // stdio用
    URL     string                // sse/http用
    Headers map[string]string     // sse/http用
    Env     map[string]string
}

type MCPTransportType string

const (
    MCPTransportStdio MCPTransportType = "stdio"
    MCPTransportSSE   MCPTransportType = "sse"
    MCPTransportHTTP  MCPTransportType = "http"
)
```

### 6.2 SDK MCPサーバー（インプロセス）

```go
// Tool はMCPツールを定義
type Tool struct {
    Name        string
    Description string
    InputSchema map[string]any
    Handler     ToolHandler
}

type ToolHandler func(args map[string]any) (*ToolResult, error)

type ToolResult struct {
    Content []ContentBlock
    IsError bool
}

// SDKMCPServer はインプロセスMCPサーバー
type SDKMCPServer struct {
    Name    string
    Version string
    Tools   []Tool
}

// NewSDKMCPServer は新しいSDKMCPServerを作成
func NewSDKMCPServer(name, version string) *SDKMCPServer

// AddTool はツールを追加
func (s *SDKMCPServer) AddTool(tool Tool)

// HandleCall はツール呼び出しを処理
func (s *SDKMCPServer) HandleCall(toolName string, args map[string]any) (*ToolResult, error)
```

### 6.3 使用例

```go
// SDK MCPサーバーを作成
calcServer := mcp.NewSDKMCPServer("calculator", "1.0.0")
calcServer.AddTool(mcp.Tool{
    Name:        "add",
    Description: "Add two numbers",
    InputSchema: map[string]any{
        "type": "object",
        "properties": map[string]any{
            "a": map[string]any{"type": "number"},
            "b": map[string]any{"type": "number"},
        },
        "required": []string{"a", "b"},
    },
    Handler: func(args map[string]any) (*mcp.ToolResult, error) {
        a := args["a"].(float64)
        b := args["b"].(float64)
        return &mcp.ToolResult{
            Content: []mcp.ContentBlock{
                {Type: "text", Text: fmt.Sprintf("%.2f + %.2f = %.2f", a, b, a+b)},
            },
        }, nil
    },
})

client := claude.NewClient(&claude.Options{
    MCPServers: map[string]any{
        "calc": calcServer,
        "external-server": claude.MCPServerConfig{
            Type:    claude.MCPTransportStdio,
            Command: "python",
            Args:    []string{"-m", "my_mcp_server"},
        },
    },
    AllowedTools: []string{"mcp__calc__add"},
})
```

### 6.4 MCPメッセージ処理

CLIからの`mcp_message`制御リクエストを処理する。

```go
func (h *ProtocolHandler) handleMCPMessage(req MCPMessageRequest) error {
    server := h.mcpServers[req.ServerName]
    if server == nil {
        return h.sendControlError(req.RequestID, "unknown server")
    }

    switch req.Message.Method {
    case "tools/call":
        result, err := server.HandleCall(
            req.Message.Params.Name,
            req.Message.Params.Arguments,
        )
        return h.sendMCPResponse(req.RequestID, req.Message.ID, result, err)

    case "tools/list":
        tools := server.ListTools()
        return h.sendMCPResponse(req.RequestID, req.Message.ID, tools, nil)
    }
}
```

## 完了条件

- [ ] SDK MCPサーバーでツールを定義できる
- [ ] CLIからのmcp_messageリクエストに応答できる
- [ ] 外部MCPサーバーの設定をCLIに渡せる
