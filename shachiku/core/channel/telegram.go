package channel

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"shachiku/core/agent"
	"shachiku/core/memory"
	"shachiku/core/models"
)

type TelegramModule struct {
	bot       *tgbotapi.BotAPI
	cancelCtx context.CancelFunc
	adminChat int64
}

func NewTelegramModule() *TelegramModule {
	return &TelegramModule{}
}

func (m *TelegramModule) Start(cfg models.LLMConfig) error {
	bot, err := tgbotapi.NewBotAPI(cfg.TelegramBotToken)
	if err != nil {
		return err
	}

	m.bot = bot
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelCtx = cancel

	log.Printf("[Telegram] Authorized on account %s", bot.Self.UserName)

	go m.listen(ctx)
	return nil
}

func (m *TelegramModule) Stop() {
	if m.cancelCtx != nil {
		m.cancelCtx()
		m.cancelCtx = nil
	}
	if m.bot != nil {
		m.bot.StopReceivingUpdates()
		m.bot = nil
	}
}

func (m *TelegramModule) listen(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := m.bot.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			return
		case update := <-updates:
			if update.Message == nil {
				continue
			}
			if update.Message.Text == "" && update.Message.Caption == "" && update.Message.Document == nil && len(update.Message.Photo) == 0 {
				continue
			}

			go m.handleMessage(update.Message)
		}
	}
}

func (m *TelegramModule) handleMessage(msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	text := msg.Text
	if text == "" && msg.Caption != "" {
		text = msg.Caption
	}

	var attachedFiles []string

	if msg.Document != nil {
		fileURL, err := m.bot.GetFileDirectURL(msg.Document.FileID)
		if err == nil {
			if path, err := downloadFileToTmp(fileURL, msg.Document.FileName); err == nil {
				attachedFiles = append(attachedFiles, path)
			}
		}
	}

	if len(msg.Photo) > 0 {
		photo := msg.Photo[len(msg.Photo)-1]
		fileURL, err := m.bot.GetFileDirectURL(photo.FileID)
		if err == nil {
			if path, err := downloadFileToTmp(fileURL, photo.FileID+".jpg"); err == nil {
				attachedFiles = append(attachedFiles, path)
			}
		}
	}

	for _, f := range attachedFiles {
		text += fmt.Sprintf("\n@%s", f)
	}

	username := msg.From.UserName

	cfg := memory.GetLLMConfig()
	if cfg.AllowedTelegramUsers == "" {
		log.Printf("[Telegram] Rejecting message because no allowed users are configured.")
		m.bot.Send(tgbotapi.NewMessage(chatID, "⛔ Unauthorized user. No allowed users are configured."))
		return
	}

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
		m.bot.Send(tgbotapi.NewMessage(chatID, "⛔ Unauthorized user. You are not allowed to interact with this agent."))
		return
	}

	// Update AdminChatID to the most recent user
	m.adminChat = chatID

	log.Printf("[Telegram] Received message from %s: %s", msg.From.UserName, text)

	// Send an initial "thinking" message
	thinkMsg, err := m.bot.Send(tgbotapi.NewMessage(chatID, "⏳ _Thinking..._"))
	if err == nil {
		// Set parsemode for edit
		editMsg := tgbotapi.NewEditMessageText(chatID, thinkMsg.MessageID, "⏳ _Thinking..._")
		editMsg.ParseMode = "Markdown"
		m.bot.Send(editMsg)
	}

	ctx := context.Background()
	var lastStepTime time.Time

	finalReply, agentErr := agent.ProcessMessage(ctx, text, func(step string) {
		// Throttle updates to Telegram to avoid hitting rate limits
		if err == nil && time.Since(lastStepTime) > 3*time.Second && step != "" {
			edit := tgbotapi.NewEditMessageText(chatID, thinkMsg.MessageID, "⏳ _"+step+"_")
			edit.ParseMode = "Markdown"
			m.bot.Send(edit)
			lastStepTime = time.Now()
		}
	})

	if agentErr != nil {
		if err == nil {
			m.bot.Send(tgbotapi.NewEditMessageText(chatID, thinkMsg.MessageID, "❌ Error: "+agentErr.Error()))
		} else {
			m.bot.Send(tgbotapi.NewMessage(chatID, "❌ Error: "+agentErr.Error()))
		}
		return
	}

	if err == nil {
		// Delete the temporary thinking message
		m.bot.Request(tgbotapi.NewDeleteMessage(chatID, thinkMsg.MessageID))
	}

	replyMsg := tgbotapi.NewMessage(chatID, finalReply)
	replyMsg.ParseMode = "Markdown"
	_, sendErr := m.bot.Send(replyMsg)

	if sendErr != nil {
		replyMsg.ParseMode = ""
		m.bot.Send(replyMsg)
	}
}

func (m *TelegramModule) SendNotification(msg string) error {
	if m.bot != nil && m.adminChat != 0 {
		replyMsg := tgbotapi.NewMessage(m.adminChat, msg)
		replyMsg.ParseMode = "Markdown"
		if _, err := m.bot.Send(replyMsg); err != nil {
			replyMsg.ParseMode = ""
			_, err = m.bot.Send(replyMsg)
			return err
		}
		return nil
	}
	return fmt.Errorf("no active telegram session or admin chat")
}
