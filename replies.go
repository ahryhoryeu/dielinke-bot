package main

import (
	"context"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type mediaJob struct {
	message *tgbotapi.Message
	match   LinkMatch
}

type mediaWorker struct {
	jobs chan mediaJob
	done chan struct{}
}

func newMediaWorker(bot *tgbotapi.BotAPI, processor *mediaProcessor) *mediaWorker {
	w := &mediaWorker{jobs: make(chan mediaJob, 10), done: make(chan struct{})}
	go func() {
		defer close(w.done)
		for job := range w.jobs {
			if err := processor.send(context.Background(), bot, job.message, job.match.OriginalURL); err != nil {
				log.Printf("media failed chat=%d message=%d stage=%v", job.message.Chat.ID, job.message.MessageID, err)
				sendLinkReply(bot, job.message, job.match)
			}
		}
	}()
	return w
}

func (w *mediaWorker) close() { close(w.jobs); <-w.done }
func (w *mediaWorker) submit(job mediaJob) bool {
	select {
	case w.jobs <- job:
		return true
	default:
		return false
	}
}

func handleMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message, cfg config, worker *mediaWorker) {
	for _, match := range FindAndTransformLinks(message.Text) {
		if match.Type == "instagram" && cfg.InstagramReelsMode == "download" {
			if worker.submit(mediaJob{message: message, match: match}) {
				continue
			}
			log.Printf("media stage=queue_full chat=%d message=%d", message.Chat.ID, message.MessageID)
		}
		sendLinkReply(bot, message, match)
	}
}

func sendLinkReply(bot *tgbotapi.BotAPI, message *tgbotapi.Message, match LinkMatch) {
	var text string
	switch match.Type {
	case "instagram":
		text = "Instagram reel detected. Here's the kkclip version:\n"
	case "twitter":
		text = "Twitter post detected. Here's the fxembed version:\n"
	case "x":
		text = "X.com post detected. Here's the fxembed version:\n"
	case "bluesky":
		text = "Bluesky post detected. Here's the fxembed version:\n"
	}
	reply := tgbotapi.NewMessage(message.Chat.ID, text+match.TransformedURL)
	reply.ReplyToMessageID = message.MessageID
	if _, err := bot.Send(reply); err != nil {
		log.Printf("reply stage=link chat=%d message=%d: %v", message.Chat.ID, message.MessageID, err)
	}
}
