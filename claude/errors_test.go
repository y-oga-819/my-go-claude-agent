package claude

import (
	"errors"
	"testing"
	"time"
)

func TestSDKError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *SDKError
		contains []string
	}{
		{
			name: "without details",
			err: &SDKError{
				Op:  "Connect",
				Err: ErrCLINotFound,
			},
			contains: []string{"Connect", "claude CLI not found"},
		},
		{
			name: "with details",
			err: &SDKError{
				Op:      "Connect",
				Err:     ErrCLIConnection,
				Details: "timeout after 30s",
			},
			contains: []string{"Connect", "CLI connection error", "timeout after 30s"},
		},
		{
			name: "with exit code",
			err: &SDKError{
				Op:       "Query",
				Err:      ErrRateLimit,
				ExitCode: ExitCodeRateLimit,
			},
			contains: []string{"Query", "rate limit exceeded", "exit code: 4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			for _, want := range tt.contains {
				if !containsString(got, want) {
					t.Errorf("SDKError.Error() = %q, should contain %q", got, want)
				}
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestSDKError_Unwrap(t *testing.T) {
	originalErr := ErrCLINotFound
	sdkErr := &SDKError{
		Op:  "Connect",
		Err: originalErr,
	}

	if !errors.Is(sdkErr, originalErr) {
		t.Error("errors.Is(sdkErr, originalErr) should be true")
	}

	unwrapped := sdkErr.Unwrap()
	if unwrapped != originalErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, originalErr)
	}
}

func TestSDKError_IsRetryable(t *testing.T) {
	tests := []struct {
		name      string
		err       *SDKError
		retryable bool
	}{
		{
			name: "retryable error",
			err: &SDKError{
				Op:        "Query",
				Err:       ErrRateLimit,
				Retryable: true,
			},
			retryable: true,
		},
		{
			name: "non-retryable error",
			err: &SDKError{
				Op:  "Query",
				Err: ErrAuthentication,
			},
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.IsRetryable(); got != tt.retryable {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.retryable)
			}
		})
	}
}

func TestSentinelErrors(t *testing.T) {
	sentinelErrors := []error{
		// CLIエラー
		ErrCLINotFound,
		ErrCLIConnection,
		ErrProcessExited,
		// プロトコルエラー
		ErrJSONDecode,
		ErrMessageParse,
		ErrControlTimeout,
		ErrBufferOverflow,
		// セッションエラー
		ErrSessionNotFound,
		// API制限エラー
		ErrRateLimit,
		ErrTokenLimit,
		ErrContextTooLong,
		ErrBudgetExceeded,
		ErrTurnsExceeded,
		ErrQuotaExhausted,
		// 認証エラー
		ErrAuthentication,
		ErrInvalidAPIKey,
		ErrSubscriptionRequired,
		// 権限エラー
		ErrToolDenied,
		ErrPermissionDenied,
		// 中断エラー
		ErrInterrupted,
		ErrCanceled,
		// 設定エラー
		ErrInvalidConfig,
		ErrModelNotFound,
	}

	for _, err := range sentinelErrors {
		if err == nil {
			t.Error("Sentinel error should not be nil")
		}
		if err.Error() == "" {
			t.Error("Sentinel error message should not be empty")
		}
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "rate limit error",
			err:       ErrRateLimit,
			retryable: true,
		},
		{
			name:      "control timeout",
			err:       ErrControlTimeout,
			retryable: true,
		},
		{
			name:      "authentication error",
			err:       ErrAuthentication,
			retryable: false,
		},
		{
			name: "sdk error with retryable flag",
			err: &SDKError{
				Op:        "Query",
				Err:       ErrRateLimit,
				Retryable: true,
			},
			retryable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.retryable {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.retryable)
			}
		})
	}
}

func TestIsAuthError(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		isAuth  bool
	}{
		{"authentication", ErrAuthentication, true},
		{"invalid api key", ErrInvalidAPIKey, true},
		{"subscription required", ErrSubscriptionRequired, true},
		{"rate limit", ErrRateLimit, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAuthError(tt.err); got != tt.isAuth {
				t.Errorf("IsAuthError() = %v, want %v", got, tt.isAuth)
			}
		})
	}
}

