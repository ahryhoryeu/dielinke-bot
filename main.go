package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type config struct {
	InstagramReelsMode string
	YTDLPPath          string
}

func loadConfig() config {
	cfg := config{
		InstagramReelsMode: strings.ToLower(os.Getenv("INSTAGRAM_REELS_MODE")),
		YTDLPPath:          os.Getenv("YTDLP_PATH"),
	}

	if cfg.InstagramReelsMode == "" {
		cfg.InstagramReelsMode = "link"
	}
	if cfg.YTDLPPath == "" {
		cfg.YTDLPPath = "yt-dlp"
	}

	return cfg
}

func main() {
	// Start HTTP server
	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Die Linke Bot - Because even socialists need to fix their links! 🚀\n\nThis bot transforms your social media links into more accessible versions. No capitalist algorithms here!"))
		})

		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		log.Println("Starting HTTP server on :8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatal("HTTP server failed:", err)
		}
	}()

	// Telegram bot setup
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN environment variable is not set")
	}
	cfg := loadConfig()

	bot, err := tgbotapi.NewBotAPIWithClient(botToken, tgbotapi.APIEndpoint, &http.Client{Timeout: 2 * time.Minute})
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = false
	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	worker := newMediaWorker(bot, newMediaProcessor(cfg))
	defer worker.close()
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		// Check if the message contains a URL
		if update.Message.Text != "" {
			handleMessage(bot, update.Message, cfg, worker)
		}
	}
}
