package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"shachiku/core/config"
	"shachiku/core/memory"
	"shachiku/core/models"
	"shachiku/core/provider"
	"shachiku/core/scheduler"
	"shachiku/core/skills"
)

// AgentAction represents a parsed JSON action from the LLM.
type AgentAction struct {
	Action      string          `json:"action"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Cron        string          `json:"cron"`
	Args        json.RawMessage `json:"args"`
	Command     string          `json:"command"`
	Path        string          `json:"path"`
	Force       bool            `json:"force"`
}

var (
	reToolCall  = regexp.MustCompile(`(?is)<tool_(?:call|use)>(.*?)</tool_(?:call|use)>`)
	tagReplacer = strings.NewReplacer(
		"<tool_call>", "", "</tool_call>", "",
		"<tool_use>", "", "</tool_use>", "",
		"<tool_input>", "", "</tool_input>", "",
	)
)

// fetchMemoryContext retrieves relevant past memories based on the user's message.
func fetchMemoryContext(cfg models.LLMConfig, message string) []string {
	emb, err := provider.GenerateEmbedding(cfg, message)
	if err == nil {
		results, searchErr := memory.SearchMemory(emb, 3)
		if searchErr == nil {
			return results
		}
	}
	return nil
}

// saveFactAsync extracts facts from the given message and saves them as vectors in the background.
func saveFactAsync(cfg models.LLMConfig, message string) {
	fact, err := provider.ExtractFacts(context.Background(), cfg, message)
	if err == nil && fact != "" {
		vec, err := provider.GenerateEmbedding(cfg, fact)
		if err == nil {
			memory.SaveFactToLongTermMemory(fact, vec)
		}
	}
}

// parseAgentReply separates the "thinking process" and the pure JSON action payload.
func parseAgentReply(reply string) (thought string, jsonStr string, finalReply string) {
	jsonStr = reply
	finalReply = reply

	// Extract thought process inside <think> tags.
	if thinkStart := strings.Index(reply, "<think>"); thinkStart != -1 {
		if thinkEnd := strings.Index(reply, "</think>"); thinkEnd != -1 && thinkEnd > thinkStart {
			thought = strings.TrimSpace(reply[thinkStart+7 : thinkEnd])
			reply = strings.TrimSpace(reply[:thinkStart] + "\n" + reply[thinkEnd+8:])
			jsonStr = reply
			finalReply = reply

			// Fallback: if everything was inside <think>, treat the thought as the main content
			if strings.TrimSpace(reply) == "" {
				jsonStr = thought
				finalReply = thought
			}
		} else {
			// Unclosed <think> tag fallback
			thought = strings.TrimSpace(reply[thinkStart+7:])
			reply = strings.TrimSpace(reply[:thinkStart])
			jsonStr = reply
			finalReply = reply

			if strings.TrimSpace(reply) == "" {
				jsonStr = thought
				finalReply = thought
			}
		}
	}

	// Unwrap tool call tags common in open-source models.
	if matches := reToolCall.FindStringSubmatch(jsonStr); len(matches) > 1 {
		jsonStr = matches[1]
	}

	// Clean up partial XML tags efficiently.
	jsonStr = tagReplacer.Replace(jsonStr)

	// Extract JSON block and further thinking text if any.
	if startIdx := strings.Index(jsonStr, "{"); startIdx != -1 {
		if thought == "" && startIdx > 0 {
			thought = strings.TrimSpace(jsonStr[:startIdx])
			if strings.HasPrefix(strings.ToLower(thought), "thinking process:") {
				thought = strings.TrimSpace(thought[17:])
			}
			finalReply = strings.TrimSpace(jsonStr[startIdx:])
		}
		if endIdx := strings.LastIndex(jsonStr, "}"); endIdx != -1 && endIdx > startIdx {
			jsonStr = jsonStr[startIdx : endIdx+1]
		}
	} else if thought == "" && strings.HasPrefix(strings.ToLower(jsonStr), "thinking process:") {
		parts := strings.SplitN(jsonStr, "\n\n", 2)
		if len(parts) == 2 {
			thought = strings.TrimSpace(parts[0][17:])
			jsonStr = parts[1]
			finalReply = jsonStr
		}
	}

	thought = strings.ReplaceAll(thought, "```json", "")
	thought = strings.TrimSpace(thought)

	return thought, jsonStr, finalReply
}

// formatSkillArgs processes edge cases in LLM outputs to generate a clean arguments string.
func formatSkillArgs(actionName string, action *AgentAction) string {
	var argsStr string
	if len(action.Args) > 0 && string(action.Args) != "null" {
		var s string
		if err := json.Unmarshal(action.Args, &s); err == nil {
			argsStr = s
		} else {
			argsStr = string(action.Args)
		}
	}

	if argsStr == "" || argsStr == `""` {
		if action.Command != "" {
			argsStr = action.Command
		} else if action.Path != "" {
			if actionName == "install_skill" {
				argsStr = fmt.Sprintf(`{"path":"%s", "force":%t}`, action.Path, action.Force)
			} else {
				argsStr = action.Path
			}
		}
	}

	if actionName == "install_skill" && action.Path == "" && !strings.HasPrefix(strings.TrimSpace(argsStr), "{") {
		argsStr = fmt.Sprintf(`{"path":"%s", "force":%v}`, argsStr, false)
	}

	return argsStr
}

