package mcp

import "context"

// Transport はMCP通信のトランスポート層インターフェース
type Transport interface {
	// Connect は接続を確立する
	Connect(ctx context.Context) error
	// Send はメッセージを送信する
	Send(msg *Message) error
	// Receive はメッセージを受信する（ブロッキング）
	Receive() (*Message, error)
	// Close は接続を閉じる
	Close() error
	// IsConnected は接続状態を返す
	IsConnected() bool
}
