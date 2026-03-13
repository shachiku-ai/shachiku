package channel

import (
	"log"
	"sync"
	"time"

	"shachiku/core/memory"
	"shachiku/core/models"
)

var (
	mu           sync.Mutex
	activeModule ChannelModule
	currentCfg   models.LLMConfig
)

type ChannelModule interface {
	Start(cfg models.LLMConfig) error
	Stop()
	SendNotification(msg string) error
}

func Init() {
	log.Println("Initializing Channel integration watcher...")
	go watchConfig()
}

func watchConfig() {
	for {
		cfg := memory.GetLLMConfig()
		mu.Lock()

		needsRestart := false

		if cfg.ChannelProvider != currentCfg.ChannelProvider {
			needsRestart = true
		} else {
			if cfg.ChannelProvider == "telegram" && (cfg.TelegramBotToken != currentCfg.TelegramBotToken) {
				needsRestart = true
			} else if cfg.ChannelProvider == "discord" && (cfg.DiscordBotToken != currentCfg.DiscordBotToken) {
				needsRestart = true
			}
		}

		if needsRestart {
			log.Printf("[Channel] Configuration changed, switching module to %s...", cfg.ChannelProvider)
			currentCfg = cfg

			if activeModule != nil {
				activeModule.Stop()
				activeModule = nil
			}

			if cfg.ChannelProvider == "telegram" && cfg.TelegramBotToken != "" {
				m := NewTelegramModule()
				err := m.Start(cfg)
				if err == nil {
					activeModule = m
				} else {
					log.Printf("[Channel] Failed to start Telegram module: %v", err)
				}
			} else if cfg.ChannelProvider == "discord" && cfg.DiscordBotToken != "" {
				m := NewDiscordModule()
				err := m.Start(cfg)
				if err == nil {
					activeModule = m
				} else {
					log.Printf("[Channel] Failed to start Discord module: %v", err)
				}
			}
		}
		mu.Unlock()

		time.Sleep(5 * time.Second)
	}
}

func SendNotification(msg string) {
	mu.Lock()
	m := activeModule
	mu.Unlock()

	if m != nil {
		m.SendNotification(msg)
	}
}
