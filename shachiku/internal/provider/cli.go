package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"shachiku/internal/memory"
	"shachiku/internal/models"
)

func generateClaudeCode(ctx context.Context, cfg models.LLMConfig, history []models.Message, systemPrompt string, taskID uint) (string, error) {
	var sb strings.Builder

	for _, msg := range history {
		role := "User"
		if msg.Role == "agent" {
			role = "Assistant"
		}
		cleanText, _ := extractImagesAndText(msg.Content)
		if cleanText != "" {
			sb.WriteString(fmt.Sprintf("%s:\n%s\n\n", role, cleanText))
		}
	}

	sb.WriteString("Assistant:\n")

	reqCtx, cancel := context.WithTimeout(ctx, 600*time.Second)
	defer cancel()

	var args []string
	if runtime.GOOS == "windows" {
		// To circumvent cmd.exe escaping issues (quotes, <, >) and length limits on Windows,
		// inject the prompt via stdin instead of CLI arguments.
		sbStr := sb.String()
		sb.Reset()
		if systemPrompt != "" {
			sb.WriteString(fmt.Sprintf("<system_instructions>\n%s\n</system_instructions>\n\n", systemPrompt))
		}
		sb.WriteString(sbStr)

		args = []string{"-y", "@anthropic-ai/claude-code", "-p", "--tools", "", "--output-format", "json", "--no-session-persistence"}
	} else {
		args = []string{"-y", "@anthropic-ai/claude-code", "-p", "--tools", "", "--output-format", "json", "--no-session-persistence", "--system-prompt", systemPrompt}
	}

	cmd := exec.CommandContext(reqCtx, "npx", args...)
	cmd.Stdin = strings.NewReader(sb.String())

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("claudecode error: %v, stderr: %s, stdout: %s", err, errBuf.String(), outBuf.String())
	}

	// Parse the JSON output format from Claude Code
	var result struct {
		Result string `json:"result"`
	}
	if parseErr := json.Unmarshal(outBuf.Bytes(), &result); parseErr != nil {
		// fallback to raw output if unmarshal fails
		outStr := strings.TrimSpace(outBuf.String())
		memory.LogTokenUsage(taskID, len(sb.String())/4, len(outStr)/4)
		return outStr, nil
	}

	memory.LogTokenUsage(taskID, len(sb.String())/4, len(result.Result)/4)
	return strings.TrimSpace(result.Result), nil
}

func generateGeminiCLI(ctx context.Context, cfg models.LLMConfig, history []models.Message, systemPrompt string, taskID uint) (string, error) {
	var sb strings.Builder

	if systemPrompt != "" {
		sb.WriteString(fmt.Sprintf("System:\n%s\n\n", systemPrompt))
	}

	for _, msg := range history {
		role := "User"
		if msg.Role == "agent" {
			role = "Assistant"
		}
		cleanText, _ := extractImagesAndText(msg.Content)
		if cleanText != "" {
			sb.WriteString(fmt.Sprintf("%s:\n%s\n\n", role, cleanText))
		}
	}

	sb.WriteString("Assistant:\n")

	reqCtx, cancel := context.WithTimeout(ctx, 600*time.Second)
	defer cancel()

	cmd := exec.CommandContext(reqCtx, "gemini", "-p", "")
	cmd.Stdin = strings.NewReader(sb.String())

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("gemini cli error: %v, stderr: %s, stdout: %s", err, errBuf.String(), outBuf.String())
	}

	outStr := strings.TrimSpace(outBuf.String())
	memory.LogTokenUsage(taskID, len(sb.String())/4, len(outStr)/4)
	return outStr, nil
}

func generateCodexCLI(ctx context.Context, cfg models.LLMConfig, history []models.Message, systemPrompt string, taskID uint) (string, error) {
	var sb strings.Builder

	// systemPrompt is passed via -c command line argument below

	for _, msg := range history {
		role := "User"
		if msg.Role == "agent" {
			role = "Assistant"
		}
		cleanText, _ := extractImagesAndText(msg.Content)
		if cleanText != "" {
			sb.WriteString(fmt.Sprintf("%s:\n%s\n\n", role, cleanText))
		}
	}

	sb.WriteString("Assistant:\n")

	reqCtx, cancel := context.WithTimeout(ctx, 600*time.Second)
	defer cancel()

	var args []string
	args = append(args, "exec")
	if systemPrompt != "" {
		args = append(args, "-c", fmt.Sprintf("developer_instructions=%q", systemPrompt))
	}
	args = append(args, "--skip-git-repo-check", "-")

	cmd := exec.CommandContext(reqCtx, "codex", args...)
	cmd.Stdin = strings.NewReader(sb.String())

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("codex error: %v, stderr: %s, stdout: %s", err, errBuf.String(), outBuf.String())
	}

	outStr := strings.TrimSpace(outBuf.String())
	memory.LogTokenUsage(taskID, len(sb.String())/4, len(outStr)/4)
	return outStr, nil
}
