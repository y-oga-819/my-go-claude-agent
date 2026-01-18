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

最もシンプルな使い方。プロンプトを送信し、結果を受け取ります。

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

### オプション付きクエリ

```go
result, err := claude.Query(ctx, "コードをレビューして", &claude.Options{
    CWD:            "/path/to/project",
    Model:          "claude-sonnet-4-5",
    MaxTurns:       5,
    PermissionMode: claude.PermissionModeAcceptEdits,
    SystemPrompt:   "あなたはコードレビューの専門家です",
})
```

### セッション継続（Resume）

前回のセッションを継続して、会話の文脈を保持できます。

```go
// 最初のクエリ
result1, err := claude.Query(ctx, "私の名前は太郎です", nil)
if err != nil {
    log.Fatal(err)
}

// セッションを継続して2回目のクエリ
result2, err := claude.Query(ctx, "私の名前は？", &claude.Options{
    Resume: result1.SessionID,
})
// → "太郎さんです" と回答される
```

### セッション分岐（Fork）

既存のセッションから分岐して、別の方向に会話を進めることができます。

```go
result, err := claude.Query(ctx, "別のアプローチを試して", &claude.Options{
    Resume:      "previous-session-id",
    ForkSession: true,  // 分岐して新しいセッションを作成
})
```

### 双方向ストリーミング

対話的な通信が必要な場合に使用します。

```go
import (
    "github.com/y-oga-819/my-go-claude-agent/claude"
    "github.com/y-oga-819/my-go-claude-agent/internal/protocol"
)

client := claude.NewClient(&claude.Options{
    Model:    "claude-sonnet-4-5",
    MaxTurns: 10,
})

stream, err := client.Connect(ctx)
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// メッセージ送信
err = stream.Send(ctx, "ファイルを読んで")
if err != nil {
    log.Fatal(err)
}

// レスポンス受信
for msg := range stream.Messages() {
    switch m := msg.(type) {
    case *protocol.AssistantMessage:
        for _, block := range m.Message.Content {
            if block.Type == "text" {
                fmt.Println(block.Text)
            }
        }
    case *protocol.ResultMessage:
        fmt.Printf("完了: %s\n", m.SessionID)
        return
    }
}
```

## Options

| フィールド | 型 | 説明 |
|-----------|-----|------|
| `CLIPath` | `string` | CLIのパス（デフォルト: "claude"） |
| `CWD` | `string` | 作業ディレクトリ |
| `SystemPrompt` | `string` | システムプロンプト |
| `AppendSystemPrompt` | `string` | システムプロンプトへの追加 |
| `Model` | `string` | 使用するモデル |
| `MaxTurns` | `int` | 最大ターン数 |
| `MaxBudgetUSD` | `float64` | 最大予算（USD） |
| `PermissionMode` | `PermissionMode` | 権限モード |
| `AllowedTools` | `[]string` | 許可するツール |
| `DisallowedTools` | `[]string` | 禁止するツール |
| `Resume` | `string` | 再開するセッションID |
| `ForkSession` | `bool` | セッションを分岐するか |
| `Continue` | `bool` | 直前のセッションを継続 |

### PermissionMode

| モード | 説明 |
|--------|------|
| `PermissionModeDefault` | デフォルト（都度確認） |
| `PermissionModeAcceptEdits` | ファイル編集を自動許可 |
| `PermissionModePlan` | 読み取り専用（計画モード） |
| `PermissionModeBypassPermissions` | 全て自動許可 |

## 機能

- ✅ ワンショットクエリ
- ✅ 双方向ストリーミング
- ✅ セッション管理（resume, fork, continue）
- ✅ ツール権限管理（canUseTool）
- ✅ フックシステム（PreToolUse, PostToolUse等）
- ✅ MCP（Model Context Protocol）サーバー統合
- ✅ ファイルチェックポイント

## パッケージ構成

```
claude/              # 公開API
  ├── query.go       # ワンショットQuery
  ├── client.go      # 双方向ストリーミングClient
  ├── options.go     # オプション定義
  ├── session.go     # セッション管理
  └── errors.go      # エラー定義

internal/
  ├── transport/     # CLI通信層
  ├── protocol/      # メッセージ・制御プロトコル
  ├── hooks/         # フックシステム
  ├── permission/    # 権限管理
  └── mcp/           # MCPサーバー統合
```

## ドキュメント

詳細な設計ドキュメントは [docs/design/](docs/design/) を参照してください。

## ライセンス

MIT License

## 参考

- [anthropics/claude-agent-sdk-python](https://github.com/anthropics/claude-agent-sdk-python)
- [Claude Code CLI Reference](https://docs.anthropic.com/en/docs/claude-code)
