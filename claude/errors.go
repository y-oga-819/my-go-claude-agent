package claude

import "errors"

var (
	// CLIエラー
	ErrCLINotFound   = errors.New("claude CLI not found")
	ErrCLIConnection = errors.New("CLI connection error")
	ErrProcessExited = errors.New("CLI process exited unexpectedly")

	// プロトコルエラー
	ErrJSONDecode     = errors.New("JSON decode error")
	ErrMessageParse   = errors.New("message parse error")
	ErrControlTimeout = errors.New("control request timeout")
	ErrBufferOverflow = errors.New("JSON buffer overflow")

	// セッションエラー
	ErrSessionNotFound = errors.New("session not found")
)

// SDKError はエラーの詳細情報を含む
type SDKError struct {
	Op      string // 操作名
	Err     error  // 元エラー
	Details string // 追加情報
}

func (e *SDKError) Error() string {
	if e.Details != "" {
		return e.Op + ": " + e.Err.Error() + " (" + e.Details + ")"
	}
	return e.Op + ": " + e.Err.Error()
}

func (e *SDKError) Unwrap() error {
	return e.Err
}
