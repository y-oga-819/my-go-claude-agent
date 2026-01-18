package claude

import (
	"errors"
	"testing"
)

func TestSDKError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *SDKError
		expected string
	}{
		{
			name: "without details",
			err: &SDKError{
				Op:  "Connect",
				Err: ErrCLINotFound,
			},
			expected: "Connect: claude CLI not found",
		},
		{
			name: "with details",
			err: &SDKError{
				Op:      "Connect",
				Err:     ErrCLIConnection,
				Details: "timeout after 30s",
			},
			expected: "Connect: CLI connection error (timeout after 30s)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("SDKError.Error() = %q, want %q", got, tt.expected)
			}
		})
	}
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

func TestSentinelErrors(t *testing.T) {
	// センチネルエラーが正しく定義されていることを確認
	sentinelErrors := []error{
		ErrCLINotFound,
		ErrCLIConnection,
		ErrProcessExited,
		ErrJSONDecode,
		ErrMessageParse,
		ErrControlTimeout,
		ErrBufferOverflow,
		ErrSessionNotFound,
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