// executeAgentAction executes the corresponding tool/task business logic.
func executeAgentAction(ctx context.Context, cfg models.LLMConfig, action *AgentAction, ctxHistory []models.Message, onStep func(stepText string)) string {
	actionName := action.Action
	if actionName == "execute_skill" {
		actionName = action.Name
	}

	if onStep != nil {
		onStep(fmt.Sprintf("Executing: %s...", actionName))
	}

	switch actionName {
	case "create_skill":
		if onStep != nil {
			onStep(fmt.Sprintf("Brainstorming skill '%s' structure and logic...", action.Name))
		}
		instructions, _ := provider.GenerateSkillInstructions(ctx, cfg, action.Name, action.Description)
		if err := skills.CreateSkill(action.Name, action.Description, instructions); err == nil {
			return fmt.Sprintf("Skill '%s' created successfully.", action.Name)
		} else {
			return fmt.Sprintf("Failed to create skill '%s': %v", action.Name, err)
		}

	case "execute_task":
		if onStep != nil {
			onStep(fmt.Sprintf("Summarizing task context for '%s'...", action.Name))
		}
		var recentMessages string
		for _, hm := range ctxHistory {
			if hm.Role == "user" || (hm.Role == "agent" && !strings.HasPrefix(hm.Content, "{")) {
				recentMessages += hm.Role + ": " + hm.Content + "\n"
			}
		}
		if len(recentMessages) > 3000 {
			recentMessages = recentMessages[len(recentMessages)-3000:]
		}

		taskMemoryContext := fetchMemoryContext(cfg, action.Description)
		taskPrompt, summarizeErr := provider.SummarizeTaskContext(ctx, cfg, action.Name, action.Description, recentMessages, taskMemoryContext)
		if summarizeErr != nil || taskPrompt == "" {
			taskPrompt = "Task Name: " + action.Name + "\nDescription/Context: " + action.Description
		}

		task, dbErr := memory.CreateTask(action.Name, action.Cron, taskPrompt)
		if dbErr != nil {
			return fmt.Sprintf("Failed to execute task '%s': %v", action.Name, dbErr)
		}

		if action.Cron != "" {
			scheduler.ScheduleTask(*task)
			return fmt.Sprintf("Recurring task '%s' scheduled with cron '%s'.", action.Name, action.Cron)
		}
		scheduler.RunTaskOnce(*task)
		return fmt.Sprintf("Task '%s' is now executing in the background.", action.Name)

	case "list_tasks":
		tasks := memory.GetTasks()
		if len(tasks) == 0 {
			return "There are currently no scheduled or background tasks running."
		}
		var result strings.Builder
		result.WriteString("Here are the current tasks:\n")
		for _, t := range tasks {
			result.WriteString(fmt.Sprintf("- ID: %d | Name: %s | Cron: %s | Status: %s\n", t.ID, t.Name, t.Cron, t.Status))
		}
		return result.String()

	case "delete_task":
		tasks, err := memory.DeleteTasksByName(action.Name)
		if err != nil {
			return fmt.Sprintf("Failed to delete task '%s': %v", action.Name, err)
		}
		for _, t := range tasks {
			scheduler.UnscheduleTask(t.ID)
		}
		return fmt.Sprintf("Successfully deleted and stopped %d task(s) named '%s'.", len(tasks), action.Name)

	case "search_memory":
		var payload struct {
			Query string `json:"query"`
		}
		var query string
		argsStr := formatSkillArgs(actionName, action)
		if err := json.Unmarshal([]byte(argsStr), &payload); err == nil && payload.Query != "" {
			query = payload.Query
		} else {
			query = argsStr // Fallback if they provide just a string
		}

		if query == "" {
			return "Error: missing query argument"
		}

		if onStep != nil {
			onStep(fmt.Sprintf("Searching memory for '%s'...", query))
		}

		emb, err := provider.GenerateEmbedding(cfg, query)
		if err != nil {
			return fmt.Sprintf("Error generating embedding: %v", err)
		}

		results, searchErr := memory.SearchMemory(emb, 10)
		if searchErr != nil || len(results) == 0 {
			return "No relevant memories found."
		}
		return "Recall results:\n- " + strings.Join(results, "\n- ")

	default:
		argsStr := formatSkillArgs(actionName, action)
		return skills.ExecuteSkill(actionName, argsStr)
	}
}

