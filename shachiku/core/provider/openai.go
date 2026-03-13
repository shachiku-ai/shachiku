package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"shachiku/core/memory"
	"shachiku/core/models"

	openai "github.com/sashabaranov/go-openai"
)

func generateOpenAI(ctx context.Context, cfg models.LLMConfig, history []models.Message, systemPrompt string, taskID uint) (string, error) {
	provider := cfg.Provider
	if provider == "" {
		provider = os.Getenv("LLM_PROVIDER")
	}

	apiKey := cfg.OpenAIAPIKey
	if provider == "local" {
		if apiKey == "" {
			apiKey = "dummy"
		}
	} else {
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		if apiKey == "" {
			return "Mock OpenAI response. Provide OPENAI_API_KEY to see real generations.\nContext loaded.", nil
		}
	}

	config := openai.DefaultConfig(apiKey)
	if baseURL := os.Getenv("OPENAI_BASE_URL"); baseURL != "" {
		config.BaseURL = baseURL
	} else if provider == "local" {
		config.BaseURL = "http://localhost:11434/v1"
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
		cleanText, imagePaths := extractImagesAndText(msg.Content)
		if len(imagePaths) > 0 {
			var multi []openai.ChatMessagePart
			if cleanText != "" {
				multi = append(multi, openai.ChatMessagePart{
					Type: openai.ChatMessagePartTypeText,
					Text: cleanText,
				})
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
					url := fmt.Sprintf("data:%s;base64,%s", mime, encoded)
					multi = append(multi, openai.ChatMessagePart{
						Type: openai.ChatMessagePartTypeImageURL,
						ImageURL: &openai.ChatMessageImageURL{
							URL: url,
						},
					})
				}
			}
			messages = append(messages, openai.ChatCompletionMessage{
				Role:         role,
				MultiContent: multi,
			})
		} else {
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    role,
				Content: msg.Content,
			})
		}
	}

	model := cfg.Model
	if model == "" {
		model = openai.GPT4o
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
		return "", fmt.Errorf("openai error: %v", err)
	}

	promptTokens := resp.Usage.PromptTokens
	completionTokens := resp.Usage.CompletionTokens
	if promptTokens == 0 && completionTokens == 0 {
		// Fallback for local providers that don't return usage
		promptTokens = len(systemPrompt) / 4
		for _, m := range history {
			promptTokens += len(m.Content) / 4
		}
		completionTokens = len(resp.Choices[0].Message.Content) / 4
	}

	memory.LogTokenUsage(taskID, promptTokens, completionTokens)
	return resp.Choices[0].Message.Content, nil
}
