# Issue #10: SessionID デッドロック修正設計

## 概要

`Connect()` 時に発生するデッドロックを修正する。

GitHub Issue: https://github.com/y-oga-819/my-go-claude-agent/issues/10

## 問題の原因

`Connect()` が `c.mu.Lock()` を保持したまま `receiveLoop` goroutine を開始し、`initialize()` を呼び出す。`receiveLoop` 内の `extractSessionIDFromRawMessage()` が `c.mu.Lock()` を取得しようとしてブロックされ、デッドロックが発生する。

```
Connect() [line 65]     : c.mu.Lock() 取得
Connect() [line 114]    : go c.receiveLoop(ctx) 開始
Connect() [line 117]    : c.initialize(ctx) → CLIにリクエスト送信
receiveLoop() [line 196]: extractSessionIDFromRawMessage() 呼び出し
extractSessionIDFromRawMessage() [line 207]: c.mu.Lock() → ブロック！
→ HandleIncoming() が呼ばれない → 30秒タイムアウト
```

## 修正方針

`sessionID` の管理を `c.mu`（Client全体のミューテックス）から分離し、`atomic.Pointer[string]` を使用したロックフリー設計に変更する。

### 採用理由

| 方針 | 性能 | 安全性 | シンプルさ |
|------|------|--------|-----------|
| 専用ミューテックス | △ | ○ | ○ |
| sync.Once + atomic.Value | ○ | ○ | △ |
| **atomic.Pointer[string]** | **○** | **○** | **○** |

SDKとして性能と安全性を最優先するため、`atomic.Pointer[string]` を採用。

## 修正内容

### 1. Client 構造体の変更

```go
// Before
type Client struct {
    // ...
    sessionID string
    mu        sync.RWMutex
    // ...
}

// After
type Client struct {
    // ...
    sessionID atomic.Pointer[string]  // ロックフリー
    mu        sync.RWMutex            // sessionID以外の状態管理用
    // ...
}
```

### 2. sessionID 関連メソッドの修正

#### extractSessionIDFromRawMessage

```go
// Before
func (c *Client) extractSessionIDFromRawMessage(rawMsg transport.RawMessage) {
    c.mu.Lock()
    defer c.mu.Unlock()

    if c.sessionID != "" {
        return
    }
    // sessionID を設定
    c.sessionID = sid
}

// After
func (c *Client) extractSessionIDFromRawMessage(rawMsg transport.RawMessage) {
    // ロック不要 - atomic操作で安全
    if c.sessionID.Load() != nil {
        return
    }
    // CompareAndSwap で初回のみ設定
    c.sessionID.CompareAndSwap(nil, &sid)
}
```

#### extractSessionIDFromResponse

```go
// Before
func (c *Client) extractSessionIDFromResponse(resp *protocol.ControlResponse) {
    if c.sessionID != "" {
        return
    }
    c.sessionID = sid
}

// After
func (c *Client) extractSessionIDFromResponse(resp *protocol.ControlResponse) {
    if c.sessionID.Load() != nil {
        return
    }
    c.sessionID.CompareAndSwap(nil, &sid)
}
```

#### SessionID / SessionIDReady

```go
// Before
func (c *Client) SessionID() (string, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if c.sessionID == "" {
        return "", ErrSessionIDNotReady
    }
    return c.sessionID, nil
}

// After
func (c *Client) SessionID() (string, error) {
    sid := c.sessionID.Load()
    if sid == nil {
        return "", ErrSessionIDNotReady
    }
    return *sid, nil
}
```

#### Send / SendToolResult

sessionID の参照部分を `c.sessionID.Load()` に変更。

### 3. 修正対象ファイル

- `claude/client.go`
- `claude/client_test.go`（必要に応じて）

## テスト観点

1. **デッドロック解消**: `Connect()` が30秒以内に完了すること
2. **sessionID 取得**: 複数の goroutine から同時にアクセスしても安全であること
3. **既存機能**: `Send()`, `SessionID()`, `SessionIDReady()` が正常に動作すること

## 環境要件

- Go 1.19+ (`atomic.Pointer[T]` は Go 1.19 で追加)
- 現在の go.mod: Go 1.23 → 問題なし
