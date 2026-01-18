package claude

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// RetryConfig はリトライの設定
type RetryConfig struct {
	MaxRetries     int           // 最大リトライ回数（デフォルト: 3）
	InitialBackoff time.Duration // 初期バックオフ（デフォルト: 1秒）
	MaxBackoff     time.Duration // 最大バックオフ（デフォルト: 30秒）
	BackoffFactor  float64       // バックオフ倍率（デフォルト: 2.0）
	Jitter         bool          // ジッターを追加するか（デフォルト: true）
}

// DefaultRetryConfig はデフォルトのリトライ設定を返す
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
		BackoffFactor:  2.0,
		Jitter:         true,
	}
}

// RetryableFunc はリトライ可能な関数の型
type RetryableFunc func(ctx context.Context) error

// WithRetry はリトライ付きで関数を実行する
func WithRetry(ctx context.Context, config *RetryConfig, fn RetryableFunc) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	backoff := config.InitialBackoff

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		err := fn(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		// リトライ可能なエラーかチェック
		if !IsRetryable(err) {
			return err
		}

		// 最後のリトライ後はリトライしない
		if attempt >= config.MaxRetries {
			break
		}

		// RetryAfterが指定されている場合はそれを使用
		if retryAfter := GetRetryAfter(err); retryAfter > 0 {
			backoff = retryAfter
		}

		// バックオフ待機
		waitDuration := backoff
		if config.Jitter {
			// ジッターを追加（±25%）
			jitter := float64(waitDuration) * 0.25 * (rand.Float64()*2 - 1)
			waitDuration = time.Duration(float64(waitDuration) + jitter)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitDuration):
		}

		// 次のバックオフを計算
		backoff = time.Duration(float64(backoff) * config.BackoffFactor)
		if backoff > config.MaxBackoff {
			backoff = config.MaxBackoff
		}
	}

	return &SDKError{
		Op:      "retry",
		Err:     lastErr,
		Details: "max retries exceeded",
	}
}

// QueryWithRetry はリトライ付きでQueryを実行する
func QueryWithRetry(ctx context.Context, prompt string, opts *Options, retryConfig *RetryConfig) (*QueryResult, error) {
	var result *QueryResult

	err := WithRetry(ctx, retryConfig, func(ctx context.Context) error {
		var queryErr error
		result, queryErr = Query(ctx, prompt, opts)
		return queryErr
	})

	return result, err
}

// ExponentialBackoff は指数バックオフの待ち時間を計算する
func ExponentialBackoff(attempt int, initial, max time.Duration, factor float64) time.Duration {
	backoff := float64(initial) * math.Pow(factor, float64(attempt))
	if backoff > float64(max) {
		backoff = float64(max)
	}
	return time.Duration(backoff)
}

// AddJitter は待ち時間にジッターを追加する
func AddJitter(d time.Duration, factor float64) time.Duration {
	jitter := float64(d) * factor * (rand.Float64()*2 - 1)
	return time.Duration(float64(d) + jitter)
}
