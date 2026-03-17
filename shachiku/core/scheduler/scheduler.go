package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"shachiku/core/memory"
	"shachiku/core/models"
	"shachiku/core/provider"
	"shachiku/core/skills"
)

var (
	c           *cron.Cron
	taskEntries = make(map[uint]cron.EntryID)
	onceTimers  = make(map[uint]*time.Timer)
	mu          sync.Mutex

	NotificationCallback func(msg string)
)

// Init starts the cron scheduler
func Init() {
	c = cron.New()

	// Load existing tasks and schedule them
	tasks := memory.GetTasks()
	for _, t := range tasks {
		if t.Cron != "" {
			cronStr := strings.TrimSpace(t.Cron)
			if strings.HasPrefix(cronStr, "delay:") || strings.HasPrefix(cronStr, "at:") || strings.HasPrefix(cronStr, "@delay ") || strings.HasPrefix(cronStr, "@at ") {
				if t.Status == "completed" {
					continue
				}
			}
			ScheduleTask(t)
		}
	}

	c.Start()
	log.Println("Scheduler initialized.")
}

func executeTaskWithLoop(task models.Task) {
	log.Printf("[Scheduler] Executing task %d: %s\n", task.ID, task.Name)
	cfg := memory.GetLLMConfig()
	availableSkills := skills.ListSkills()

	// Initial user query to start the task
	msg := fmt.Sprintf("<task_execution_request>\n<task_name>%s</task_name>\n<task_context_and_history_prompt>\n%s\n</task_context_and_history_prompt>\n</task_execution_request>", task.Name, task.Prompt)

	// Fetch last execution log for this recurring task
	pastLogs := memory.GetTaskLogs(task.ID)
	if len(pastLogs) > 0 {
		lastLog := pastLogs[len(pastLogs)-1]
		msg += fmt.Sprintf("\n\n<previous_execution_result>\nThis task is recurring. Here is the final report/result from the LAST time this task ran (at %s):\n--------\n%s\n--------\n</previous_execution_result>", lastLog.CreatedAt.Format(time.RFC3339), lastLog.Output)
	}

	// Create a temporary history just for this task run
	ctxHistory := []models.Message{
		{Role: "user", Content: msg},
	}

	// Retrieve long-term memory for task
	var memoryContext []string
	emb, err := provider.GenerateEmbedding(cfg, task.Prompt)
	if err == nil {
		results, searchErr := memory.SearchMemory(emb, 3)
		if searchErr == nil {
			memoryContext = results
		} else {
			log.Printf("[Scheduler] Error searching memory: %v", searchErr)
		}
	} else {
		log.Printf("[Scheduler] Error generating embedding for memory search: %v", err)
	}

	// Retrieve memory specifically for user preferences (language, style)
	prefEmb, err := provider.GenerateEmbedding(cfg, "User's language preference, tone, personality, and communication style")
	if err == nil {
		prefResults, prefSearchErr := memory.SearchMemory(prefEmb, 2)
		if prefSearchErr == nil {
			memoryContext = append(memoryContext, prefResults...)
		}
	}

	maxIterations := cfg.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 50
	}
	var finalReply string
	isSuccess := true

	for i := 0; i < maxIterations; i++ {
		reply, runtimeErr := provider.GenerateResponse(context.Background(), cfg, ctxHistory, availableSkills, memoryContext, task.ID)
		if runtimeErr != nil {
			log.Printf("[Scheduler] Error executing task: %v\n", runtimeErr)
			errMsg := "Error: " + runtimeErr.Error()
			memory.CreateTaskLog(task.ID, errMsg)
			memory.UpdateTaskStatus(task.ID, "error")

			// Notify user in chat
			reportMsg := fmt.Sprintf("❌ **Background Task Error: %s** (ID: %d)\n\n%s", task.Name, task.ID, errMsg)
			memory.AddMessage("system_notification", reportMsg)
			if NotificationCallback != nil {
				NotificationCallback(reportMsg)
			}
			return
		}

		finalReply = reply

		// Parse action
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

		// Extract thought from <think> tags if present
		if thinkStart := strings.Index(reply, "<think>"); thinkStart != -1 {
			if thinkEnd := strings.Index(reply, "</think>"); thinkEnd != -1 && thinkEnd > thinkStart {
				thought = strings.TrimSpace(reply[thinkStart+7 : thinkEnd])
				// Remove the think block from the reply entirely
				reply = strings.TrimSpace(reply[:thinkStart] + "\n" + reply[thinkEnd+8:])
				jsonStr = reply
				finalReply = reply // Update finalReply so it doesn't contain the thought tags

				if strings.TrimSpace(reply) == "" {
					jsonStr = thought
					finalReply = thought
				}
			} else {
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

		dec := json.NewDecoder(strings.NewReader(jsonStr))
		jsonErr := dec.Decode(&agentAction)
		if jsonErr == nil && agentAction.Action != "" {
			var executionResult string
			actionName := agentAction.Action

			// Normalize action name
			if actionName == "execute_skill" {
				actionName = agentAction.Name
			}

			// Background tasks don't typically span chat, they just execute generic skills
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

			if actionName == "search_memory" {
				var query string
				var payload struct {
					Query string `json:"query"`
				}
				if err := json.Unmarshal([]byte(argsStr), &payload); err == nil && payload.Query != "" {
					query = payload.Query
				} else {
					query = argsStr
				}

				if query == "" {
					skillResult := "Error: missing query argument"
					executionResult = skillResult
				} else {
					emb, err := provider.GenerateEmbedding(cfg, query)
					if err != nil {
						skillResult := fmt.Sprintf("Error generating embedding: %v", err)
						executionResult = skillResult
					} else {
						results, searchErr := memory.SearchMemory(emb, 10)
						if searchErr != nil || len(results) == 0 {
							skillResult := "No relevant memories found."
							executionResult = skillResult
						} else {
							skillResult := "Recall results:\n- " + strings.Join(results, "\n- ")
							executionResult = skillResult
						}
					}
				}
			} else {
				log.Printf("[Scheduler] Executing skill '%s' with args: %s", actionName, argsStr)
				skillResult := skills.ExecuteSkill(actionName, argsStr)
				executionResult = skillResult
			}

			ctxHistory = append(ctxHistory, models.Message{Role: "agent", Content: reply})
			toolResultMsg := fmt.Sprintf("<tool_output name=\"%s\">\n%s\n</tool_output>", actionName, executionResult)
			ctxHistory = append(ctxHistory, models.Message{Role: "user", Content: toolResultMsg})

		} else if strings.HasPrefix(strings.TrimSpace(jsonStr), "{") && strings.HasSuffix(strings.TrimSpace(jsonStr), "}") {
			log.Printf("[Scheduler] JSON parse failed: %v, payload: %s", jsonErr, jsonStr)
			ctxHistory = append(ctxHistory, models.Message{Role: "agent", Content: reply})
			ctxHistory = append(ctxHistory, models.Message{Role: "user", Content: fmt.Sprintf("<system_error>\nYour JSON failed to parse. Error: %v. Please make sure to output valid JSON. Do not write raw objects inside strings without escaping, and ensure the JSON is fully closed.\n</system_error>", jsonErr)})
			continue
		} else {
			// Finished execution
			break
		}

		if i == maxIterations-1 {
			log.Printf("[Scheduler] Hit max iterations limit. Forcing final summarization.")
			tempCtx := append(ctxHistory, models.Message{Role: "user", Content: fmt.Sprintf("<system_warning>\nYou have reached the maximum safety limit for automated steps (%d iterations). Please immediately provide a final report of this background task's progress. DO NOT output any JSON actions anymore. Remember to prefix your response with SUCCESS: or ERROR:.\n</system_warning>", maxIterations)})
			forcedReply, err := provider.GenerateResponse(context.Background(), cfg, tempCtx, availableSkills, nil, task.ID)
			if err == nil && forcedReply != "" {
				finalReply = forcedReply
			} else {
				finalReply = "ERROR: Task reached iteration limit. Last state: " + reply
			}
			isSuccess = false
		}
	}

	// Clean up think tags if they leaked into finalReply check
	if thinkStart := strings.Index(finalReply, "<think>"); thinkStart != -1 {
		if thinkEnd := strings.Index(finalReply, "</think>"); thinkEnd != -1 && thinkEnd > thinkStart {
			finalReply = strings.TrimSpace(finalReply[:thinkStart] + "\n" + finalReply[thinkEnd+8:])
		}
	}

	cleanReply := strings.TrimSpace(finalReply)
	isSemanticSuccess := isSuccess

	if strings.HasPrefix(cleanReply, "ERROR:") {
		isSemanticSuccess = false
		cleanReply = strings.TrimSpace(strings.TrimPrefix(cleanReply, "ERROR:"))
	} else if strings.HasPrefix(cleanReply, "SUCCESS:") {
		cleanReply = strings.TrimSpace(strings.TrimPrefix(cleanReply, "SUCCESS:"))
	}

	// Always save the raw final reply to the Task's own log
	memory.CreateTaskLog(task.ID, cleanReply)

	if isSemanticSuccess {
		cronStr := strings.TrimSpace(task.Cron)
		if cronStr == "" || strings.HasPrefix(cronStr, "delay:") || strings.HasPrefix(cronStr, "at:") || strings.HasPrefix(cronStr, "@delay ") || strings.HasPrefix(cronStr, "@at ") {
			memory.UpdateTaskStatus(task.ID, "completed")
		} else {
			memory.UpdateTaskStatus(task.ID, "running")
		}
		reportMsg := fmt.Sprintf("📝 **Background Task Completed: %s** (ID: %d)\n\n%s", task.Name, task.ID, cleanReply)
		memory.AddMessage("system_notification", reportMsg)
		if NotificationCallback != nil {
			NotificationCallback(reportMsg)
		}
	} else {
		memory.UpdateTaskStatus(task.ID, "error")
		errorMsg := fmt.Sprintf("❌ **Background Task Failed: %s** (ID: %d)\n\n%s", task.Name, task.ID, cleanReply)
		memory.AddMessage("system_notification", errorMsg)
		if NotificationCallback != nil {
			NotificationCallback(errorMsg)
		}
	}
}

// ScheduleTask adds a recurring task to the cron scheduler or schedules a one-time task
func ScheduleTask(task models.Task) {
	if c == nil {
		return
	}

	cronStr := strings.TrimSpace(task.Cron)
	if strings.HasPrefix(cronStr, "delay:") || strings.HasPrefix(cronStr, "@delay ") {
		delayStr := strings.TrimPrefix(cronStr, "delay:")
		delayStr = strings.TrimPrefix(delayStr, "@delay ")
		delayStr = strings.TrimSpace(delayStr)

		d, err := time.ParseDuration(delayStr)
		if err != nil {
			log.Printf("Failed to parse delay task %d: %v\n", task.ID, err)
			return
		}

		targetTime := task.CreatedAt.Add(d)
		now := time.Now()

		if now.After(targetTime) || now.Equal(targetTime) {
			log.Printf("Task %d delay passed. Running now.\n", task.ID)
			RunTaskOnce(task)
			return
		}

		remaining := targetTime.Sub(now)
		timer := time.AfterFunc(remaining, func() {
			executeTaskWithLoop(task)
			mu.Lock()
			delete(onceTimers, task.ID)
			mu.Unlock()
		})

		mu.Lock()
		onceTimers[task.ID] = timer
		mu.Unlock()

		log.Printf("Task %d scheduled for one-time execution at %s\n", task.ID, targetTime)
		return
	}

	if strings.HasPrefix(cronStr, "at:") || strings.HasPrefix(cronStr, "@at ") {
		atStr := strings.TrimPrefix(cronStr, "at:")
		atStr = strings.TrimPrefix(atStr, "@at ")
		atStr = strings.TrimSpace(atStr)

		targetTime, err := time.Parse(time.RFC3339, atStr)
		if err != nil {
			log.Printf("Failed to parse 'at' time %s for task %d: %v\n", atStr, task.ID, err)
			return
		}

		now := time.Now()
		if now.After(targetTime) || now.Equal(targetTime) {
			log.Printf("Task %d 'at' time passed. Running now.\n", task.ID)
			RunTaskOnce(task)
			return
		}

		remaining := targetTime.Sub(now)
		timer := time.AfterFunc(remaining, func() {
			executeTaskWithLoop(task)
			mu.Lock()
			delete(onceTimers, task.ID)
			mu.Unlock()
		})

		mu.Lock()
		onceTimers[task.ID] = timer
		mu.Unlock()

		log.Printf("Task %d scheduled for one-time execution at %s\n", task.ID, targetTime)
		return
	}

	entryID, err := c.AddFunc(task.Cron, func() {
		executeTaskWithLoop(task)
	})

	if err != nil {
		log.Printf("Failed to schedule task %d: %v\n", task.ID, err)
	} else {
		taskEntries[task.ID] = entryID
		log.Printf("Task %d scheduled with cron '%s' (EntryID: %v)\n", task.ID, task.Cron, entryID)
	}
}

// RunTaskOnce runs a task immediately in the background
func RunTaskOnce(task models.Task) {
	go executeTaskWithLoop(task)
}

// UnscheduleTask removes a specific task from the running cron scheduler
func UnscheduleTask(taskID uint) {
	if c == nil {
		return
	}
	if entryID, exists := taskEntries[taskID]; exists {
		c.Remove(entryID)
		delete(taskEntries, taskID)
		log.Printf("[Scheduler] Unschooled task ID %d (EntryID: %v)\n", taskID, entryID)
	}

	mu.Lock()
	if timer, exists := onceTimers[taskID]; exists {
		timer.Stop()
		delete(onceTimers, taskID)
		log.Printf("[Scheduler] Unschooled one-time task ID %d\n", taskID)
	}
	mu.Unlock()
}

// ClearAllTasks removes all tasks from the running cron scheduler
func ClearAllTasks() {
	if c == nil {
		return
	}
	for id, entryID := range taskEntries {
		c.Remove(entryID)
		delete(taskEntries, id)
	}

	mu.Lock()
	for id, timer := range onceTimers {
		timer.Stop()
		delete(onceTimers, id)
	}
	mu.Unlock()

	log.Println("[Scheduler] All scheduled tasks cleared.")
}
