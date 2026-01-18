# my-go-claude-agent

Go言語によるClaude Agent SDK（非公式）

## 概要

Claude Code CLIと通信し、プログラムからClaudeのエージェント機能を利用するためのGoライブラリ。

公式Python SDK（[anthropics/claude-agent-sdk-python](https://github.com/anthropics/claude-agent-sdk-python)）の設計を参考に、同等の機能をGo言語で再実装しています。

## 前提条件

- Go 1.23以上
- Claude Code CLI v2.0.0以上
- Claudeサブスクリプション（Pro/Max）またはAnthropic APIキー

## インストール

```bash
go get github.com/y-oga-819/my-go-claude-agent
```

## クイックスタート

### ワンショットクエリ

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/y-oga-819/my-go-claude-agent/claude"
)

func main() {
    ctx := context.Background()

    result, err := claude.Query(ctx, "Hello, Claude!", nil)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Session: %s\n", result.SessionID)
    fmt.Printf("Cost: $%.4f\n", result.TotalCost)
}
```

### 双方向ストリーミング

```go
client := claude.NewClient(&claude.Options{
    Model:    "claude-sonnet-4-5",
    MaxTurns: 10,
})

stream, err := client.Stream(ctx)
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// メッセージ送信
client.Send(claude.UserMessage{Content: "ファイルを読んで"})

// レスポンス受信
for msg := range stream.Messages() {
    switch m := msg.(type) {
    case *claude.AssistantMessage:
        for _, block := range m.Content {
            if block.Type == "text" {
                fmt.Println(block.Text)
            }
        }
    }
}
```

## 機能

- ✅ ワンショットクエリ
- ✅ 双方向ストリーミング
- ✅ ツール権限管理（canUseTool）
- ✅ フックシステム（PreToolUse, PostToolUse等）
- ✅ MCP（Model Context Protocol）サーバー統合
- ✅ セッション管理（resume, fork）

## ドキュメント

詳細な設計ドキュメントは [docs/design/](docs/design/) を参照してください。

## ライセンス

MIT License

## 参考

- [anthropics/claude-agent-sdk-python](https://github.com/anthropics/claude-agent-sdk-python)
- [Claude Code CLI Reference](https://code.claude.com/docs/en/cli-reference)
