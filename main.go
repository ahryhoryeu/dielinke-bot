package main

import (
	"log"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN environment variable is not set")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		// Check if the message contains a URL
		if update.Message.Text != "" {
			handleMessage(bot, update.Message)
		}
	}
}

func handleMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	matches := FindAndTransformLinks(message.Text)

	for _, match := range matches {
		var replyText string
		switch match.Type {
		case "instagram":
			replyText = "Instagram reel detected. Here's the kkinstagram version:\n" + match.TransformedURL
		case "twitter":
			replyText = "Twitter post detected. Here's the fxembed version:\n" + match.TransformedURL
		case "x":
			replyText = "X.com post detected. Here's the fxembed version:\n" + match.TransformedURL
		case "bluesky":
			replyText = "Bluesky post detected. Here's the fxembed version:\n" + match.TransformedURL
		}

		reply := tgbotapi.NewMessage(message.Chat.ID, replyText)
		reply.ReplyToMessageID = message.MessageID
		bot.Send(reply)
	}
}