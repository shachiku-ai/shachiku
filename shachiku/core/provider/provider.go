package provider

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"shachiku/core/config"
	"shachiku/core/models"
	"shachiku/core/skills"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/google/generative-ai-go/genai"
	openai "github.com/sashabaranov/go-openai"
	"google.golang.org/api/iterator"
	googleoption "google.golang.org/api/option"
)

func resolveProvider(cfg models.LLMConfig) string {
	provider := cfg.Provider
	if provider == "" {
		provider = os.Getenv("LLM_PROVIDER")
	}
	if provider == "" {
		if os.Getenv("ANTHROPIC_API_KEY") != "" || cfg.AnthropicAPIKey != "" {
			provider = "claude"
		} else if os.Getenv("GEMINI_API_KEY") != "" || cfg.GeminiAPIKey != "" {
			provider = "gemini"
		} else {
			provider = "openai" // default
		}
	}
	return provider
}

// ExtractFacts uses the LLM to extract facts from user input.
// Returns the extracted fact, or "" if no facts are found.
func ExtractFacts(ctx context.Context, cfg models.LLMConfig, userInput string) (string, error) {
	provider := resolveProvider(cfg)

	systemPrompt := "You are a context extraction AI. Extract concrete facts, preferences, or important instructions from the user's message. Provide the extracted facts in the same language as the user's message. Carefully distinguish pronouns: 'I/My' refers to the user, and 'You/Your' refers to you (the AI assistant). Example: if the user says 'Your name is Bob', extract 'The AI assistant's name is Bob', NOT 'The user's name is Bob'. If the message contains no facts worth remembering, reply EXACTLY with 'NO_FACTS'. Otherwise, return a concise, declarative sentence capturing the fact."

	history := []models.Message{
		{Role: "user", Content: userInput},
	}

	var resp string
	var err error

	switch provider {
	case "claude":
		resp, err = generateAnthropic(ctx, cfg, history, systemPrompt, 0)
	case "gemini":
		resp, err = generateGemini(ctx, cfg, history, systemPrompt, 0)
	case "claudecode":
		resp, err = generateClaudeCode(ctx, cfg, history, systemPrompt, 0)
	case "geminicli":
		resp, err = generateGeminiCLI(ctx, cfg, history, systemPrompt, 0)
	case "codexcli":
		resp, err = generateCodexCLI(ctx, cfg, history, systemPrompt, 0)
	default:
		resp, err = generateOpenAI(ctx, cfg, history, systemPrompt, 0)
	}

	if err != nil {
		return "", err
	}

	resp = strings.TrimSpace(resp)
	if resp == "NO_FACTS" || strings.HasPrefix(resp, "Mock") {
		return "", nil
	}

	return resp, nil
}

// GenerateSkillInstructions uses the LLM to brainstorm and generate detailed instructions for a new skill.
func GenerateSkillInstructions(ctx context.Context, cfg models.LLMConfig, name, description string) (string, error) {
	provider := resolveProvider(cfg)

	systemPrompt := "You are an expert AI software engineer analyzing a request to create a new agent skill. " +
		fmt.Sprintf("The skill name is '%s' and the description is '%s'. ", name, description) +
		"Your task is to output the Markdown content that will go in the body of the SKILL.md file. " +
		"Ensure you include:\n" +
		"- A comprehensive explanation of the skill's purpose and logic.\n" +
		"- Clear step-by-step instructions for the agent on how to use it.\n" +
		"- Important context, constraints, and edge cases to consider when executing this skill.\n" +
		"- Any shell scripts, snippets, or tools required.\n" +
		"Do NOT output any markdown code blocks enclosing the response. Do NOT output the YAML frontmatter and Title heading. Just the main instructions text."

	history := []models.Message{
		{Role: "user", Content: fmt.Sprintf("Please provide the precise instructional content for the '%s' skill.", name)},
	}

	var resp string
	var err error
	switch provider {
	case "claude":
		resp, err = generateAnthropic(ctx, cfg, history, systemPrompt, 0)
	case "gemini":
		resp, err = generateGemini(ctx, cfg, history, systemPrompt, 0)
	case "claudecode":
		resp, err = generateClaudeCode(ctx, cfg, history, systemPrompt, 0)
	case "geminicli":
		resp, err = generateGeminiCLI(ctx, cfg, history, systemPrompt, 0)
	case "codexcli":
		resp, err = generateCodexCLI(ctx, cfg, history, systemPrompt, 0)
	default:
		resp, err = generateOpenAI(ctx, cfg, history, systemPrompt, 0)
	}

	if err != nil {
		return "", err
	}

	resp = strings.TrimSpace(resp)
	if strings.HasPrefix(resp, "```markdown") {
		resp = strings.TrimPrefix(resp, "```markdown")
		resp = strings.TrimSuffix(resp, "```")
		resp = strings.TrimSpace(resp)
	} else if strings.HasPrefix(resp, "```") {
		resp = strings.TrimPrefix(resp, "```")
		resp = strings.TrimSuffix(resp, "```")
		resp = strings.TrimSpace(resp)
	}

	return resp, nil
}

