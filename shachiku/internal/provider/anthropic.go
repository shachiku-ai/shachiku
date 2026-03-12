package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"shachiku/internal/memory"
	"shachiku/internal/models"

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

	messages := []anthropic.MessageParam{}
	for _, msg := range history {
		cleanText, imagePaths := extractImagesAndText(msg.Content)
		var blocks []anthropic.ContentBlockParamUnion
		if cleanText != "" {
			blocks = append(blocks, anthropic.NewTextBlock(cleanText))
		}
		for _, imgPath := range imagePaths {
			data, err := os.ReadFile(imgPath)
			if err == nil {
				encoded := base64.StdEncoding.EncodeToString(data)
				mime := "image/jpeg"
				if strings.HasSuffix(strings.ToLower(imgPath), ".png") {
					mime = "image/png"
				} else if strings.HasSuffix(strings.ToLower(imgPath), ".gif") {
					mime = "image/gif"
				} else if strings.HasSuffix(strings.ToLower(imgPath), ".webp") {
					mime = "image/webp"
				}
				blocks = append(blocks, anthropic.NewImageBlockBase64(mime, encoded))
			}
		}
		if len(blocks) == 0 {
			// fallback if empty
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

	resp, err := client.Messages.New(reqCtx, anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: 8192,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
		Messages:  messages,
	})

	if err != nil {
		return "", fmt.Errorf("anthropic error: %v", err)
	}

	memory.LogTokenUsage(taskID, int(resp.Usage.InputTokens), int(resp.Usage.OutputTokens))

	if len(resp.Content) > 0 {
		return resp.Content[0].Text, nil
	}
	return "", fmt.Errorf("empty claude response")
}