func TestIsLimitError(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		isLimit bool
	}{
		{"rate limit", ErrRateLimit, true},
		{"token limit", ErrTokenLimit, true},
		{"context too long", ErrContextTooLong, true},
		{"budget exceeded", ErrBudgetExceeded, true},
		{"turns exceeded", ErrTurnsExceeded, true},
		{"quota exhausted", ErrQuotaExhausted, true},
		{"authentication", ErrAuthentication, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsLimitError(tt.err); got != tt.isLimit {
				t.Errorf("IsLimitError() = %v, want %v", got, tt.isLimit)
			}
		})
	}
}

func TestIsPermissionError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		isPermission bool
	}{
		{"tool denied", ErrToolDenied, true},
		{"permission denied", ErrPermissionDenied, true},
		{"rate limit", ErrRateLimit, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPermissionError(tt.err); got != tt.isPermission {
				t.Errorf("IsPermissionError() = %v, want %v", got, tt.isPermission)
			}
		})
	}
}

func TestGetRetryAfter(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		retryAfter time.Duration
	}{
		{
			name: "with retry after",
			err: &SDKError{
				Op:         "Query",
				Err:        ErrRateLimit,
				RetryAfter: 5 * time.Second,
			},
			retryAfter: 5 * time.Second,
		},
		{
			name:       "without retry after",
			err:        ErrRateLimit,
			retryAfter: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetRetryAfter(tt.err); got != tt.retryAfter {
				t.Errorf("GetRetryAfter() = %v, want %v", got, tt.retryAfter)
			}
		})
	}
}

func TestErrorFromExitCode(t *testing.T) {
	tests := []struct {
		code     int
		expected error
	}{
		{0, nil},
		{2, ErrAuthentication},
		{3, ErrInvalidConfig},
		{4, ErrRateLimit},
		{5, ErrBudgetExceeded},
		{130, ErrInterrupted},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := ErrorFromExitCode(tt.code)
			if tt.expected == nil && got != nil {
				t.Errorf("ErrorFromExitCode(%d) = %v, want nil", tt.code, got)
			} else if tt.expected != nil && !errors.Is(got, tt.expected) {
				t.Errorf("ErrorFromExitCode(%d) = %v, want %v", tt.code, got, tt.expected)
			}
		})
	}
}

func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *APIError
		contains []string
	}{
		{
			name: "with status code",
			err: &APIError{
				Type:       "rate_limit_error",
				Message:    "Rate limit exceeded",
				StatusCode: 429,
			},
			contains: []string{"rate_limit_error", "Rate limit exceeded", "429"},
		},
		{
			name: "without status code",
			err: &APIError{
				Type:    "invalid_request",
				Message: "Invalid request",
			},
			contains: []string{"invalid_request", "Invalid request"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			for _, want := range tt.contains {
				if !containsSubstring(got, want) {
					t.Errorf("APIError.Error() = %q, should contain %q", got, want)
				}
			}
		})
	}
}

func TestResultError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *ResultError
		expected string
	}{
		{
			name: "with code",
			err: &ResultError{
				Code:    "rate_limit",
				Message: "Rate limit exceeded",
			},
			expected: "rate_limit: Rate limit exceeded",
		},
		{
			name: "without code",
			err: &ResultError{
				Message: "Unknown error",
			},
			expected: "Unknown error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("ResultError.Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestNewSDKError(t *testing.T) {
	err := NewSDKError("Query", ErrRateLimit)
	if err.Op != "Query" {
		t.Errorf("Op = %q, want %q", err.Op, "Query")
	}
	if !errors.Is(err, ErrRateLimit) {
		t.Error("should wrap ErrRateLimit")
	}
}

func TestNewSDKErrorWithDetails(t *testing.T) {
	err := NewSDKErrorWithDetails("Query", ErrRateLimit, "retry after 5s")
	if err.Op != "Query" {
		t.Errorf("Op = %q, want %q", err.Op, "Query")
	}
	if err.Details != "retry after 5s" {
		t.Errorf("Details = %q, want %q", err.Details, "retry after 5s")
	}
}

func TestNewRetryableError(t *testing.T) {
	err := NewRetryableError("Query", ErrRateLimit, 5*time.Second)
	if !err.Retryable {
		t.Error("Retryable should be true")
	}
	if err.RetryAfter != 5*time.Second {
		t.Errorf("RetryAfter = %v, want %v", err.RetryAfter, 5*time.Second)
	}
}
