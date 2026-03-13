package main

import (
	"log"
	"os"
	"path/filepath"

	"shachiku/core/api"
	"shachiku/core/config"
	"shachiku/core/memory"
	"shachiku/core/scheduler"
	"shachiku/core/ssl"
	"shachiku/core/telegram"
)

var Version = "dev"

func main() {
	ssl.InitCertificate()

	log.Printf("Initializing AI Agent API (Version: %s)...", Version)

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

	certPath := filepath.Join(config.GetCertDir(), "certificate.crt")
	keyPath := filepath.Join(config.GetCertDir(), "private.key")

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
