package claude

import (
	"errors"
	"fmt"
	"time"
)

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

	// API制限エラー
	ErrRateLimit       = errors.New("rate limit exceeded")
	ErrTokenLimit      = errors.New("token limit exceeded")
	ErrContextTooLong  = errors.New("context too long")
	ErrBudgetExceeded  = errors.New("budget exceeded")
	ErrTurnsExceeded   = errors.New("max turns exceeded")
	ErrQuotaExhausted  = errors.New("quota exhausted")

	// 認証エラー
	ErrAuthentication  = errors.New("authentication failed")
	ErrInvalidAPIKey   = errors.New("invalid API key")
	ErrSubscriptionRequired = errors.New("subscription required")

	// 権限エラー
	ErrToolDenied      = errors.New("tool execution denied")
	ErrPermissionDenied = errors.New("permission denied")

	// 中断エラー
	ErrInterrupted     = errors.New("operation interrupted")
	ErrCanceled        = errors.New("operation canceled")

	// 設定エラー
	ErrInvalidConfig   = errors.New("invalid configuration")
	ErrModelNotFound   = errors.New("model not found")
)

// ExitCode はCLI終了コードを表す
type ExitCode int

const (
	ExitCodeSuccess          ExitCode = 0
	ExitCodeError            ExitCode = 1
	ExitCodeAuthFailure      ExitCode = 2
	ExitCodeConfigError      ExitCode = 3
	ExitCodeRateLimit        ExitCode = 4
	ExitCodeBudgetExceeded   ExitCode = 5
	ExitCodeInterrupted      ExitCode = 130 // SIGINT
)

// SDKError はエラーの詳細情報を含む
type SDKError struct {
	Op         string        // 操作名
	Err        error         // 元エラー
	Details    string        // 追加情報
	ExitCode   ExitCode      // CLI終了コード
	Retryable  bool          // リトライ可能か
	RetryAfter time.Duration // リトライまでの待ち時間
}

func (e *SDKError) Error() string {
	msg := e.Op + ": " + e.Err.Error()
	if e.Details != "" {
		msg += " (" + e.Details + ")"
	}
	if e.ExitCode != 0 {
		msg += fmt.Sprintf(" [exit code: %d]", e.ExitCode)
	}
	return msg
}

func (e *SDKError) Unwrap() error {
	return e.Err
}

// IsRetryable はエラーがリトライ可能かを返す
func (e *SDKError) IsRetryable() bool {
	return e.Retryable
}

// APIError はAPIからのエラーレスポンスを表す
type APIError struct {
	Type       string `json:"type"`
	Message    string `json:"message"`
	StatusCode int    `json:"status_code,omitempty"`
}

func (e *APIError) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("%s: %s (status: %d)", e.Type, e.Message, e.StatusCode)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// ResultError はResult messageに含まれるエラー情報
type ResultError struct {
	Code    string // エラーコード
	Message string // エラーメッセージ
	Details any    // 追加詳細
}

func (e *ResultError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return e.Message
}

// NewSDKError は新しいSDKErrorを作成する
func NewSDKError(op string, err error) *SDKError {
	return &SDKError{
		Op:  op,
		Err: err,
	}
}

// NewSDKErrorWithDetails は詳細情報付きのSDKErrorを作成する
func NewSDKErrorWithDetails(op string, err error, details string) *SDKError {
	return &SDKError{
		Op:      op,
		Err:     err,
		Details: details,
	}
}

// NewRetryableError はリトライ可能なエラーを作成する
func NewRetryableError(op string, err error, retryAfter time.Duration) *SDKError {
	return &SDKError{
		Op:         op,
		Err:        err,
		Retryable:  true,
		RetryAfter: retryAfter,
	}
}

// IsRetryable はエラーがリトライ可能かを判定する
func IsRetryable(err error) bool {
	var sdkErr *SDKError
	if errors.As(err, &sdkErr) {
		return sdkErr.Retryable
	}
	// 一般的にリトライ可能なエラー
	return errors.Is(err, ErrRateLimit) || errors.Is(err, ErrControlTimeout)
}

// IsAuthError は認証関連のエラーかを判定する
func IsAuthError(err error) bool {
	return errors.Is(err, ErrAuthentication) ||
		errors.Is(err, ErrInvalidAPIKey) ||
		errors.Is(err, ErrSubscriptionRequired)
}

// IsLimitError は制限関連のエラーかを判定する
func IsLimitError(err error) bool {
	return errors.Is(err, ErrRateLimit) ||
		errors.Is(err, ErrTokenLimit) ||
		errors.Is(err, ErrContextTooLong) ||
		errors.Is(err, ErrBudgetExceeded) ||
		errors.Is(err, ErrTurnsExceeded) ||
		errors.Is(err, ErrQuotaExhausted)
}

// IsPermissionError は権限関連のエラーかを判定する
func IsPermissionError(err error) bool {
	return errors.Is(err, ErrToolDenied) || errors.Is(err, ErrPermissionDenied)
}

// GetRetryAfter はリトライまでの待ち時間を取得する
func GetRetryAfter(err error) time.Duration {
	var sdkErr *SDKError
	if errors.As(err, &sdkErr) {
		return sdkErr.RetryAfter
	}
	return 0
}

// ErrorFromExitCode は終了コードからエラーを生成する
func ErrorFromExitCode(code int) error {
	switch ExitCode(code) {
	case ExitCodeSuccess:
		return nil
	case ExitCodeAuthFailure:
		return ErrAuthentication
	case ExitCodeConfigError:
		return ErrInvalidConfig
	case ExitCodeRateLimit:
		return ErrRateLimit
	case ExitCodeBudgetExceeded:
		return ErrBudgetExceeded
	case ExitCodeInterrupted:
		return ErrInterrupted
	default:
		return fmt.Errorf("CLI exited with code %d", code)
	}
}
