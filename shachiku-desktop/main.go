package main

import (
	"fmt"
	"log"
	"net"
	"net/http"

	"shachiku/core/api"
	"shachiku/core/memory"
	"shachiku/core/scheduler"
	"shachiku/core/ssl"
	"shachiku/core/telegram"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var Version = "dev"

func main() {
	ssl.InitCertificate()

	log.Printf("Initializing AI Agent API for Desktop (Version: %s)...", Version)

	// Initialize long-term memory (Qdrant/SQLite)
	memory.Init()

	// Start scheduled tasks (Cron)
	scheduler.Init()

	// Initialize Telegram Bot Watcher
	telegram.Init()

	// Wire background notifications to Telegram
	scheduler.NotificationCallback = telegram.SendNotification

	// Initialize API Routes
	r := api.SetupRoutes()

	// Listen on a random available port on localhost
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("Failed to listen on a port: %v", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	log.Printf("Internal Desktop API running at %s", url)

	// Start the backend server in a goroutine
	go func() {
		if err := http.Serve(listener, r); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Create and run the Wails application mapping to that random port
	app := application.New(application.Options{
		Name:        "shachiku-desktop",
		Description: "Shachiku AI Agent Client",
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Shachiku Agent",
		Width:            1024,
		Height:           768,
		BackgroundColour: application.NewRGB(27, 38, 54),
		URL:              url,
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
