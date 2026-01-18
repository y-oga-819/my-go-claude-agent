# my-go-claude-agent

Go言語によるClaude Agent SDK（非公式）

## 概要

Claude Code CLIと通信し、プログラムからClaudeのエージェント機能を利用するためのGoライブラリ。
公式Python SDK（anthropics/claude-agent-sdk-python）の設計を参考に、同等の機能をGo言語で再実装する。

## 目的

- Claude Code CLIをサブプロセスとして起動し、JSON/RPCプロトコルで通信
- ワンショットクエリと双方向ストリーミングの両方をサポート
- ツール権限管理（canUseTool）とフック機能の提供
- MCP（Model Context Protocol）サーバーの統合

## 主な機能

1. **ワンショットクエリ**: 単一のプロンプトを送信し、結果を受け取る
2. **双方向ストリーミング**: マルチターン会話、リアルタイムメッセージ処理
3. **ツール権限管理**: ツール実行前に許可/拒否を制御
4. **フックシステム**: PreToolUse, PostToolUse等のイベントに対するコールバック
5. **MCPサーバー統合**: 外部MCPサーバーおよびインプロセスMCPサーバーのサポート
6. **セッション管理**: セッションの継続、分岐、ファイルチェックポイント

## 前提条件

- Claude Code CLI v2.0.0以上がインストールされていること
- Claudeサブスクリプション（Pro/Max）またはAnthropic APIキー

## 参考

- [anthropics/claude-agent-sdk-python](https://github.com/anthropics/claude-agent-sdk-python)
- [Claude Code CLI Reference](https://code.claude.com/docs/en/cli-reference)
- [claude-cli-protocol](https://github.com/mzhaom/claude-cli-protocol)
