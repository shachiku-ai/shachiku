package models

import "time"

type Message struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Role      string    `json:"Role"`
	Content   string    `json:"Content"`
	CreatedAt time.Time `json:"CreatedAt"`
}

type Task struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name"`
	Cron      string    `json:"cron"`
	Prompt    string    `json:"prompt"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"CreatedAt"`
}

type TaskLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TaskID    uint      `json:"task_id"`
	Output    string    `json:"output"`
	CreatedAt time.Time `json:"CreatedAt"`
}

type Fact struct {
	ID        string `json:"id"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

type LLMConfig struct {
	ID                   uint   `gorm:"primaryKey" json:"id"`
	Provider             string `json:"provider"`
	Model                string `json:"model"`
	OpenAIAPIKey         string `json:"openai_api_key"`
	AnthropicAPIKey      string `json:"anthropic_api_key"`
	GeminiAPIKey         string `json:"gemini_api_key"`
	OpenRouterAPIKey     string `json:"openrouter_api_key"`
	LocalAPIKey          string `json:"local_api_key"`
	LocalEndpoint        string `json:"local_endpoint"`
	TelegramBotToken     string `json:"telegram_bot_token"`
	AllowedTelegramUsers string `json:"allowed_telegram_users"`
	DiscordBotToken      string `json:"discord_bot_token"`
	AllowedDiscordUsers  string `json:"allowed_discord_users"`
	ChannelProvider      string `json:"channel_provider"`
	MaxIterations        int    `json:"max_iterations"`
	AIName               string `json:"ai_name"`
	AIPersonality        string `json:"ai_personality"`
	AIRole               string `json:"ai_role"`
	AISoul               string `json:"ai_soul"`
	SetupCompleted       bool   `json:"setup_completed"`
}

type TokenLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	TaskID       uint      `json:"task_id"` // 0 if it's a regular chat message
	CreatedAt    time.Time `json:"CreatedAt"`
}

type TokenUsage struct {
	InputTokens  int
	OutputTokens int
}
