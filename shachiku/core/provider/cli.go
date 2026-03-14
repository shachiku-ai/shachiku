package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"shachiku/core/memory"
	"shachiku/core/models"
)

func generateClaudeCode(ctx context.Context, cfg models.LLMConfig, history []models.Message, systemPrompt string, taskID uint) (string, error) {
	var sb strings.Builder

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
				content = strings.ReplaceAll(content, m[0], "[Attached File: "+path+"]")
			}
			sb.WriteString(fmt.Sprintf("%s:\n%s\n\n", role, content))
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

	configureCmd(cmd)

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
	var includedDirs []string
	var promptFiles []string
	dirMap := make(map[string]bool)
	fileRegex := regexp.MustCompile(`(?m)^@(/.*)$`)

	if systemPrompt != "" {
		sb.WriteString(fmt.Sprintf("System:\n%s\n\n", systemPrompt))
	}

	for _, msg := range history {
		role := "User"
		if msg.Role == "agent" {
			role = "Assistant"
		}
		if msg.Content != "" {
			sb.WriteString(fmt.Sprintf("%s:\n%s\n\n", role, msg.Content))

			matches := fileRegex.FindAllStringSubmatch(msg.Content, -1)
			for _, m := range matches {
				if len(m) > 1 {
					path := strings.TrimSpace(m[1])
					dir := filepath.Dir(path)
					if !dirMap[dir] {
						dirMap[dir] = true
						includedDirs = append(includedDirs, dir)
					}
					promptFiles = append(promptFiles, "@"+path)
				}
			}
		}
	}

	sb.WriteString("Assistant:\n")

	reqCtx, cancel := context.WithTimeout(ctx, 600*time.Second)
	defer cancel()

	args := []string{"-p", strings.Join(promptFiles, " ")}
	for _, dir := range includedDirs {
		args = append(args, "--include-directories", dir)
	}

	cmd := exec.CommandContext(reqCtx, "gemini", args...)
	cmd.Stdin = strings.NewReader(sb.String())

	configureCmd(cmd)

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
	var imageArgs []string
	var dirs []string
	dirMap := make(map[string]bool)
	fileRegex := regexp.MustCompile(`(?m)^@(/.*)$`)

	// systemPrompt is passed via -c command line argument below

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
				dir := filepath.Dir(path)
				if !dirMap[dir] {
					dirMap[dir] = true
					dirs = append(dirs, dir)
				}
				ext := strings.ToLower(filepath.Ext(path))
				if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif" || ext == ".webp" {
					imageArgs = append(imageArgs, "-i", path)
				}
				content = strings.ReplaceAll(content, m[0], "[Attached File: "+path+"]")
			}
			sb.WriteString(fmt.Sprintf("%s:\n%s\n\n", role, content))
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
	args = append(args, "--skip-git-repo-check")
	for _, dir := range dirs {
		args = append(args, "--add-dir", dir)
	}
	args = append(args, imageArgs...)
	args = append(args, "-")

	cmd := exec.CommandContext(reqCtx, "codex", args...)
	cmd.Stdin = strings.NewReader(sb.String())

	configureCmd(cmd)

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
