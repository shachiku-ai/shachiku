package main

import (
	"log"
	"os"
	"path/filepath"

	"shachiku/internal/api"
	"shachiku/internal/config"
	"shachiku/internal/memory"
	"shachiku/internal/scheduler"
	"shachiku/internal/ssl"
	"shachiku/internal/telegram"
)

func main() {
	ssl.InitCertificate()

	log.Println("Initializing AI Agent API...")

	// Initialize long-term memory (Qdrant)
	memory.Init()

	// Start scheduled tasks (Cron)
	scheduler.Init()

	// Initialize Telegram Bot Watcher
	telegram.Init()

	// Wire background notifications to Telegram
	scheduler.NotificationCallback = telegram.SendNotification

	// Initialize API Routes
	r := api.SetupRoutes()

	certPath := filepath.Join(config.GetDataDir(), "certificate.crt")
	keyPath := filepath.Join(config.GetDataDir(), "private.key")

	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			log.Println("Agent API is running on :443 (HTTPS)")
			if err := r.RunTLS(":443", certPath, keyPath); err != nil {
				log.Fatal(err)
			}
			return
		}
	}

	bindAddr := ":8080"
	if os.Getenv("IS_PUBLIC") == "" {
		bindAddr = "127.0.0.1:8080"
	}

	log.Printf("Agent API is running on %s\n", bindAddr)
	if err := r.Run(bindAddr); err != nil {
		log.Fatal(err)
	}
}
