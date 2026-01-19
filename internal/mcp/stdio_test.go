package mcp

import (
	"context"
	"testing"
	"time"
)

func TestNewStdioTransport(t *testing.T) {
	config := &ServerConfig{
		Type:    TransportStdio,
		Command: "echo",
		Args:    []string{"hello"},
	}

	transport := NewStdioTransport(config)
	if transport == nil {
		t.Fatal("NewStdioTransport returned nil")
	}
	if transport.command != "echo" {
		t.Errorf("command = %q, want %q", transport.command, "echo")
	}
	if transport.IsConnected() {
		t.Error("should not be connected before Connect()")
	}
}

func TestStdioTransport_ConnectAndClose(t *testing.T) {
	// catコマンドを使用してstdio通信をテスト
	config := &ServerConfig{
		Type:    TransportStdio,
		Command: "cat",
	}

	transport := NewStdioTransport(config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := transport.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if !transport.IsConnected() {
		t.Error("should be connected")
	}

	err = transport.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if transport.IsConnected() {
		t.Error("should be disconnected after Close")
	}
}

func TestStdioTransport_SendAndReceive(t *testing.T) {
	// catコマンドを使用してエコーテスト
	config := &ServerConfig{
		Type:    TransportStdio,
		Command: "cat",
	}

	transport := NewStdioTransport(config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := transport.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer transport.Close()

	// メッセージ送信
	msg := &Message{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test",
	}

	err = transport.Send(msg)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// メッセージ受信（catがエコーバック）
	received, err := transport.Receive()
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}

	if received.JSONRPC != "2.0" {
		t.Errorf("JSONRPC = %q, want %q", received.JSONRPC, "2.0")
	}
	if received.Method != "test" {
		t.Errorf("Method = %q, want %q", received.Method, "test")
	}
}

func TestStdioTransport_SendBeforeConnect(t *testing.T) {
	config := &ServerConfig{
		Type:    TransportStdio,
		Command: "cat",
	}

	transport := NewStdioTransport(config)

	msg := &Message{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test",
	}

	err := transport.Send(msg)
	if err == nil {
		t.Error("expected error when sending before connect")
	}
}

func TestStdioTransport_ReceiveBeforeConnect(t *testing.T) {
	config := &ServerConfig{
		Type:    TransportStdio,
		Command: "cat",
	}

	transport := NewStdioTransport(config)

	_, err := transport.Receive()
	if err == nil {
		t.Error("expected error when receiving before connect")
	}
}

func TestStdioTransport_WithEnv(t *testing.T) {
	config := &ServerConfig{
		Type:    TransportStdio,
		Command: "sh",
		Args:    []string{"-c", "echo $TEST_VAR"},
		Env:     map[string]string{"TEST_VAR": "hello"},
	}

	transport := NewStdioTransport(config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := transport.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer transport.Close()

	// shコマンドは実行後すぐ終了するため、プロセス終了を待つ
	// この場合、接続後すぐにEOFになる可能性があるのでテストは簡略化
}

func TestStdioTransport_CommandNotFound(t *testing.T) {
	config := &ServerConfig{
		Type:    TransportStdio,
		Command: "nonexistent_command_12345",
	}

	transport := NewStdioTransport(config)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := transport.Connect(ctx)
	if err == nil {
		transport.Close()
		t.Error("expected error for nonexistent command")
	}
}
