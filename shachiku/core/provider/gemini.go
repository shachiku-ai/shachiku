package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

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

	fileRegex := regexp.MustCompile(`(?m)^@(/.*)$`)

	buildParts := func(msgText string) []genai.Part {
		content := msgText
		matches := fileRegex.FindAllStringSubmatch(content, -1)
		var parts []genai.Part

		for _, m := range matches {
			path := strings.TrimSpace(m[1])
			content = strings.ReplaceAll(content, m[0], "")

			data, err := os.ReadFile(path)
			if err != nil {
				continue // skip on error
			}

			ext := strings.ToLower(filepath.Ext(path))
			mimeType := ""
			switch ext {
			case ".png":
				mimeType = "image/png"
			case ".jpeg", ".jpg":
				mimeType = "image/jpeg"
			case ".webp":
				mimeType = "image/webp"
			case ".heic":
				mimeType = "image/heic"
			case ".heif":
				mimeType = "image/heif"
			case ".pdf":
				mimeType = "application/pdf"
			case ".mp3":
				mimeType = "audio/mp3"
			case ".ogg":
				mimeType = "audio/ogg"
			case ".wav":
				mimeType = "audio/wav"
			case ".aac":
				mimeType = "audio/aac"
			case ".flac":
				mimeType = "audio/flac"
			case ".mp4":
				mimeType = "video/mp4"
			case ".mpeg":
				mimeType = "video/mpeg"
			case ".mov":
				mimeType = "video/quicktime"
			case ".webm":
				mimeType = "video/webm"
			default:
				// Fallback to DetectContentType for images
				detected := http.DetectContentType(data)
				if strings.HasPrefix(detected, "image/") {
					mimeType = detected
				}
			}

			if mimeType != "" {
				parts = append(parts, genai.Blob{
					MIMEType: mimeType,
					Data:     data,
				})
			} else if utf8.Valid(data) {
				parts = append(parts, genai.Text(fmt.Sprintf("\n\n[Attached File: %s]\n%s\n", path, string(data))))
			} else {
				parts = append(parts, genai.Text(fmt.Sprintf("\n\n[Attached File: %s] (binary file omitted, no direct support. Use bash/python tools to read if needed.)\n", path)))
			}
		}

		content = strings.TrimSpace(content)
		if content != "" {
			parts = append(parts, genai.Text(content))
		}
		if len(parts) == 0 {
			parts = append(parts, genai.Text(" "))
		}
		return parts
	}

	cs := model.StartChat()
	for i := 0; i < len(history)-1; i++ {
		msg := history[i]
		role := "user"
		if msg.Role == "agent" {
			role = "model"
		}

		parts := buildParts(msg.Content)

		cs.History = append(cs.History, &genai.Content{
			Role:  role,
			Parts: parts,
		})
	}

	var lastMsg string
	if len(history) > 0 {
		lastMsg = history[len(history)-1].Content
	}

	lastParts := buildParts(lastMsg)

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
