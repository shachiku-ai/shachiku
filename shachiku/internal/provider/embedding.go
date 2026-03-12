package provider

import (
	"context"
	"fmt"
	"os"
	"time"

	"shachiku/internal/models"

	openai "github.com/sashabaranov/go-openai"
)

// GenerateEmbedding returns an embedding vector for the given text
func GenerateEmbedding(cfg models.LLMConfig, text string) ([]float32, error) {
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
			// Mock a 1536-dimensional embedding
			return make([]float32, 1536), nil
		}
	}

	config := openai.DefaultConfig(apiKey)
	if baseURL := os.Getenv("OPENAI_BASE_URL"); baseURL != "" {
		config.BaseURL = baseURL
	} else if provider == "local" {
		config.BaseURL = "http://localhost:11434/v1"
	}
	client := openai.NewClientWithConfig(config)
	req := openai.EmbeddingRequest{
		Input: []string{text},
		Model: openai.AdaEmbeddingV2,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.CreateEmbeddings(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("provider embedding error: %v", err)
	}

	return resp.Data[0].Embedding, nil
}
