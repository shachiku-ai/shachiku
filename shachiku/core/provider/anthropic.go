package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

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

	messages := []anthropic.MessageParam{}
	for _, msg := range history {
		var blocks []anthropic.ContentBlockParamUnion
		if msg.Content != "" {
			blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
		} else {
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
