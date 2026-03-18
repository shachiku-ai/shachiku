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

// sanitizeUTF8 strips any invalid UTF-8 bytes from the input string,
// preventing protobuf serialization errors in the Gemini API.
func sanitizeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return strings.ToValidUTF8(s, "")
}

func generateGemini(ctx context.Context, cfg models.LLMConfig, history []models.Message, systemPrompt string, taskID uint) (string, error) {
	apiKeyStr := cfg.GeminiAPIKey
	if apiKeyStr == "" {
		apiKeyStr = os.Getenv("GEMINI_API_KEY")
	}
	if apiKeyStr == "" {
		return "Mock Gemini response. Provide GEMINI_API_KEY to see real generations.", nil
	}

	apiKeys := strings.Split(apiKeyStr, ",")
	var validKeys []string
	for _, k := range apiKeys {
		k = strings.TrimSpace(k)
		if k != "" {
			validKeys = append(validKeys, k)
		}
	}
	if len(validKeys) == 0 {
		return "Mock Gemini response. Provide GEMINI_API_KEY to see real generations.", nil
	}

	modelID := cfg.Model
	if modelID == "" {
		modelID = "gemini-2.5-flash"
	}

	fileRegex := regexp.MustCompile(`(?m)^@(/.*)$`)

	buildParts := func(msgText string) []genai.Part {
		content := sanitizeUTF8(msgText)
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

	var lastErr error
	for _, apiKey := range validKeys {
		client, err := genai.NewClient(ctx, googleoption.WithAPIKey(apiKey))
		if err != nil {
			lastErr = fmt.Errorf("gemini client error: %v", err)
			continue
		}

		model := client.GenerativeModel(modelID)
		model.SystemInstruction = &genai.Content{
			Parts: []genai.Part{genai.Text(sanitizeUTF8(systemPrompt))},
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

		reqMap := map[string]interface{}{
			"Model":             modelID,
			"SystemInstruction": systemPrompt,
			"History":           cs.History,
			"LastParts":         lastParts,
		}
		reqJSON, _ := json.MarshalIndent(reqMap, "", "  ")

		keyPreview := apiKey
		if len(keyPreview) > 4 {
			keyPreview = keyPreview[len(keyPreview)-4:]
		}
		fmt.Printf("=== [Gemini API Request (Key ending in ...%s)] ===\n%s\n============================\n", keyPreview, string(reqJSON))

		resp, err := cs.SendMessage(ctxReq, lastParts...)
		cancel()

		if err != nil {
			client.Close()
			lastErr = fmt.Errorf("gemini api error: %v", err)
			
			errStr := strings.ToLower(err.Error())
			if strings.Contains(errStr, "429") || strings.Contains(errStr, "quota") || strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "exhausted") {
				fmt.Printf("[Provider] Gemini API key ...%s exhausted or rate-limited, switching to next key...\n", keyPreview)
				continue
			}
			return "", lastErr
		}

		if resp.UsageMetadata != nil {
			memory.LogTokenUsage(taskID, int(resp.UsageMetadata.PromptTokenCount), int(resp.UsageMetadata.CandidatesTokenCount))
		}
		
		client.Close()

		if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
			if txt, ok := resp.Candidates[0].Content.Parts[0].(genai.Text); ok {
				return string(txt), nil
			}
		}
		return "", fmt.Errorf("empty gemini response")
	}

	return "", lastErr
}
