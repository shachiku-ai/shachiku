package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"shachiku/internal/memory"
	"shachiku/internal/models"
	"shachiku/internal/provider"
	"shachiku/internal/scheduler"
	"shachiku/internal/skills"
)

// ProcessMessage runs the full multi-step LLM reasoning loop to handle a user message.
func ProcessMessage(ctx context.Context, message string, onStep func(stepText string)) (string, error) {
	cfg := memory.GetLLMConfig()
	emb, err := provider.GenerateEmbedding(cfg, message)
	var memoryContext []string
	if err == nil {
		results, searchErr := memory.SearchMemory(emb, 3)
		if searchErr == nil {
			memoryContext = results
		} else {
			_ = searchErr
		}
	}

	memory.AddMessage("user", message)
	ctxHistory := memory.GetRecentHistory()

	maxIterations := cfg.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 50
	}
	var finalReply string
	availableSkills := skills.ListSkills()

	for i := 0; i < maxIterations; i++ {
		if ctx.Err() != nil {
			log.Printf("[Agent] Context cancelled")
			return "", ctx.Err()
		}

		reply, err := provider.GenerateResponse(ctx, cfg, ctxHistory, availableSkills, memoryContext, 0)
		if err != nil {
			log.Printf("[Agent] Error from provider: %v", err)
			return "", err
		}

		log.Printf("[Agent] Raw Response (Iter %d):\n%s\n", i+1, reply)
		finalReply = reply

		var agentAction struct {
			Action      string          `json:"action"`
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Cron        string          `json:"cron"`
			Args        json.RawMessage `json:"args"`
			Command     string          `json:"command"`
			Path        string          `json:"path"`
			Force       bool            `json:"force"`
		}

		jsonStr := reply
		var thought string

		if thinkStart := strings.Index(reply, "<think>"); thinkStart != -1 {
			if thinkEnd := strings.Index(reply, "</think>"); thinkEnd != -1 && thinkEnd > thinkStart {
				thought = strings.TrimSpace(reply[thinkStart+7 : thinkEnd])
				reply = strings.TrimSpace(reply[:thinkStart] + "\n" + reply[thinkEnd+8:])
				jsonStr = reply
				finalReply = reply
			}
		}

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

		if thought != "" {
			thought = strings.ReplaceAll(thought, "```json", "")
			thought = strings.TrimSpace(thought)
			if thought != "" && onStep != nil {
				onStep(fmt.Sprintf("Thinking: %s", thought))
			}
		}

		if jsonErr := json.Unmarshal([]byte(jsonStr), &agentAction); jsonErr == nil && agentAction.Action != "" {
			var executionResult string
			actionName := agentAction.Action

			if actionName == "execute_skill" {
				actionName = agentAction.Name
			}

			if onStep != nil {
				onStep(fmt.Sprintf("Executing: %s...", actionName))
			}

			switch actionName {
			case "create_skill":
				if onStep != nil {
					onStep(fmt.Sprintf("Brainstorming skill '%s' structure and logic...", agentAction.Name))
				}
				instructions, _ := provider.GenerateSkillInstructions(ctx, cfg, agentAction.Name, agentAction.Description)
				err := skills.CreateSkill(agentAction.Name, agentAction.Description, instructions)
				if err == nil {
					executionResult = fmt.Sprintf("Skill '%s' created successfully.", agentAction.Name)
				} else {
					executionResult = fmt.Sprintf("Failed to create skill '%s': %v", agentAction.Name, err)
				}
			case "execute_task":
				if onStep != nil {
					onStep(fmt.Sprintf("Summarizing task context for '%s'...", agentAction.Name))
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

				var taskMemoryContext []string
				taskEmb, taskEmbErr := provider.GenerateEmbedding(cfg, agentAction.Description)
				if taskEmbErr == nil {
					taskResults, taskSearchErr := memory.SearchMemory(taskEmb, 3)
					if taskSearchErr == nil {
						taskMemoryContext = taskResults
					}
				}

				taskPrompt, summarizeErr := provider.SummarizeTaskContext(ctx, cfg, agentAction.Name, agentAction.Description, recentMessages, taskMemoryContext)
				if summarizeErr != nil || taskPrompt == "" {
					taskPrompt = "Task Name: " + agentAction.Name + "\nDescription/Context: " + agentAction.Description
				}

				task, dbErr := memory.CreateTask(agentAction.Name, agentAction.Cron, taskPrompt)
				if dbErr == nil {
					if agentAction.Cron != "" {
						scheduler.ScheduleTask(*task)
						executionResult = fmt.Sprintf("Recurring task '%s' scheduled with cron '%s'.", agentAction.Name, agentAction.Cron)
					} else {
						scheduler.RunTaskOnce(*task)
						executionResult = fmt.Sprintf("Task '%s' is now executing in the background.", agentAction.Name)
					}
				} else {
					executionResult = fmt.Sprintf("Failed to execute task '%s': %v", agentAction.Name, dbErr)
				}
			case "list_tasks":
				tasks := memory.GetTasks()
				if len(tasks) == 0 {
					executionResult = "There are currently no scheduled or background tasks running."
				} else {
					executionResult = "Here are the current tasks:\n"
					for _, t := range tasks {
						executionResult += fmt.Sprintf("- ID: %d | Name: %s | Cron: %s | Status: %s\n", t.ID, t.Name, t.Cron, t.Status)
					}
				}
			case "delete_task":
				tasks, err := memory.DeleteTasksByName(agentAction.Name)
				if err != nil {
					executionResult = fmt.Sprintf("Failed to delete task '%s': %v", agentAction.Name, err)
				} else {
					for _, t := range tasks {
						scheduler.UnscheduleTask(t.ID)
					}
					executionResult = fmt.Sprintf("Successfully deleted and stopped %d task(s) named '%s'.", len(tasks), agentAction.Name)
				}
			default:
				var argsStr string
				if len(agentAction.Args) > 0 && string(agentAction.Args) != "null" {
					var s string
					if err := json.Unmarshal(agentAction.Args, &s); err == nil {
						argsStr = s
					} else {
						argsStr = string(agentAction.Args)
					}
				}

				if argsStr == "" || argsStr == `""` {
					if agentAction.Command != "" {
						argsStr = agentAction.Command
					} else if agentAction.Path != "" {
						if actionName == "install_skill" {
							argsStr = fmt.Sprintf(`{"path":"%s", "force":%t}`, agentAction.Path, agentAction.Force)
						} else {
							argsStr = agentAction.Path
						}
					}
				}

				if actionName == "install_skill" && agentAction.Path == "" && !strings.HasPrefix(strings.TrimSpace(argsStr), "{") {
					argsStr = fmt.Sprintf(`{"path":"%s", "force":%v}`, argsStr, false)
				}

				skillResult := skills.ExecuteSkill(actionName, argsStr)
				executionResult = skillResult
			}

			ctxHistory = append(ctxHistory, models.Message{Role: "agent", Content: reply})

			systemPromptPrompt := fmt.Sprintf("You just executed the action/skill '%s'.\nThe result was:\n--------\n%s\n--------\nAnalyze the result. If you need to perform MORE actions to accomplish the user's goal, output the NEXT JSON action. If the task is fully complete, provide the final response to the user in a natural, conversational way without outputting any JSON. IMPORTANT: The final response MUST explicitly contain the exact details, prices, or data found in the result. DO NOT just summarize; provide the actual data. Do NOT include any 'Thought process:' or internal thinking in your final response—just directly address the user. Ensure your final response tone, language, and personality strictly match any preferences or personality traits found in your long-term memory context.", actionName, executionResult)
			ctxHistory = append(ctxHistory, models.Message{Role: "user", Content: systemPromptPrompt})

		} else if strings.HasPrefix(strings.TrimSpace(jsonStr), "{") && strings.HasSuffix(strings.TrimSpace(jsonStr), "}") {
			ctxHistory = append(ctxHistory, models.Message{Role: "agent", Content: reply})
			ctxHistory = append(ctxHistory, models.Message{Role: "user", Content: fmt.Sprintf("System: Your JSON failed to parse. Error: %v. Please make sure to output valid JSON. Do not write raw objects inside strings without escaping, and ensure the JSON is fully closed.", jsonErr)})
			continue
		} else {
			break
		}

		if i == maxIterations-1 {
			tempCtx := append(ctxHistory, models.Message{Role: "user", Content: fmt.Sprintf("System: You have reached the maximum safety limit for automated steps (%d iterations). Please immediately provide a final conversational summary of your progress to the user. DO NOT output any JSON actions anymore.", maxIterations)})
			forcedReply, err := provider.GenerateResponse(ctx, cfg, tempCtx, availableSkills, memoryContext, 0)
			if err == nil && forcedReply != "" {
				finalReply = forcedReply
			} else {
				finalReply = "I have performed numerous steps in the background but reached my safety iteration limit. Please check my previous actions."
			}
		}
	}

	go func(msg string, lcfg models.LLMConfig) {
		fact, err := provider.ExtractFacts(context.Background(), lcfg, msg)
		if err == nil && fact != "" {
			vec, err := provider.GenerateEmbedding(lcfg, fact)
			if err == nil {
				memory.SaveFactToLongTermMemory(fact, vec)
			}
		}
	}(message, cfg)

	memory.AddMessage("agent", finalReply)
	return finalReply, nil
}
