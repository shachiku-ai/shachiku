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

	openai "github.com/sashabaranov/go-openai"
)

func generateOpenAICompatible(ctx context.Context, cfg models.LLMConfig, history []models.Message, systemPrompt string, taskID uint) (string, error) {
	apiKey := cfg.OpenAICompatibleAPIKey
	if apiKey == "" {
		apiKey = "dummy" // Provider might not need a real key but go-openai expects one
	}

	config := openai.DefaultConfig(apiKey)
	if cfg.OpenAICompatibleEndpoint != "" {
		config.BaseURL = cfg.OpenAICompatibleEndpoint
	}

	client := openai.NewClientWithConfig(config)

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
	}

	fileRegex := regexp.MustCompile(`(?m)^@(/.*)$`)

	for _, msg := range history {
		role := openai.ChatMessageRoleUser
		if msg.Role == "agent" {
			role = openai.ChatMessageRoleAssistant
		}

		content := msg.Content
		matches := fileRegex.FindAllStringSubmatch(content, -1)

		if len(matches) == 0 {
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    role,
				Content: content,
			})
			continue
		}

		var parts []openai.ChatMessagePart
		for _, m := range matches {
			path := strings.TrimSpace(m[1])
			content = strings.ReplaceAll(content, m[0], "")

			data, err := os.ReadFile(path)
			if err != nil {
				continue // skip on error
			}

			contentType := http.DetectContentType(data)
			if strings.HasPrefix(contentType, "image/") {
				parts = append(parts, openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeImageURL,
					ImageURL: &openai.ChatMessageImageURL{
						URL: fmt.Sprintf("data:%s;base64,%s", contentType, base64.StdEncoding.EncodeToString(data)),
					},
				})
			} else if utf8.Valid(data) {
				parts = append(parts, openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeText,
					Text: fmt.Sprintf("\n\n[Attached File: %s]\n%s\n", path, string(data)),
				})
			} else {
				parts = append(parts, openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeText,
					Text: fmt.Sprintf("\n\n[Attached File: %s] (binary file omitted, no direct support. Use bash/python tools to read if needed.)\n", path),
				})
			}
		}

		content = strings.TrimSpace(content)
		if content != "" {
			parts = append(parts, openai.ChatMessagePart{
				Type: openai.ChatMessagePartTypeText,
				Text: content,
			})
		}

		messages = append(messages, openai.ChatCompletionMessage{
			Role:         role,
			MultiContent: parts,
		})
	}

	model := cfg.Model
	if model == "" {
		model = openai.GPT3Dot5Turbo // Default model name if none specified
	}

	reqCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	req := openai.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
	}

	reqJSON, _ := json.MarshalIndent(req, "", "  ")
	fmt.Printf("=== [OpenAI Compatible API Request] ===\n%s\n============================\n", string(reqJSON))

	resp, err := client.CreateChatCompletion(reqCtx, req)

	if err != nil {
		return "", fmt.Errorf("openai compatible error: %v", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openai compatible error: successfully connected but no choices returned in the response (possibly blocked or filtered)")
	}

	promptTokens := resp.Usage.PromptTokens
	completionTokens := resp.Usage.CompletionTokens
	if promptTokens == 0 && completionTokens == 0 {
		// Fallback for providers that don't return usage
		promptTokens = len(systemPrompt) / 4
		for _, m := range history {
			promptTokens += len(m.Content) / 4
		}
		completionTokens = len(resp.Choices[0].Message.Content) / 4
	}

	memory.LogTokenUsage(taskID, promptTokens, completionTokens)

	finalContent := resp.Choices[0].Message.Content
	if reasoning := resp.Choices[0].Message.ReasoningContent; reasoning != "" {
		finalContent = fmt.Sprintf("[[thought]]\n%s\n[[/thought]]\n%s", reasoning, finalContent)
	}

	return finalContent, nil
}
