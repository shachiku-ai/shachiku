package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"shachiku/core/memory"
	"shachiku/core/models"

	bridge "github.com/shachiku-ai/shachiku-cli-bridge"
)

func runCLIBridge(ctx context.Context, p bridge.Provider, history []models.Message, systemPrompt string, taskID uint) (string, error) {
	var sb strings.Builder
	var images []string

	fileRegex := regexp.MustCompile(`(?m)^@(/.*)$`)

	for _, msg := range history {
		role := "User"
		if msg.Role == "agent" {
			role = "Assistant"
		}
		if msg.Content != "" {
			content := msg.Content
			matches := fileRegex.FindAllStringSubmatch(content, -1)
			for _, m := range matches {
				path := strings.TrimSpace(m[1])
				images = append(images, path)
				content = strings.ReplaceAll(content, m[0], "[Attached File: "+path+"]")
			}
			sb.WriteString(fmt.Sprintf("%s:\n%s\n\n", role, content))
		}
	}

	sb.WriteString("Assistant:\n")

	b := bridge.NewBridge()

	req := &bridge.Request{
		Provider:     p,
		SystemPrompt: systemPrompt,
		Messages: []bridge.Message{
			{
				Role:    "user",
				Content: sb.String(),
			},
		},
		Files: images,
	}

	outStr, err := b.Execute(ctx, req)
	if err != nil {
		return "", fmt.Errorf("bridge error (%s): %v. output: %s", p, err, outStr)
	}

	outStr = strings.TrimSpace(outStr)
	memory.LogTokenUsage(taskID, len(sb.String())/4, len(outStr)/4)

	return outStr, nil
}

func generateClaudeCode(ctx context.Context, cfg models.LLMConfig, history []models.Message, systemPrompt string, taskID uint) (string, error) {
	return runCLIBridge(ctx, bridge.ProviderClaude, history, systemPrompt, taskID)
}

func generateGeminiCLI(ctx context.Context, cfg models.LLMConfig, history []models.Message, systemPrompt string, taskID uint) (string, error) {
	return runCLIBridge(ctx, bridge.ProviderGemini, history, systemPrompt, taskID)
}

func generateCodexCLI(ctx context.Context, cfg models.LLMConfig, history []models.Message, systemPrompt string, taskID uint) (string, error) {
	return runCLIBridge(ctx, bridge.ProviderCodex, history, systemPrompt, taskID)
}
