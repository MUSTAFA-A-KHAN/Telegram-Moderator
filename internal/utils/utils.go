package utils

import (
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// EscapeMarkdownV2 helper to escape special characters for MarkdownV2
func EscapeMarkdownV2(text string) string {
	specialChars := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	for _, char := range specialChars {
		text = strings.ReplaceAll(text, char, "\\"+char)
	}
	return text
}

// DeleteMessageAfter sends a message and deletes it after the specified duration
func DeleteMessageAfter(bot *tgbotapi.BotAPI, chatID int64, messageID int, d time.Duration) {
	go func() {
		time.Sleep(d)
		delMsg := tgbotapi.NewDeleteMessage(chatID, messageID)
		bot.Send(delMsg)
	}()
}
