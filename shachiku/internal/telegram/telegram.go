package telegram

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"shachiku/internal/agent"
	"shachiku/internal/memory"
)

var (
	cancelCtx    context.CancelFunc
	mu           sync.Mutex
	currentToken string

	ActiveBot   *tgbotapi.BotAPI
	AdminChatID int64
)

func Init() {
	log.Println("Initializing Telegram integration watcher...")
	go watchConfig()
}

func watchConfig() {
	for {
		cfg := memory.GetLLMConfig()
		mu.Lock()
		if cfg.TelegramBotToken != currentToken {
			log.Printf("[Telegram] Token changed or detected, checking if start needed...")
			currentToken = cfg.TelegramBotToken
			if cancelCtx != nil {
				cancelCtx()
				cancelCtx = nil
			}

			if currentToken != "" {
				ctx, cancel := context.WithCancel(context.Background())
				cancelCtx = cancel
				go startBot(ctx, currentToken)
			}
		}
		mu.Unlock()

		time.Sleep(5 * time.Second)
	}
}

func startBot(ctx context.Context, token string) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Printf("[Telegram] Failed to start bot or invalid token: %v", err)
		return
	}

	mu.Lock()
	ActiveBot = bot
	mu.Unlock()

	log.Printf("[Telegram] Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			bot.StopReceivingUpdates()
			return
		case update := <-updates:
			if update.Message == nil || update.Message.Text == "" {
				continue
			}

			go handleMessage(bot, update.Message)
		}
	}
}

func handleMessage(bot *tgbotapi.BotAPI, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	text := msg.Text
	username := msg.From.UserName

	cfg := memory.GetLLMConfig()
	if cfg.AllowedTelegramUsers != "" {
		allowed := false
		users := strings.Split(cfg.AllowedTelegramUsers, ",")
		for _, u := range users {
			if strings.TrimSpace(u) == username {
				allowed = true
				break
			}
		}
		if !allowed {
			log.Printf("[Telegram] Rejecting message from unauthorized user: %s", username)
			bot.Send(tgbotapi.NewMessage(chatID, "⛔ Unauthorized user. You are not allowed to interact with this agent."))
			return
		}
	}

	// Update AdminChatID to the most recent user
	mu.Lock()
	AdminChatID = chatID
	mu.Unlock()

	log.Printf("[Telegram] Received message from %s: %s", msg.From.UserName, text)

	// Send an initial "thinking" message
	thinkMsg, err := bot.Send(tgbotapi.NewMessage(chatID, "⏳ _Thinking..._"))
	if err == nil {
		// Set parsemode for edit
		editMsg := tgbotapi.NewEditMessageText(chatID, thinkMsg.MessageID, "⏳ _Thinking..._")
		editMsg.ParseMode = "Markdown"
		bot.Send(editMsg)
	}

	ctx := context.Background()
	var lastStepTime time.Time

	finalReply, agentErr := agent.ProcessMessage(ctx, text, func(step string) {
		// Throttle updates to Telegram to avoid hitting rate limits
		if err == nil && time.Since(lastStepTime) > 3*time.Second && step != "" {
			edit := tgbotapi.NewEditMessageText(chatID, thinkMsg.MessageID, "⏳ _"+step+"_")
			edit.ParseMode = "Markdown"
			bot.Send(edit)
			lastStepTime = time.Now()
		}
	})

	if agentErr != nil {
		if err == nil {
			bot.Send(tgbotapi.NewEditMessageText(chatID, thinkMsg.MessageID, "❌ Error: "+agentErr.Error()))
		} else {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Error: "+agentErr.Error()))
		}
		return
	}

	if err == nil {
		// Delete the temporary thinking message
		bot.Request(tgbotapi.NewDeleteMessage(chatID, thinkMsg.MessageID))
	}

	replyMsg := tgbotapi.NewMessage(chatID, finalReply)
	// Try standard markdown parsing for cleaner display if LLM outputs markdown
	replyMsg.ParseMode = "Markdown"
	_, sendErr := bot.Send(replyMsg)

	if sendErr != nil {
		// Telegram markdown parsing is strict and might fail. Fallback to raw text without Markdown
		replyMsg.ParseMode = ""
		bot.Send(replyMsg)
	}
}

// SendNotification pushes a message directly to the latest active Telegram chat
func SendNotification(msg string) {
	mu.Lock()
	bot := ActiveBot
	chatID := AdminChatID
	mu.Unlock()

	if bot != nil && chatID != 0 {
		replyMsg := tgbotapi.NewMessage(chatID, msg)
		replyMsg.ParseMode = "Markdown"
		if _, err := bot.Send(replyMsg); err != nil {
			replyMsg.ParseMode = ""
			bot.Send(replyMsg)
		}
	}
}
