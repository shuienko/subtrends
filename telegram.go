package main

import (
	"database/sql"
	"log"
	"os"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "github.com/mattn/go-sqlite3"
)

const (
	dbPath     = "data/subtrends.db"
	maxHistory = 10
)

type Bot struct {
	api    *tgbotapi.BotAPI
	db     *sql.DB
	logger *log.Logger
}

func NewBot(token string) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Create messages table if not exists with UNIQUE constraint
	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS messages (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            message_text TEXT NOT NULL UNIQUE,
            timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
        )
    `)
	if err != nil {
		return nil, err
	}

	logger := log.New(os.Stdout, "TelegramBot: ", log.LstdFlags)

	return &Bot{
		api:    api,
		db:     db,
		logger: logger,
	}, nil
}

func (b *Bot) saveMessage(text string) error {
	// Use INSERT OR REPLACE to handle duplicates
	_, err := b.db.Exec(`
        INSERT OR REPLACE INTO messages (message_text, timestamp) 
        VALUES (?, CURRENT_TIMESTAMP)
    `, text)
	if err != nil {
		return err
	}

	// Keep only last N unique messages
	_, err = b.db.Exec(`
        DELETE FROM messages 
        WHERE id NOT IN (
            SELECT id FROM messages 
            ORDER BY timestamp DESC 
            LIMIT ?
        )
    `, maxHistory)
	return err
}

func (b *Bot) getHistory() ([]string, error) {
	rows, err := b.db.Query(`
        SELECT message_text 
        FROM messages 
        ORDER BY timestamp DESC 
        LIMIT ?
    `, maxHistory)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []string
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			return nil, err
		}
		messages = append(messages, text)
	}
	return messages, nil
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

	if message.Text == "/history" {
		history, err := b.getHistory()
		if err != nil {
			return err
		}

		// Create inline keyboard with message history
		var buttons [][]tgbotapi.InlineKeyboardButton
		for _, msg := range history {
			button := tgbotapi.NewInlineKeyboardButtonData(
				msg[:min(40, len(msg))],
				msg,
			)
			buttons = append(buttons, []tgbotapi.InlineKeyboardButton{button})
		}

		keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)
		reply := tgbotapi.NewMessage(message.Chat.ID, "HISTORY:")
		reply.ReplyMarkup = keyboard
		_, err = b.api.Send(reply)
		return err
	}

	// Handle regular message
	token, err := getRedditAccessToken(&RateLimiter{})
	if err != nil {
		log.Fatalf("Failed to get access token: %v", err)
	}
	data, err := subredditData(message.Text, token)
	summary, err := summarizePosts(data)

	if err := b.saveMessage(message.Text); err != nil {
		return err
	}

	reply := tgbotapi.NewMessage(message.Chat.ID, summary)
	_, err = b.api.Send(reply)
	return err
}

func (b *Bot) handleCallback(callback *tgbotapi.CallbackQuery) error {
	// Get Reddit data using the callback data (subreddit name)
	token, err := getRedditAccessToken(&RateLimiter{})
	if err != nil {
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
