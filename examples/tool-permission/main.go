// ツール使用許可の対話的確認サンプル
//
// このサンプルは、canUseToolコールバックを使用してツールの使用許可を
// 対話的に確認する方法を示します。
//
// デフォルトでは、canUseToolコールバックが未設定の場合、すべてのツールが
// 自動的に許可されます。このサンプルでは、ツールの種類に応じて：
// - 読み取り系ツール: 自動許可
// - 書き込み系ツール: ユーザーに確認
//
// 実行: go run examples/tool-permission/main.go
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/y-oga-819/my-go-claude-agent/claude"
	"github.com/y-oga-819/my-go-claude-agent/internal/protocol"
)

// 読み取り専用ツール（自動許可）
var readOnlyTools = map[string]bool{
	"Read":        true,
	"Glob":        true,
	"Grep":        true,
	"LSP":         true,
	"WebSearch":   true,
	"WebFetch":    true,
	"TaskList":    true,
	"TaskGet":     true,
	"TaskOutput":  true,
}

// 常に許可するツール
var alwaysAllowTools = map[string]bool{
	"AskUserQuestion": true, // ユーザーへの質問は常に許可
}

// 常に拒否するツール
var alwaysDenyTools = map[string]bool{
	// 必要に応じて追加
}

func main() {
	fmt.Println("=== ツール使用許可の対話的確認サンプル ===")
	fmt.Println()
	fmt.Println("このサンプルでは、ツールの使用許可を対話的に確認します。")
	fmt.Println("- 読み取り系ツール: 自動許可")
	fmt.Println("- 書き込み系ツール: ユーザーに確認を求めます")
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	reader := bufio.NewReader(os.Stdin)

	client := claude.NewClient(&claude.Options{
		// ツール使用許可のコールバックを設定
		CanUseTool: func(
			ctx context.Context,
			toolName string,
			input map[string]any,
			permCtx *claude.ToolPermissionContext,
		) (*claude.PermissionResult, error) {
			return handleToolPermission(reader, toolName, input, permCtx)
		},
	})
	defer client.Close()

	fmt.Println("Claude CLIに接続中...")
	stream, err := client.Connect(ctx)
	if err != nil {
		fmt.Printf("接続エラー: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("接続完了")
	fmt.Println()

	// 対話ループ
	for {
		fmt.Print("あなた: ")
		userInput, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		userInput = strings.TrimSpace(userInput)

		if userInput == "" {
			continue
		}
		if userInput == "exit" || userInput == "quit" {
			fmt.Println("終了します")
			break
		}

		// メッセージ送信
		if err := stream.Send(ctx, userInput); err != nil {
			fmt.Printf("送信エラー: %v\n", err)
			continue
		}

		// レスポンス受信
		if err := receiveResponse(ctx, stream); err != nil {
			fmt.Printf("受信エラー: %v\n", err)
		}
		fmt.Println()
	}
}

// handleToolPermission はツール使用許可を判定する
func handleToolPermission(
	reader *bufio.Reader,
	toolName string,
	input map[string]any,
	_ *claude.ToolPermissionContext,
) (*claude.PermissionResult, error) {
	// 常に拒否するツール
	if alwaysDenyTools[toolName] {
		fmt.Printf("\n[自動拒否] ツール '%s' は使用禁止です\n", toolName)
		return &claude.PermissionResult{
			Allow:   false,
			Message: fmt.Sprintf("ツール '%s' は使用が禁止されています", toolName),
		}, nil
	}

	// 常に許可するツール
	if alwaysAllowTools[toolName] {
		fmt.Printf("\n[自動許可] ツール '%s'\n", toolName)
		return &claude.PermissionResult{Allow: true}, nil
	}

	// 読み取り専用ツールは自動許可
	if readOnlyTools[toolName] {
		fmt.Printf("\n[自動許可] 読み取りツール '%s'\n", toolName)
		return &claude.PermissionResult{Allow: true}, nil
	}

	// その他のツールはユーザーに確認
	return askUserPermission(reader, toolName, input)
}

// askUserPermission はユーザーにツール使用許可を確認する
func askUserPermission(
	reader *bufio.Reader,
	toolName string,
	input map[string]any,
) (*claude.PermissionResult, error) {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║               ツール使用許可の確認                           ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Printf("ツール名: %s\n", toolName)
	fmt.Println("入力パラメータ:")

	// 入力パラメータを整形して表示
	inputJSON, _ := json.MarshalIndent(input, "  ", "  ")
	// 長すぎる場合は省略
	inputStr := string(inputJSON)
	if len(inputStr) > 500 {
		inputStr = inputStr[:500] + "\n  ... (省略)"
	}
	fmt.Printf("  %s\n", inputStr)
	fmt.Println()

	// ツール固有の情報を表示
	showToolSpecificInfo(toolName, input)

	fmt.Print("このツールの実行を許可しますか? [y/n/a]: ")
	fmt.Println("  y = 許可, n = 拒否, a = 今後このツールを自動許可")

	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))

	switch answer {
	case "y", "yes":
		fmt.Println("-> 許可しました")
		return &claude.PermissionResult{Allow: true}, nil

	case "a", "always":
		fmt.Printf("-> 許可しました（'%s' は今後自動許可）\n", toolName)
		alwaysAllowTools[toolName] = true
		return &claude.PermissionResult{Allow: true}, nil

	default:
		fmt.Println("-> 拒否しました")
		return &claude.PermissionResult{
			Allow:   false,
			Message: "ユーザーによって拒否されました",
		}, nil
	}
}

// showToolSpecificInfo はツール固有の情報を表示する
func showToolSpecificInfo(toolName string, input map[string]any) {
	switch toolName {
	case "Bash":
		if cmd, ok := input["command"].(string); ok {
			fmt.Printf("実行コマンド: %s\n", cmd)
		}
	case "Write":
		if path, ok := input["file_path"].(string); ok {
			fmt.Printf("書き込み先: %s\n", path)
		}
	case "Edit":
		if path, ok := input["file_path"].(string); ok {
			fmt.Printf("編集対象: %s\n", path)
		}
	case "Task":
		if desc, ok := input["description"].(string); ok {
			fmt.Printf("タスク: %s\n", desc)
		}
	}
	fmt.Println()
}

// receiveResponse はCLIからのレスポンスを受信して表示する
func receiveResponse(ctx context.Context, stream *claude.Stream) error {
	fmt.Print("Claude: ")

	for {
		select {
		case msg, ok := <-stream.Messages():
			if !ok {
				return nil
			}

			switch m := msg.(type) {
			case *protocol.AssistantMessage:
				// テキストコンテンツを表示
				for _, block := range m.Message.Content {
					if block.Type == "text" {
						fmt.Print(block.Text)
					}
				}

			case *protocol.ResultMessage:
				fmt.Println()
				return nil
			}

		case err := <-stream.Errors():
			return err

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
