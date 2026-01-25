// Issue #10 デッドロック修正の動作確認スクリプト
//
// 修正前: Connect()が30秒でタイムアウト（デッドロック）
// 修正後: Connect()が正常に完了
//
// 実行: go run examples/verify-deadlock-fix/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/y-oga-819/my-go-claude-agent/claude"
)

func main() {
	fmt.Println("=== Issue #10 デッドロック修正 動作確認 ===")
	fmt.Println()

	// タイムアウトを10秒に設定（修正前は30秒でタイムアウトしていた）
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := claude.NewClient(&claude.Options{
		PermissionMode: claude.PermissionModeBypassPermissions,
	})
	defer client.Close()

	fmt.Println("1. Connect() を開始...")
	startTime := time.Now()

	stream, err := client.Connect(ctx)
	elapsed := time.Since(startTime)

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Printf("   FAIL: タイムアウト（%v） - デッドロックが発生している可能性\n", elapsed)
			os.Exit(1)
		}
		fmt.Printf("   FAIL: エラー発生（%v）: %v\n", elapsed, err)
		os.Exit(1)
	}

	fmt.Printf("   OK: Connect() 完了（%v）\n", elapsed)
	fmt.Println()

	// SessionID の確認
	fmt.Println("2. SessionID を確認...")
	sid, err := stream.SessionID()
	if err != nil {
		fmt.Printf("   INFO: SessionID未取得（正常）: %v\n", err)
	} else {
		fmt.Printf("   OK: SessionID = %s\n", sid)
	}
	fmt.Println()

	// 簡単なメッセージ送信テスト
	fmt.Println("3. メッセージ送信テスト...")
	if err := stream.Send(ctx, "Hello, this is a test message. Please respond with 'OK'."); err != nil {
		fmt.Printf("   FAIL: Send() エラー: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("   OK: メッセージ送信完了")
	fmt.Println()

	// レスポンス受信（5秒待機）
	fmt.Println("4. レスポンス受信...")
	receiveCtx, receiveCancel := context.WithTimeout(ctx, 5*time.Second)
	defer receiveCancel()

	messageCount := 0
	for {
		select {
		case msg, ok := <-stream.Messages():
			if !ok {
				fmt.Println("   INFO: メッセージチャネルがクローズ")
				goto done
			}
			messageCount++
			msgType := msg.MessageType()
			fmt.Printf("   受信[%d]: type=%s\n", messageCount, msgType)

			// resultメッセージを受信したら終了
			if msgType == "result" {
				fmt.Println("   OK: result メッセージ受信")
				goto done
			}

		case err := <-stream.Errors():
			fmt.Printf("   WARN: エラー受信: %v\n", err)

		case <-receiveCtx.Done():
			fmt.Println("   INFO: 受信タイムアウト（5秒）")
			goto done
		}
	}

done:
	fmt.Println()

	// 最終的なSessionID確認
	fmt.Println("5. 最終 SessionID 確認...")
	sid, err = stream.SessionID()
	if err != nil {
		fmt.Printf("   WARN: SessionID取得失敗: %v\n", err)
	} else {
		fmt.Printf("   OK: SessionID = %s\n", sid)
	}
	fmt.Println()

	fmt.Println("=== 検証完了 ===")
	fmt.Println()
	fmt.Println("結果: デッドロックは発生していません")
}