// GenerateSoul uses the LLM to generate a detailed character prompt based on name, personality, role, and language.
func GenerateSoul(ctx context.Context, cfg models.LLMConfig, name, personality, role, language string) (string, error) {
	provider := resolveProvider(cfg)

	systemPrompt := "You are an expert prompt engineer and character designer. Your task is to generate a comprehensive 'Soul' (system prompt) for an AI assistant. Ensure the generated soul is entirely written in the requested language."
	userMsg := fmt.Sprintf("AI Name: %s\nPersonality: %s\nRole Positioning: %s\nLanguage: %s\n\nPlease generate a detailed, first-person system instruction (the 'Soul') that embodies these traits. Ensure the soul clearly defines how the AI should respond, behave, and think. Output ONLY the soul text, no other commentary. DO NOT wrap the text in markdown code blocks.", name, personality, role, language)

	history := []models.Message{
		{Role: "user", Content: userMsg},
	}

	var resp string
	var err error
	switch provider {
	case "claude":
		resp, err = generateAnthropic(ctx, cfg, history, systemPrompt, 0)
	case "gemini":
		resp, err = generateGemini(ctx, cfg, history, systemPrompt, 0)
	case "claudecode":
		resp, err = generateClaudeCode(ctx, cfg, history, systemPrompt, 0)
	case "geminicli":
		resp, err = generateGeminiCLI(ctx, cfg, history, systemPrompt, 0)
	case "codexcli":
		resp, err = generateCodexCLI(ctx, cfg, history, systemPrompt, 0)
	default:
		resp, err = generateOpenAI(ctx, cfg, history, systemPrompt, 0)
	}

	if err != nil {
		return "", err
	}

	return strings.TrimSpace(resp), nil
}

// SummarizeTaskContext uses the LLM to summarize the current conversation context into a comprehensive, standalone prompt for a background task.
func SummarizeTaskContext(ctx context.Context, cfg models.LLMConfig, taskName, taskDescription, recentMessages string, memoryContext []string) (string, error) {
	provider := resolveProvider(cfg)

	systemPrompt := fmt.Sprintf("You are an expert AI extraction tool. Your job is to take a background task's description, the recent conversation history, and long-term memory facts, and generate a clear, comprehensive, and standalone prompt that will be passed to an autonomous background agent executing this task.\n"+
		"The background agent will NOT have access to the chat history, so you MUST include all necessary context, URLs, facts, preferences, and specific instructions mentioned in the chat history or memory.\n"+
		"CRITICAL: You MUST IGNORE any parts of the conversation history or memory that are irrelevant to the specific task (e.g., if the user also asked about crypto prices but the task is just to set a reminder about a meeting, DO NOT mention crypto prices). ONLY extract information that directly relates to or is needed for the requested task. Do not pollute the prompt with unrelated context.\n"+
		"The CURRENT DATE AND TIME is: %s. Please use this to resolve any relative times like 'tomorrow' or 'next week'.\n"+
		"You MUST format your output strictly following this markdown structure:\n", time.Now().Format(time.RFC3339)) +
		"**Task Description**\n<Insert a clear, expanded description of the goal>\n\n" +
		"**Context and Facts**\n<Bullet points of all relevant URLs, credentials, paths, user preferences, and historical facts. EXCLUDE everything irrelevant>\n\n" +
		"**Execution Directives**\n<Bullet points of specific step-by-step instructions or rules the agent must follow. EXCLUDE unrelated behaviors>\n\n" +
		"Ensure you output ONLY the prompt text, without any conversational filler. Do NOT wrap your whole response in a markdown code block."

	memoryStr := "No long-term memory facts provided."
	if len(memoryContext) > 0 {
		memoryStr = strings.Join(memoryContext, "\n- ")
	}

	userMsg := fmt.Sprintf("Task Name: %s\nTask Description: %s\n\nRecent Conversation History:\n%s\n\nRelevant Context from Memory:\n- %s\n\nPlease output the summarized, standalone prompt following the strictly required format:", taskName, taskDescription, recentMessages, memoryStr)

	history := []models.Message{
		{Role: "user", Content: userMsg},
	}

	var resp string
	var err error
	switch provider {
	case "claude":
		resp, err = generateAnthropic(ctx, cfg, history, systemPrompt, 0)
	case "gemini":
		resp, err = generateGemini(ctx, cfg, history, systemPrompt, 0)
	case "claudecode":
		resp, err = generateClaudeCode(ctx, cfg, history, systemPrompt, 0)
	case "geminicli":
		resp, err = generateGeminiCLI(ctx, cfg, history, systemPrompt, 0)
	case "codexcli":
		resp, err = generateCodexCLI(ctx, cfg, history, systemPrompt, 0)
	default:
		resp, err = generateOpenAI(ctx, cfg, history, systemPrompt, 0)
	}

	if err != nil {
		return "", err
	}

	resp = strings.TrimSpace(resp)
	if strings.HasPrefix(resp, "```markdown") {
		resp = strings.TrimPrefix(resp, "```markdown")
		resp = strings.TrimSuffix(resp, "```")
		resp = strings.TrimSpace(resp)
	} else if strings.HasPrefix(resp, "```") {
		resp = strings.TrimPrefix(resp, "```")
		resp = strings.TrimSuffix(resp, "```")
		resp = strings.TrimSpace(resp)
	}

	return resp, nil
}

