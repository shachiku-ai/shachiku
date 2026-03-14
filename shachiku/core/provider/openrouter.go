package provider

import (
	"context"
	"fmt"
	"os"
	"time"

	"shachiku/core/memory"
	"shachiku/core/models"

	openai "github.com/sashabaranov/go-openai"
)

func generateOpenRouter(ctx context.Context, cfg models.LLMConfig, history []models.Message, systemPrompt string, taskID uint) (string, error) {
	apiKey := cfg.OpenRouterAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENROUTER_API_KEY")
	}
	if apiKey == "" {
		return "Mock OpenRouter response. Provide OPENROUTER_API_KEY to see real generations.\nContext loaded.", nil
	}

	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://openrouter.ai/api/v1"

	// OpenRouter recommends these headers for rankings and visibility, though not strictly required
	if url := os.Getenv("OPENROUTER_SITE_URL"); url != "" {
		// Custom header injection not easily supported with default go-openai client
		// without a custom HTTP client/RoundTripper. OpenRouter works fine without them.
	}

	client := openai.NewClientWithConfig(config)

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
	}

	for _, msg := range history {
		role := openai.ChatMessageRoleUser
		if msg.Role == "agent" {
			role = openai.ChatMessageRoleAssistant
		}
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    role,
			Content: msg.Content,
		})
	}

	model := cfg.Model
	if model == "" {
		model = "google/gemini-2.5-pro" // OpenRouter default if not specified
	}

	reqCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	resp, err := client.CreateChatCompletion(
		reqCtx,
		openai.ChatCompletionRequest{
			Model:    model,
			Messages: messages,
		},
	)

	if err != nil {
		return "", fmt.Errorf("openrouter error: %v", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openrouter error: successfully connected but no choices returned in the response (possibly blocked or filtered)")
	}

	promptTokens := resp.Usage.PromptTokens
	completionTokens := resp.Usage.CompletionTokens
	if promptTokens == 0 && completionTokens == 0 {
		promptTokens = len(systemPrompt) / 4
		for _, m := range history {
			promptTokens += len(m.Content) / 4
		}
		completionTokens = len(resp.Choices[0].Message.Content) / 4
	}

	memory.LogTokenUsage(taskID, promptTokens, completionTokens)
	return resp.Choices[0].Message.Content, nil
}
