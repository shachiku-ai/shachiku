package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"shachiku/core/memory"
	"shachiku/core/models"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

func generateAnthropic(ctx context.Context, cfg models.LLMConfig, history []models.Message, systemPrompt string, taskID uint) (string, error) {
	apiKey := cfg.AnthropicAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		return "Mock Claude response. Provide ANTHROPIC_API_KEY to see real generations.", nil
	}

	client := anthropic.NewClient(option.WithAPIKey(apiKey))

	fileRegex := regexp.MustCompile(`(?m)^@(/.*)$`)

	messages := []anthropic.MessageParam{}
	for _, msg := range history {
		content := msg.Content
		matches := fileRegex.FindAllStringSubmatch(content, -1)
		var blocks []anthropic.ContentBlockParamUnion

		for _, m := range matches {
			path := strings.TrimSpace(m[1])
			content = strings.ReplaceAll(content, m[0], "")

			data, err := os.ReadFile(path)
			if err != nil {
				continue // skip on error
			}

			contentType := http.DetectContentType(data)
			if strings.HasPrefix(contentType, "image/") {
				if contentType == "image/jpeg" || contentType == "image/png" || contentType == "image/gif" || contentType == "image/webp" {
					blocks = append(blocks, anthropic.NewImageBlockBase64(contentType, base64.StdEncoding.EncodeToString(data)))
				}
			} else if utf8.Valid(data) {
				// Text files
				blocks = append(blocks, anthropic.NewTextBlock(fmt.Sprintf("\n\n[Attached File: %s]\n%s\n", path, string(data))))
			} else {
				blocks = append(blocks, anthropic.NewTextBlock(fmt.Sprintf("\n\n[Attached File: %s] (binary file omitted, no direct support. Use bash/python tools to read if needed.)\n", path)))
			}
		}

		content = strings.TrimSpace(content)
		if content != "" {
			blocks = append(blocks, anthropic.NewTextBlock(content))
		}

		if len(blocks) == 0 {
			blocks = append(blocks, anthropic.NewTextBlock(" "))
		}

		if msg.Role == "agent" {
			messages = append(messages, anthropic.NewAssistantMessage(blocks...))
		} else {
			messages = append(messages, anthropic.NewUserMessage(blocks...))
		}
	}

	model := cfg.Model
	if model == "" {
		model = string(anthropic.ModelClaude3_7SonnetLatest)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	req := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: 8192,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
		Messages:  messages,
	}

	reqJSON, _ := json.MarshalIndent(req, "", "  ")
	fmt.Printf("=== [Anthropic API Request] ===\n%s\n===============================\n", string(reqJSON))

	resp, err := client.Messages.New(reqCtx, req)

	if err != nil {
		return "", fmt.Errorf("anthropic error: %v", err)
	}

	memory.LogTokenUsage(taskID, int(resp.Usage.InputTokens), int(resp.Usage.OutputTokens))

	if len(resp.Content) > 0 {
		return resp.Content[0].Text, nil
	}
	return "", fmt.Errorf("empty claude response")
}