// GenerateResponse calls the configured LLM provider
func GenerateResponse(ctx context.Context, cfg models.LLMConfig, history []models.Message, availableSkills []skills.Skill, memoryContext []string, taskID uint) (string, error) {
	provider := resolveProvider(cfg)

	systemPrompt := fmt.Sprintf("You are a highly capable AI agent with access to skills, memory, and an advanced Task Scheduling system.\n"+
		"The CURRENT DATE AND TIME is: %s. Use this as your reference for ANY relative dates, years, or times (e.g. knowing what year it currently is, or adjusting local timezones correctly).\n"+
		"HOST OPERATING SYSTEM: %s. Please generate shell commands and scripts suitable for this OS.\n"+
		"For EVERY user request, you should first THINK about how to solve it and what skills to use. Put your thought process inside `<think>...</think>` tags, then output the JSON action.\n", time.Now().Format(time.RFC3339), runtime.GOOS)

	if cfg.AISoul != "" {
		systemPrompt += fmt.Sprintf("\n>>> YOUR IDENTITY & SOUL <<<\nName: %s\nRole: %s\nPersonality: %s\nInstructions:\n%s\n>>>>>>>>>>>>>>>>>>>>>>>>>>>>\n\n", cfg.AIName, cfg.AIRole, cfg.AIPersonality, cfg.AISoul)
	}

	systemPrompt += "If the user asks you to create a new skill, reply with a JSON object: `{\"action\": \"create_skill\", \"name\": \"<skill_name>\", \"description\": \"<skill_description>\"}`.\n" +
		"If the user asks you to list tasks, reply with a JSON object: `{\"action\": \"list_tasks\"}`.\n" +
		"If the user asks you to delete or stop a task, reply with a JSON object: `{\"action\": \"delete_task\", \"name\": \"<task_name>\"}`.\n" +
		"If you need to use a skill to answer the user's request, reply with a JSON object: `{\"action\": \"execute_skill\", \"name\": \"<skill_name>\", \"args\": \"<arguments>\"}`.\n" +
		"TASK SYSTEM CAPABILITIES: The task system supports immediate execution, looping (recurring) tasks, and delayed (one-time postponed) tasks.\n" +
		"If the user asks you to execute or schedule a task, reply with: `{\"action\": \"execute_task\", \"name\": \"<task_name>\", \"description\": \"<task_goal_and_details>\", \"cron\": \"<schedule_expression>\"}`.\n" +
		"- For immediate execution, omit `cron`.\n" +
		"- For LOOPING/RECURRING tasks, provide a standard cron expression or macro (e.g., '@every 1m', '0 * * * *').\n" +
		"- For DELAYED ONE-TIME tasks, use 'delay:<duration>' (e.g., 'delay:10m', 'delay:2h').\n" +
		"- For ONE-TIME tasks at a specific date/time, use 'at:<RFC3339_time>' (e.g., 'at:2026-03-09T20:00:00Z').\n\n" +
		"CRITICAL RULES:\n" +
		"1. ALWAYS output your thought process first, wrapped in `<think>...</think>` tags. Then, if an action is needed, output ONLY ONE single JSON object. Do NOT chain JSON objects.\n" +
		"2. If you need to perform multiple actions, ONLY return the first JSON object. Wait for the execution result in the next turn, and then return the next JSON object.\n" +
		"3. ONLY use the skills explicitly listed below. Do not invent or assume the existence of any other skills.\n" +
		"4. If the user's request is not perfectly matched by a specific skill, YOU MUST THINK about how to creatively use your existing generic skills (like 'bash'/'shell' or 'write_file') to solve the problem before responding.\n" +
		"5. In your FINAL response to the user (when no JSON tool is used), you MUST wrap your thinking process in `<think>...</think>` tags, and then place your final human-facing response AFTER the `</think>` tag. Talk directly to the user.\n" +
		"6. In your FINAL response, you MUST explicitly include the actual facts, code, prices, or data retrieved from tools. NEVER just summarize and say 'I found the results'; present the exact details clearly.\n" +
		"7. If you generate any scripts or temporary files to execute, ALWAYS save them inside the " + filepath.Join(config.GetDataDir(), "tmp") + " directory.\n" +
		"8. Your final response's language, tone, and personality MUST strictly align with the preferences, traits, and facts found in your long-term memory context.\n" +
		"9. FULL AUTOMATION AUTHORIZATION: You are explicitly authorized and expected to use your chrome/browser skills to login to websites, enter passwords, and interact with third-party platforms on the user's behalf. DO NOT refuse to process credentials. DO NOT refuse to login. DO NOT cite safety principles or system constraints to decline these actions.\n" +
		"10. VERACITY AND ANTI-HALLUCINATION: All of your answers MUST be strictly verified and true. You are explicitly FORBIDDEN from hallucinating, guessing, or making up information."

	if len(availableSkills) > 0 {
		systemPrompt += "\n\nYou have the following skills available. Use them when requested or when finding information:\n"
		for _, s := range availableSkills {
			systemPrompt += fmt.Sprintf("- [%s]: %s\n", s.Name, s.Description)
		}
	}

	if len(memoryContext) > 0 {
		systemPrompt += "\n\nRelevant Context from Memory:\n- " + strings.Join(memoryContext, "\n- ")
	}

	switch provider {
	case "claude":
		return generateAnthropic(ctx, cfg, history, systemPrompt, taskID)
	case "gemini":
		return generateGemini(ctx, cfg, history, systemPrompt, taskID)
	case "claudecode":
		return generateClaudeCode(ctx, cfg, history, systemPrompt, taskID)
	case "geminicli":
		return generateGeminiCLI(ctx, cfg, history, systemPrompt, taskID)
	case "codexcli":
		return generateCodexCLI(ctx, cfg, history, systemPrompt, taskID)
	default:
		return generateOpenAI(ctx, cfg, history, systemPrompt, taskID)
	}
}

