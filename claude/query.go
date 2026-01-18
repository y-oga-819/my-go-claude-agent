package claude

import (
	"context"
	"fmt"
	"strings"

	"github.com/y-oga-819/my-go-claude-agent/internal/protocol"
	"github.com/y-oga-819/my-go-claude-agent/internal/transport"
)

// QueryResult はクエリの結果を表す
type QueryResult struct {
	Messages  []protocol.Message
	Result    *protocol.ResultMessage
	SessionID string
	TotalCost float64
	Usage     protocol.Usage
}

// Query は単一のプロンプトを送信し、結果を返す
func Query(ctx context.Context, prompt string, opts *Options) (*QueryResult, error) {
	if opts == nil {
		opts = &Options{}
	}

	// Transport設定
	config := transport.Config{
		CLIPath:       opts.CLIPath,
		CWD:           opts.CWD,
		StreamingMode: false, // ワンショットモード
		Args:          buildQueryArgs(prompt, opts),
	}

	t := transport.NewSubprocessTransport(config)

	// 接続
	if err := t.Connect(ctx); err != nil {
		return nil, &SDKError{Op: "connect", Err: ErrCLIConnection, Details: err.Error()}
	}
	defer t.Close()

	// stdinをクローズしてプロンプト送信完了を通知
	if err := t.EndInput(); err != nil {
		return nil, &SDKError{Op: "end_input", Err: err}
	}

	// メッセージ収集
	result := &QueryResult{
		Messages: make([]protocol.Message, 0),
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case err := <-t.Errors():
			if err != nil {
				return nil, &SDKError{Op: "receive", Err: err}
			}

		case rawMsg, ok := <-t.Messages():
			if !ok {
				// チャネルがクローズされた
				if result.Result == nil {
					return nil, &SDKError{Op: "receive", Err: ErrProcessExited}
				}
				return result, nil
			}

			msg, err := protocol.ParseMessage(rawMsg.Data)
			if err != nil {
				return nil, &SDKError{Op: "parse", Err: ErrMessageParse, Details: err.Error()}
			}

			switch m := msg.(type) {
			case *protocol.AssistantMessage:
				result.Messages = append(result.Messages, m)

			case *protocol.SystemMessage:
				result.Messages = append(result.Messages, m)

			case *protocol.ResultMessage:
				result.Result = m
				result.SessionID = m.SessionID
				result.TotalCost = m.TotalCostUSD
				result.Usage = m.Usage
				// 結果を受け取ったら終了
				return result, nil
			}
		}
	}
}

func buildQueryArgs(prompt string, opts *Options) []string {
	args := []string{}

	// システムプロンプト
	if opts.SystemPrompt != "" {
		args = append(args, "--system-prompt", opts.SystemPrompt)
	}
	if opts.AppendSystemPrompt != "" {
		args = append(args, "--append-system-prompt", opts.AppendSystemPrompt)
	}

	// モデル設定
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	if opts.FallbackModel != "" {
		args = append(args, "--fallback-model", opts.FallbackModel)
	}

	// 制限設定
	if opts.MaxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", opts.MaxTurns))
	}
	if opts.MaxBudgetUSD > 0 {
		args = append(args, "--max-budget-usd", fmt.Sprintf("%.2f", opts.MaxBudgetUSD))
	}

	// 権限設定
	if opts.PermissionMode != "" {
		args = append(args, "--permission-mode", string(opts.PermissionMode))
	}
	if len(opts.AllowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(opts.AllowedTools, ","))
	}
	if len(opts.DisallowedTools) > 0 {
		args = append(args, "--disallowedTools", strings.Join(opts.DisallowedTools, ","))
	}

	// セッション設定
	if opts.Resume != "" {
		args = append(args, "--resume", opts.Resume)
	}

	// プロンプト（ワンショットモード）- 最後に配置
	args = append(args, "--print", "--", prompt)

	return args
}
