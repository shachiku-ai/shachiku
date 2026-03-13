package channel

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"shachiku/core/agent"
	"shachiku/core/memory"
	"shachiku/core/models"
)

type DiscordModule struct {
	session   *discordgo.Session
	adminChat string
}

func NewDiscordModule() *DiscordModule {
	return &DiscordModule{}
}

func (m *DiscordModule) Start(cfg models.LLMConfig) error {
	dg, err := discordgo.New("Bot " + cfg.DiscordBotToken)
	if err != nil {
		return err
	}

	m.session = dg

	dg.AddHandler(m.messageCreate)

	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages | discordgo.IntentsMessageContent

	err = dg.Open()
	if err != nil {
		return err
	}

	log.Printf("[Discord] Authorized on account %s", dg.State.User.Username)

	return nil
}

func (m *DiscordModule) Stop() {
	if m.session != nil {
		m.session.Close()
		m.session = nil
	}
}

func (m *DiscordModule) messageCreate(s *discordgo.Session, msg *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself
	if msg.Author.ID == s.State.User.ID {
		return
	}

	text := msg.Content

	var attachedFiles []string
	for _, attachment := range msg.Attachments {
		if path, err := downloadFileToTmp(attachment.URL, attachment.Filename); err == nil {
			attachedFiles = append(attachedFiles, path)
		}
	}

	for _, f := range attachedFiles {
		text += fmt.Sprintf("\n@%s", f)
	}

	if strings.TrimSpace(text) == "" {
		return
	}

	username := msg.Author.Username
	userID := msg.Author.ID

	cfg := memory.GetLLMConfig()
	if cfg.AllowedDiscordUsers == "" {
		log.Printf("[Discord] Rejecting message because no allowed users are configured.")
		s.ChannelMessageSend(msg.ChannelID, "⛔ Unauthorized user. No allowed users are configured.")
		return
	}

	allowed := false
	users := strings.Split(cfg.AllowedDiscordUsers, ",")
	for _, u := range users {
		u = strings.TrimSpace(u)
		if u == username || u == userID {
			allowed = true
			break
		}
	}

	if !allowed {
		log.Printf("[Discord] Rejecting message from unauthorized user: %s (ID: %s)", username, userID)
		s.ChannelMessageSend(msg.ChannelID, "⛔ Unauthorized user. You are not allowed to interact with this agent.")
		return
	}

	// Update AdminChatID to the most recent channel
	m.adminChat = msg.ChannelID

	log.Printf("[Discord] Received message from %s: %s", username, text)

	// Send an initial "thinking" message
	thinkMsg, err := s.ChannelMessageSend(msg.ChannelID, "⏳ *Thinking...*")

	ctx := context.Background()
	var lastStepTime time.Time

	finalReply, agentErr := agent.ProcessMessage(ctx, text, func(step string) {
		if err == nil && time.Since(lastStepTime) > 3*time.Second && step != "" {
			s.ChannelMessageEdit(msg.ChannelID, thinkMsg.ID, "⏳ *"+step+"*")
			lastStepTime = time.Now()
		}
	})

	if agentErr != nil {
		if err == nil {
			s.ChannelMessageEdit(msg.ChannelID, thinkMsg.ID, "❌ Error: "+agentErr.Error())
		} else {
			s.ChannelMessageSend(msg.ChannelID, "❌ Error: "+agentErr.Error())
		}
		return
	}

	if err == nil {
		s.ChannelMessageDelete(msg.ChannelID, thinkMsg.ID)
	}

	// Discord max message length is 2000 characters
	if len(finalReply) > 2000 {
		var msgs []string
		for i := 0; i < len(finalReply); i += 2000 {
			end := i + 2000
			if end > len(finalReply) {
				end = len(finalReply)
			}
			msgs = append(msgs, finalReply[i:end])
		}
		for _, mstr := range msgs {
			s.ChannelMessageSend(msg.ChannelID, mstr)
		}
	} else {
		s.ChannelMessageSend(msg.ChannelID, finalReply)
	}
}

func (m *DiscordModule) SendNotification(msgStr string) error {
	if m.session != nil && m.adminChat != "" {
		if len(msgStr) > 2000 {
			msgStr = msgStr[:1997] + "..."
		}
		_, err := m.session.ChannelMessageSend(m.adminChat, msgStr)
		return err
	}
	return fmt.Errorf("no active discord session or admin chat")
}
