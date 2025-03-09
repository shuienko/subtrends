package main

import (
	"log"
	"os"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api    *tgbotapi.BotAPI
	logger *log.Logger
}

func NewBot(token string) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	logger := log.New(os.Stdout, "TelegramBot: ", log.LstdFlags)

	return &Bot{
		api:    api,
		logger: logger,
	}, nil
}

func (b *Bot) handleMessage(message *tgbotapi.Message) error {
	// Get authorized user ID from environment variable
	authorizedUserIDStr := os.Getenv("AUTHORIZED_USER_ID")
	authorizedUserID, err := strconv.ParseInt(authorizedUserIDStr, 10, 64)
	if err != nil {
		return err
	}

	// Check if user is authorized
	if message.From.ID != authorizedUserID {
		reply := tgbotapi.NewMessage(message.Chat.ID, "Unauthorized user")
		_, err := b.api.Send(reply)
		return err
	}

	// Handle regular message
	token, err := getRedditAccessToken()
	if err != nil {
		log.Printf("Failed to get access token: %v", err)
		return err
	}
	data, err := subredditData(message.Text, token)
	if err != nil {
		return err
	}
	summary, err := summarizePosts(data)
	if err != nil {
		return err
	}

	reply := tgbotapi.NewMessage(message.Chat.ID, summary)
	_, err = b.api.Send(reply)
	return err
}

func (b *Bot) handleCallback(callback *tgbotapi.CallbackQuery) error {
	// Get Reddit data using the callback data (subreddit name)
	token, err := getRedditAccessToken()
	if err != nil {
		log.Printf("Failed to get access token: %v", err)
		return err
	}

	data, err := subredditData(callback.Data, token)
	if err != nil {
		return err
	}

	summary, err := summarizePosts(data)
	if err != nil {
		return err
	}

	// Send the summary to the user
	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, summary)
	_, err = b.api.Send(msg)
	return err
}
