package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// Executor はシェルコマンドフックを実行する
type Executor struct {
	shell string
}

// NewExecutor は新しいExecutorを作成する
func NewExecutor() *Executor {
	return &Executor{
		shell: "sh",
	}
}

// CommandInput はコマンドに渡すJSON
type CommandInput struct {
	SessionID      string         `json:"session_id"`
	TranscriptPath string         `json:"transcript_path,omitempty"`
	CWD            string         `json:"cwd,omitempty"`
	HookEventName  string         `json:"hook_event_name"`
	ToolName       string         `json:"tool_name,omitempty"`
	ToolInput      map[string]any `json:"tool_input,omitempty"`
	ToolOutput     map[string]any `json:"tool_output,omitempty"`
	ToolUseID      string         `json:"tool_use_id,omitempty"`
}

// CommandOutput はコマンドからの出力JSON
type CommandOutput struct {
	Continue           bool                   `json:"continue"`
	StopReason         string                 `json:"stopReason,omitempty"`
	SuppressOutput     bool                   `json:"suppressOutput,omitempty"`
	Decision           string                 `json:"decision,omitempty"`
	SystemMessage      string                 `json:"systemMessage,omitempty"`
	Reason             string                 `json:"reason,omitempty"`
	HookSpecificOutput *CommandSpecificOutput `json:"hookSpecificOutput,omitempty"`
}

// CommandSpecificOutput はフック固有の出力
type CommandSpecificOutput struct {
	HookEventName            string         `json:"hookEventName,omitempty"`
	PermissionDecision       string         `json:"permissionDecision,omitempty"`
	PermissionDecisionReason string         `json:"permissionDecisionReason,omitempty"`
	UpdatedInput             map[string]any `json:"updatedInput,omitempty"`
	AdditionalContext        string         `json:"additionalContext,omitempty"`
}

// Execute はコマンドを実行しOutputを返す
func (e *Executor) Execute(ctx context.Context, command string, input *Input, timeout time.Duration) (*Output, error) {
	// タイムアウト付きコンテキスト
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// コマンドを作成
	cmd := exec.CommandContext(ctx, e.shell, "-c", command)

	// 環境変数を設定
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("CLAUDE_SESSION_ID=%s", input.SessionID),
	)
	if input.CWD != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("CLAUDE_PROJECT_DIR=%s", input.CWD))
		cmd.Dir = input.CWD
	}

	// stdin用のJSONを作成
	cmdInput := CommandInput{
		SessionID:      input.SessionID,
		TranscriptPath: input.TranscriptPath,
		CWD:            input.CWD,
		HookEventName:  input.HookEventName,
		ToolName:       input.ToolName,
		ToolInput:      input.ToolInput,
		ToolOutput:     input.ToolOutput,
	}

	inputJSON, err := json.Marshal(cmdInput)
	if err != nil {
		return nil, fmt.Errorf("marshal input: %w", err)
	}

	// stdin/stdout/stderrを設定
	cmd.Stdin = bytes.NewReader(inputJSON)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// コマンド実行
	err = cmd.Run()

	// コンテキストエラーを先にチェック
	if ctx.Err() != nil {
		return nil, fmt.Errorf("execute command: %w", ctx.Err())
	}

	// 終了コードを取得
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			// シグナルで終了した場合（-1）はエラーとして扱う
			if exitCode == -1 {
				return nil, fmt.Errorf("execute command: process killed")
			}
		} else {
			// その他のエラー
			return nil, fmt.Errorf("execute command: %w", err)
		}
	}

	// 終了コードに応じた処理
	switch exitCode {
	case 0:
		// 成功: stdoutをJSONとしてパース
		return e.parseSuccessOutput(stdout.Bytes())

	case 2:
		// ブロック: stderrをエラーメッセージとして使用
		return &Output{
			Continue: false,
			Decision: "block",
			Reason:   stderr.String(),
		}, nil

	default:
		// 非ブロッキングエラー: 処理継続
		return &Output{
			Continue:      true,
			SystemMessage: stderr.String(),
		}, nil
	}
}

// parseSuccessOutput はexit 0時のstdoutをパースする
func (e *Executor) parseSuccessOutput(data []byte) (*Output, error) {
	// 空の出力は継続
	if len(bytes.TrimSpace(data)) == 0 {
		return &Output{Continue: true}, nil
	}

	var cmdOutput CommandOutput
	if err := json.Unmarshal(data, &cmdOutput); err != nil {
		// JSONパース失敗は継続（デフォルト動作）
		return &Output{Continue: true}, nil
	}

	// CommandOutputをOutputに変換
	output := &Output{
		Continue:       cmdOutput.Continue,
		StopReason:     cmdOutput.StopReason,
		SuppressOutput: cmdOutput.SuppressOutput,
		Decision:       cmdOutput.Decision,
		SystemMessage:  cmdOutput.SystemMessage,
		Reason:         cmdOutput.Reason,
	}

	// HookSpecificOutputを変換
	if cmdOutput.HookSpecificOutput != nil {
		output.HookSpecificOutput = &SpecificOutput{
			HookEventName:            cmdOutput.HookSpecificOutput.HookEventName,
			PermissionDecision:       cmdOutput.HookSpecificOutput.PermissionDecision,
			PermissionDecisionReason: cmdOutput.HookSpecificOutput.PermissionDecisionReason,
			UpdatedInput:             cmdOutput.HookSpecificOutput.UpdatedInput,
		}
	}

	return output, nil
}