// ProcessMessage runs the automated multi-step LLM reasoning loop.
func ProcessMessage(ctx context.Context, message string, onStep func(stepText string)) (string, error) {
	cfg := memory.GetLLMConfig()
	memoryContext := fetchMemoryContext(cfg, message)

	memory.AddMessage("user", message)
	ctxHistory := memory.GetRecentHistory()

	maxIterations := cfg.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 50
	}

	logsDir := filepath.Join(config.GetDataDir(), "logs")
	os.MkdirAll(logsDir, 0755)
	logFile := filepath.Join(logsDir, fmt.Sprintf("chat_%s.md", time.Now().Format("2006-01-02")))
	f, _ := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	
	availableSkills := skills.ListSkills()
	if f != nil {
		f.WriteString(fmt.Sprintf("\n\n## 🧑 User: %s\n*🕰️ Time: %s*\n\n", message, time.Now().Format(time.RFC3339)))
		
		// Log the initial system prompt to provide full debugging context
		systemPrompt := provider.BuildSystemPrompt(cfg, availableSkills, memoryContext)
		f.WriteString(fmt.Sprintf("### 🛠️ System Prompt\n```md\n%s\n```\n\n", systemPrompt))
	}

	var finalReply string

	for i := 0; i < maxIterations; i++ {
		if ctx.Err() != nil {
			log.Printf("[Agent] Context cancelled")
			if f != nil {
				f.Close()
			}
			return "", ctx.Err()
		}

		reply, err := provider.GenerateResponse(ctx, cfg, ctxHistory, availableSkills, memoryContext, 0)
		if err != nil {
			log.Printf("[Agent] Error from provider: %v", err)
			if f != nil {
				f.WriteString(fmt.Sprintf("**❌ Error from provider:** %v\n", err))
				f.Close()
			}
			return "", err
		}

		log.Printf("[Agent] Raw Response (Iter %d):\n%s\n", i+1, reply)

		if f != nil {
			f.WriteString(fmt.Sprintf("### 🤖 Iteration %d\n**Raw Response:**\n```\n%s\n```\n\n", i+1, reply))
		}

		thought, jsonStr, parsedReply := parseAgentReply(reply)
		finalReply = parsedReply

		if thought != "" {
			if onStep != nil {
				onStep(fmt.Sprintf("Thinking: %s", thought))
			}
			if f != nil {
				f.WriteString(fmt.Sprintf("**💭 Thinking Process:**\n```\n%s\n```\n\n", thought))
			}
		}

		var agentAction AgentAction
		jsonErr := json.Unmarshal([]byte(jsonStr), &agentAction)

		if jsonErr == nil && agentAction.Action != "" {
			executionResult := executeAgentAction(ctx, cfg, &agentAction, ctxHistory, onStep)

			if f != nil {
				f.WriteString(fmt.Sprintf("**⚙️ Tool Execution (%s):**\n```\n%s\n```\n\n", agentAction.Action, executionResult))
			}

			ctxHistory = append(ctxHistory, models.Message{Role: "agent", Content: reply})

			toolResultMsg := fmt.Sprintf("<tool_output name=\"%s\">\n%s\n</tool_output>", agentAction.Action, executionResult)
			ctxHistory = append(ctxHistory, models.Message{Role: "user", Content: toolResultMsg})
		} else if strings.HasPrefix(strings.TrimSpace(jsonStr), "{") && strings.HasSuffix(strings.TrimSpace(jsonStr), "}") {
			ctxHistory = append(ctxHistory, models.Message{Role: "agent", Content: reply})
			ctxHistory = append(ctxHistory, models.Message{Role: "user", Content: fmt.Sprintf("<system_error>\nYour JSON failed to parse. Error: %v. Please make sure to output valid JSON. Do not write raw objects inside strings without escaping, and ensure the JSON is fully closed.\n</system_error>", jsonErr)})
			continue
		} else {
			// Model exited the loop intentionally or didn't output JSON.
			break
		}

		if i == maxIterations-1 {
			tempCtx := append(ctxHistory, models.Message{Role: "user", Content: fmt.Sprintf("<system_warning>\nYou have reached the maximum safety limit for automated steps (%d iterations). Please immediately provide a final conversational summary of your progress to the user. DO NOT output any JSON actions anymore.\n</system_warning>", maxIterations)})
			forcedReply, err := provider.GenerateResponse(ctx, cfg, tempCtx, availableSkills, memoryContext, 0)
			if err == nil && forcedReply != "" {
				finalReply = forcedReply
			} else {
				finalReply = "I have performed numerous steps in the background but reached my safety iteration limit. Please check my previous actions."
			}
		}
	}

	if f != nil {
		f.WriteString(fmt.Sprintf("### ✨ Final Reply\n%s\n\n---\n", finalReply))
		f.Close()
	}

	go saveFactAsync(cfg, message)

	memory.AddMessage("agent", finalReply)
	return finalReply, nil
}
