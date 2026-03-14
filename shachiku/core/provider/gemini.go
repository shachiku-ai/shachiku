package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"shachiku/core/memory"
	"shachiku/core/models"

	"github.com/google/generative-ai-go/genai"
	googleoption "google.golang.org/api/option"
)

func generateGemini(ctx context.Context, cfg models.LLMConfig, history []models.Message, systemPrompt string, taskID uint) (string, error) {
	apiKey := cfg.GeminiAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}
	if apiKey == "" {
		return "Mock Gemini response. Provide GEMINI_API_KEY to see real generations.", nil
	}

	client, err := genai.NewClient(ctx, googleoption.WithAPIKey(apiKey))
	if err != nil {
		return "", fmt.Errorf("gemini client error: %v", err)
	}
	defer client.Close()

	modelID := cfg.Model
	if modelID == "" {
		modelID = "gemini-2.5-flash"
	}

	model := client.GenerativeModel(modelID)
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(systemPrompt)},
	}

	cs := model.StartChat()
	for i := 0; i < len(history)-1; i++ {
		msg := history[i]
		role := "user"
		if msg.Role == "agent" {
			role = "model"
		}

		var parts []genai.Part
		if msg.Content != "" {
			parts = append(parts, genai.Text(msg.Content))
		} else {
			parts = append(parts, genai.Text(" "))
		}

		cs.History = append(cs.History, &genai.Content{
			Role:  role,
			Parts: parts,
		})
	}

	var lastMsg string
	if len(history) > 0 {
		lastMsg = history[len(history)-1].Content
	}

	var lastParts []genai.Part
	if lastMsg != "" {
		lastParts = append(lastParts, genai.Text(lastMsg))
	} else {
		lastParts = append(lastParts, genai.Text(" "))
	}

	ctxReq, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	reqMap := map[string]interface{}{
		"Model":             modelID,
		"SystemInstruction": systemPrompt,
		"History":           cs.History,
		"LastParts":         lastParts,
	}
	reqJSON, _ := json.MarshalIndent(reqMap, "", "  ")
	fmt.Printf("=== [Gemini API Request] ===\n%s\n============================\n", string(reqJSON))

	resp, err := cs.SendMessage(ctxReq, lastParts...)
	if err != nil {
		return "", fmt.Errorf("gemini api error: %v", err)
	}

	if resp.UsageMetadata != nil {
		memory.LogTokenUsage(taskID, int(resp.UsageMetadata.PromptTokenCount), int(resp.UsageMetadata.CandidatesTokenCount))
	}

	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		if txt, ok := resp.Candidates[0].Content.Parts[0].(genai.Text); ok {
			return string(txt), nil
		}
	}
	return "", fmt.Errorf("empty gemini response")
}
