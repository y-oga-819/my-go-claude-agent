// AskUserQuestionツールの処理サンプル
//
// このサンプルは、canUseToolコールバックを使用してAskUserQuestionツールを
// 処理する方法を示します。
//
// AskUserQuestionツールは、CLIがユーザーに質問を投げかける際に使用されます。
// SDKを使用する場合、このツールはcanUseToolコールバックを通じて処理する
// 必要があります。
//
// 実行: go run examples/ask-user-question/main.go
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/y-oga-819/my-go-claude-agent/claude"
)

func main() {
	fmt.Println("=== AskUserQuestion ツール処理サンプル ===")
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	client := claude.NewClient(&claude.Options{
		// canUseToolコールバックを設定
		// AskUserQuestionを含むすべてのツール使用リクエストがここに来る
		CanUseTool: handleCanUseTool,
	})
	defer client.Close()

	fmt.Println("1. Connect() を開始...")
	stream, err := client.Connect(ctx)
	if err != nil {
		fmt.Printf("   FAIL: Connect() エラー: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("   OK: Connect() 完了")
	fmt.Println()

	// AskUserQuestionをトリガーするようなプロンプトを送信
	// 注: 実際にAskUserQuestionが呼ばれるかはモデルの判断による
	fmt.Println("2. メッセージ送信...")
	prompt := "Help me decide on the tech stack for a new mobile app. Ask me clarifying questions about my requirements before making recommendations."
	if err := stream.Send(ctx, prompt); err != nil {
		fmt.Printf("   FAIL: Send() エラー: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("   OK: メッセージ送信完了")
	fmt.Println()

	// レスポンス受信
	fmt.Println("3. レスポンス受信...")
	for {
		select {
		case msg, ok := <-stream.Messages():
			if !ok {
				fmt.Println("   INFO: メッセージチャネルがクローズ")
				goto done
			}
			msgType := msg.MessageType()
			fmt.Printf("   受信: type=%s\n", msgType)

			if msgType == "result" {
				fmt.Println("   OK: result メッセージ受信")
				goto done
			}

		case err := <-stream.Errors():
			fmt.Printf("   WARN: エラー受信: %v\n", err)

		case <-ctx.Done():
			fmt.Println("   INFO: タイムアウト")
			goto done
		}
	}

done:
	fmt.Println()
	fmt.Println("=== 完了 ===")
}

// handleCanUseTool はツール使用リクエストを処理するコールバック
func handleCanUseTool(
	ctx context.Context,
	toolName string,
	input map[string]any,
	permCtx *claude.ToolPermissionContext,
) (*claude.PermissionResult, error) {
	// AskUserQuestionツールの場合は特別な処理が必要
	if toolName == "AskUserQuestion" {
		return handleAskUserQuestion(ctx, input)
	}

	// その他のツールは許可
	fmt.Printf("   [canUseTool] ツール '%s' を許可\n", toolName)
	return &claude.PermissionResult{Allow: true}, nil
}

// handleAskUserQuestion はAskUserQuestionツールを処理する
func handleAskUserQuestion(_ context.Context, input map[string]any) (*claude.PermissionResult, error) {
	fmt.Println()
	fmt.Println("=== AskUserQuestion ===")

	// questionsを取得
	questionsRaw, ok := input["questions"].([]any)
	if !ok {
		fmt.Println("   ERROR: questions が見つかりません")
		return &claude.PermissionResult{
			Allow:   false,
			Message: "Invalid AskUserQuestion input: questions not found",
		}, nil
	}

	answers := make(map[string]string)
	reader := bufio.NewReader(os.Stdin)

	// 各質問を処理
	for _, qRaw := range questionsRaw {
		q, ok := qRaw.(map[string]any)
		if !ok {
			continue
		}

		question := getString(q, "question")
		header := getString(q, "header")
		multiSelect := getBool(q, "multiSelect")
		options := getOptions(q)

		fmt.Printf("\n[%s] %s\n", header, question)
		for i, opt := range options {
			fmt.Printf("  %d. %s - %s\n", i+1, opt.Label, opt.Description)
		}

		if multiSelect {
			fmt.Println("  (カンマ区切りで複数選択可能、または自由入力)")
		} else {
			fmt.Println("  (番号を入力、または自由入力)")
		}

		fmt.Print("回答: ")
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(response)

		// 回答を解析
		answer := parseResponse(response, options)
		answers[question] = answer
		fmt.Printf("   -> %s\n", answer)
	}

	fmt.Println()
	fmt.Println("=== AskUserQuestion 完了 ===")
	fmt.Println()

	// UpdatedInputに元のquestionsとanswersを含めて返す
	return &claude.PermissionResult{
		Allow: true,
		UpdatedInput: map[string]any{
			"questions": input["questions"],
			"answers":   answers,
		},
	}, nil
}

// Option は選択肢を表す
type Option struct {
	Label       string
	Description string
}

// getString はmapから文字列を取得する
func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// getBool はmapからboolを取得する
func getBool(m map[string]any, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

// getOptions はmapからoptionsを取得する
func getOptions(q map[string]any) []Option {
	optionsRaw, ok := q["options"].([]any)
	if !ok {
		return nil
	}

	var options []Option
	for _, optRaw := range optionsRaw {
		opt, ok := optRaw.(map[string]any)
		if !ok {
			continue
		}
		options = append(options, Option{
			Label:       getString(opt, "label"),
			Description: getString(opt, "description"),
		})
	}
	return options
}

// parseResponse はユーザーの入力を解析する
func parseResponse(response string, options []Option) string {
	// 空の場合はデフォルト（最初の選択肢）
	if response == "" && len(options) > 0 {
		return options[0].Label
	}

	// 番号が入力された場合
	parts := strings.Split(response, ",")
	var labels []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if num, err := strconv.Atoi(part); err == nil {
			if num >= 1 && num <= len(options) {
				labels = append(labels, options[num-1].Label)
			}
		}
	}

	if len(labels) > 0 {
		return strings.Join(labels, ", ")
	}

	// 自由入力
	return response
}
