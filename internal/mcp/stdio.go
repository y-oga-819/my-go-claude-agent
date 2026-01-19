package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

// StdioTransport はstdioベースのトランスポート
type StdioTransport struct {
	command string
	args    []string
	env     map[string]string

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	stderr io.ReadCloser

	mu     sync.Mutex
	closed bool
}

// NewStdioTransport は新しいStdioTransportを作成する
func NewStdioTransport(config *ServerConfig) *StdioTransport {
	return &StdioTransport{
		command: config.Command,
		args:    config.Args,
		env:     config.Env,
		closed:  true,
	}
}

// Connect はサブプロセスを起動して接続する
func (t *StdioTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.closed {
		return errors.New("already connected")
	}

	t.cmd = exec.CommandContext(ctx, t.command, t.args...)

	// 環境変数を設定
	if len(t.env) > 0 {
		t.cmd.Env = os.Environ()
		for k, v := range t.env {
			t.cmd.Env = append(t.cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// stdin/stdout/stderrをパイプで接続
	stdin, err := t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	t.stdin = stdin

	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	t.stdout = bufio.NewReader(stdout)

	stderr, err := t.cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	t.stderr = stderr

	// プロセスを起動
	if err := t.cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return fmt.Errorf("failed to start process: %w", err)
	}

	t.closed = false
	return nil
}

// Send はメッセージを送信する
func (t *StdioTransport) Send(msg *Message) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return errors.New("not connected")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// 改行で区切る
	data = append(data, '\n')

	_, err = t.stdin.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

// Receive はメッセージを受信する
func (t *StdioTransport) Receive() (*Message, error) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil, errors.New("not connected")
	}
	stdout := t.stdout
	t.mu.Unlock()

	// 1行読み取り
	line, err := stdout.ReadBytes('\n')
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("connection closed: %w", err)
		}
		return nil, fmt.Errorf("failed to read message: %w", err)
	}

	var msg Message
	if err := json.Unmarshal(line, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return &msg, nil
}

// Close は接続を閉じる
func (t *StdioTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	t.closed = true

	var errs []error

	if t.stdin != nil {
		if err := t.stdin.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close stdin: %w", err))
		}
	}

	if t.stderr != nil {
		if err := t.stderr.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close stderr: %w", err))
		}
	}

	if t.cmd != nil && t.cmd.Process != nil {
		// プロセスの終了を待つ
		_ = t.cmd.Wait()
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// IsConnected は接続状態を返す
func (t *StdioTransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return !t.closed
}