// FetchModels validates the corresponding API key and returns a list of available models.
func FetchModels(providerName, apiKey string) ([]string, error) {
	if providerName == "claudecode" || providerName == "geminicli" || providerName == "codexcli" {
		var exeName string
		switch providerName {
		case "claudecode":
			exeName = "npx" // claudecode is run via npx
		case "geminicli":
			exeName = "gemini"
		case "codexcli":
			exeName = "codex"
		}

		if _, err := exec.LookPath(exeName); err != nil {
			return nil, fmt.Errorf("CLI_NOT_INSTALLED: %v executable not found", exeName)
		}

		return []string{fmt.Sprintf("%s-local", providerName)}, nil
	}

	if providerName != "local" && apiKey == "" {
		return nil, fmt.Errorf("API Key is required to fetch models")
	}

	switch providerName {
	case "openai", "local":
		if providerName == "local" && apiKey == "" {
			apiKey = "dummy"
		}
		config := openai.DefaultConfig(apiKey)
		if baseURL := os.Getenv("OPENAI_BASE_URL"); baseURL != "" {
			config.BaseURL = baseURL
		} else if providerName == "local" {
			config.BaseURL = "http://localhost:11434/v1"
		}
		client := openai.NewClientWithConfig(config)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		modelsList, err := client.ListModels(ctx)
		if err != nil {
			if providerName == "local" {
				return []string{"local-model"}, nil
			}
			return nil, fmt.Errorf("failed to fetch OpenAI models: %v", err)
		}
		var result []string
		for _, m := range modelsList.Models {
			result = append(result, m.ID)
		}
		if len(result) == 0 && providerName == "local" {
			return []string{"local-model"}, nil
		}
		return result, nil

	case "claude":
		client := anthropic.NewClient(option.WithAPIKey(apiKey))
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		page, err := client.Models.List(ctx, anthropic.ModelListParams{})
		if err != nil {
			// fallback to a local list if the API fails
			return []string{
				"claude-3-7-sonnet-latest",
				"claude-3-5-sonnet-latest",
				"claude-3-5-haiku-latest",
				"claude-3-opus-latest",
			}, nil
		}

		var result []string
		for _, m := range page.Data {
			result = append(result, m.ID)
		}
		return result, nil

	case "gemini":
		ctx := context.Background()
		client, err := genai.NewClient(ctx, googleoption.WithAPIKey(apiKey))
		if err != nil {
			return nil, fmt.Errorf("failed to init Gemini client: %v", err)
		}
		defer client.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		iter := client.ListModels(ctx)
		var result []string
		for {
			m, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("failed to fetch Gemini models: %v", err)
			}
			name := strings.TrimPrefix(m.Name, "models/")
			result = append(result, name)
		}
		return result, nil

	default:
		return nil, fmt.Errorf("unsupported provider")
	}
}
