package main

import (
	"log"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	// Get token
	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN environment variable is not set")
	}

	// Create bot instance
	bot, err := NewBot(token)
	if err != nil {
		log.Fatal(err)
	}
	defer bot.db.Close()

	// Start bot
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30
	updates := bot.api.GetUpdatesChan(updateConfig)

	bot.logger.Println("Bot started")

	// Handle messages and queries
	for update := range updates {
		if update.Message != nil {
			if err := bot.handleMessage(update.Message); err != nil {
				bot.logger.Printf("Error handling message: %v", err)
			}
		} else if update.CallbackQuery != nil {
			if err := bot.handleCallback(update.CallbackQuery); err != nil {
				bot.logger.Printf("Error handling callback: %v", err)
			}
		}
	}
}
