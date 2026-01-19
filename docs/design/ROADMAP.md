# Go Claude Agent SDK - ロードマップ

公式TypeScript/Python SDKとの機能比較と今後の実装計画。

> 最終更新: 2026-01-19
>
> 参考: [TypeScript SDK Reference](https://platform.claude.com/docs/en/agent-sdk/typescript)

## 実装完了度サマリー

| 領域 | 実装済み | 総数 | 完了率 |
|------|---------|------|--------|
| コア関数 | 4 | 4 | **100%** |
| オプション | 16 | 24 | **67%** |
| メッセージタイプ | 4 | 6 | **67%** |
| Query メソッド | 2 | 10 | **20%** |
| フック | 7 | 12 | **58%** |
| MCP機能 | 3 | 4 | **75%** |

### **総合完了率: 約 65%**

---

## 詳細比較表

### コア関数

| 公式SDK機能 | Go実装状況 | 完了度 |
|------------|-----------|--------|
| query() / claudeCode() | ✅ Query() 実装済み | 100% |
| createSession() | ✅ session.go 実装済み | 100% |
| resumeSession() | ✅ Resume オプション | 100% |
| V2 send()/stream() | ✅ Client/Stream 実装済み | 100% |

### オプション (Options)

| 公式SDK機能 | Go実装状況 | 完了度 |
|------------|-----------|--------|
| model | ✅ | 100% |
| systemPrompt | ✅ | 100% |
| maxTurns | ✅ | 100% |
| maxBudgetUsd | ✅ | 100% |
| cwd | ✅ | 100% |
| env | ✅ | 100% |
| permissionMode | ✅ | 100% |
| allowedTools / disallowedTools | ✅ | 100% |
| resume / forkSession / continue | ✅ | 100% |
| enableFileCheckpointing | ✅ | 100% |
| hooks | ✅ | 100% |
| mcpServers | ✅ | 100% |
| canUseTool | ✅ | 100% |
| abortController | ✅ Context使用 | 100% |
| fallbackModel | ✅ | 100% |
| additionalDirectories | ❌ 未実装 | 0% |
| agents (サブエージェント定義) | ❌ 未実装 | 0% |
| outputFormat (構造化出力) | ❌ 未実装 | 0% |
| plugins | ❌ 未実装 | 0% |
| sandbox | ❌ 未実装 | 0% |
| settingSources | ❌ 未実装 | 0% |
| betas | ❌ 未実装 | 0% |
| includePartialMessages | ❌ 未実装 | 0% |
| maxThinkingTokens | ❌ 未実装 | 0% |

### メッセージタイプ

| 公式SDK機能 | Go実装状況 | 完了度 |
|------------|-----------|--------|
| SDKAssistantMessage | ✅ | 100% |
| SDKUserMessage | ✅ | 100% |
| SDKResultMessage | ✅ | 100% |
| SDKSystemMessage | ✅ | 100% |
| SDKPartialAssistantMessage | ❌ 未実装 | 0% |
| SDKCompactBoundaryMessage | ❌ 未実装 | 0% |

### Query メソッド

| 公式SDK機能 | Go実装状況 | 完了度 |
|------------|-----------|--------|
| interrupt() | ✅ | 100% |
| rewindFiles() | ✅ | 100% |
| setPermissionMode() | ❌ 未実装 | 0% |
| setModel() | ❌ 未実装 | 0% |
| setMaxThinkingTokens() | ❌ 未実装 | 0% |
| supportedCommands() | ❌ 未実装 | 0% |
| supportedModels() | ❌ 未実装 | 0% |
| mcpServerStatus() | ❌ 未実装 | 0% |
| accountInfo() | ❌ 未実装 | 0% |

### フック (Hooks)

| 公式SDK機能 | Go実装状況 | 完了度 |
|------------|-----------|--------|
| PreToolUse | ✅ | 100% |
| PostToolUse | ✅ | 100% |
| PostToolUseFailure | ❌ 未実装 | 0% |
| Notification | ✅ | 100% |
| UserPromptSubmit | ✅ | 100% |
| SessionStart | ❌ 未実装 | 0% |
| SessionEnd | ❌ 未実装 | 0% |
| Stop | ✅ | 100% |
| SubagentStart | ❌ 未実装 | 0% |
| SubagentStop | ✅ | 100% |
| PreCompact | ✅ | 100% |
| PermissionRequest | ❌ 未実装 | 0% |

### MCP機能

| 公式SDK機能 | Go実装状況 | 完了度 |
|------------|-----------|--------|
| tool() 定義 | ✅ | 100% |
| createSdkMcpServer() | ✅ MCPServer実装 | 100% |
| Stdio トランスポート | ✅ | 100% |
| HTTP トランスポート | ✅ | 100% |
| SSE トランスポート | ❌ 未実装 | 0% |

---

## 実装不要な機能

以下の機能は本Go SDKでは実装予定なし：

- **従量課金API連携** - Claude Codeの従量課金APIは使用しない方針

---

## 今後の実装計画

### Phase 1: 高優先度

動的制御とサーバー情報取得機能。

| 機能 | 説明 | 関連ファイル |
|------|------|-------------|
| `setModel()` | 実行中のモデル変更 | `claude/client.go` |
| `setPermissionMode()` | 実行中の権限モード変更 | `claude/client.go` |
| `supportedModels()` | 利用可能モデル一覧取得 | `internal/protocol/` |
| `accountInfo()` | アカウント情報取得 | `internal/protocol/` |
| SSE トランスポート | MCP SSEサーバー接続 | `internal/mcp/sse.go` |
| 追加フック | `SessionStart`, `SessionEnd`, `PostToolUseFailure`, `PermissionRequest`, `SubagentStart` | `internal/hooks/` |

### Phase 2: 中優先度

高度な機能拡張。

| 機能 | 説明 | 関連ファイル |
|------|------|-------------|
| `outputFormat` | JSON Schema ベースの構造化出力 | `claude/options.go` |
| `agents` | プログラマティックなサブエージェント定義 | `claude/options.go` |
| `sandbox` | コマンド実行のサンドボックス化 | `claude/options.go` |
| `includePartialMessages` | ストリーミング中の部分メッセージ | `internal/protocol/` |
| `SDKCompactBoundaryMessage` | コンパクト境界メッセージ | `internal/protocol/` |

### Phase 3: 低優先度

必要に応じて実装。

| 機能 | 説明 |
|------|------|
| `plugins` | ローカルプラグインのロード |
| `settingSources` | ファイルシステム設定の読み込み制御 |
| `betas` | ベータ機能フラグ |
| `additionalDirectories` | 追加アクセスディレクトリ |
| `maxThinkingTokens` | 思考トークン制限 |

---

## 完了したマイルストーン

### v0.1.0 - 基盤実装 ✅

- [x] プロジェクト初期化
- [x] Transport層（subprocess通信）
- [x] ワンショットQuery API
- [x] 制御プロトコル（Initialize, Interrupt, RewindFiles）
- [x] フックシステム（7種類のイベント）
- [x] 権限管理（canUseTool）
- [x] MCPクライアント・サーバー統合
- [x] セッション管理（Resume, Fork, Continue, FileCheckpointing）

---

## 参考リンク

- [公式 TypeScript SDK Reference](https://platform.claude.com/docs/en/agent-sdk/typescript)
- [公式 TypeScript SDK V2 Preview](https://platform.claude.com/docs/en/agent-sdk/typescript-v2-preview)
- [公式 Python SDK](https://github.com/anthropics/claude-agent-sdk-python)
- [Claude Code CLI Reference](https://docs.anthropic.com/en/docs/claude-code)
