package transport

import "context"

// RawMessage はCLIから受信した生のJSONメッセージ
type RawMessage struct {
	Type string
	Data map[string]any
	Raw  []byte // 元のJSON
}

// Transport はCLIとの通信を抽象化するインターフェース
type Transport interface {
	// Connect はCLIプロセスを起動して接続する
	Connect(ctx context.Context) error

	// Write はCLIのstdinにデータを書き込む
	Write(data []byte) error

	// Messages は受信メッセージのチャネルを返す
	Messages() <-chan RawMessage

	// Errors はエラーのチャネルを返す
	Errors() <-chan error

	// EndInput はstdinをクローズする（ワンショットモード用）
	EndInput() error

	// Close はプロセスを終了する
	Close() error

	// IsConnected は接続状態を返す
	IsConnected() bool
}

// Config はTransportの設定
type Config struct {
	CLIPath       string            // CLIのパス
	Args          []string          // コマンドライン引数
	Env           map[string]string // 追加の環境変数
	CWD           string            // 作業ディレクトリ
	StreamingMode bool              // 双方向ストリーミングモード
	MaxBufferSize int               // JSONバッファの最大サイズ
}
