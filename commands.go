package main

import (
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func isAdmin(bot *tgbotapi.BotAPI, chatID int64, userID int64) bool {
	config := tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			ChatID: chatID,
			UserID: userID,
		},
	}
	member, err := bot.GetChatMember(config)
	if err != nil {
		return false
	}
	return member.Status == "creator" || member.Status == "administrator"
}

func handleCommand(bot *tgbotapi.BotAPI, ds *DataStore, message *tgbotapi.Message) {
	command := message.Command()

	switch command {
	case "tagAll", "tagall":
		// Check if user is an admin
		if !isAdmin(bot, message.Chat.ID, message.From.ID) {
			msg := tgbotapi.NewMessage(message.Chat.ID, "Sorry, only administrators can use the /tagAll command.")
			msg.ReplyToMessageID = message.MessageID
			bot.Send(msg)
			return
		}

		members, err := ds.GetGroupMembers(message.Chat.ID)
		if err != nil {
			msg := tgbotapi.NewMessage(message.Chat.ID, "Error retrieving group members.")
			bot.Send(msg)
			return
		}

		if len(members) == 0 {
			msg := tgbotapi.NewMessage(message.Chat.ID, "No members tracked yet.")
			bot.Send(msg)
			return
		}

		var tags []string
		for _, member := range members {
			if member.Username != "" {
				tags = append(tags, "@"+member.Username)
			} else {
				// Use markdown v2 inline mention for users without username
				// Note: Requires parsing mode to be MarkdownV2
				firstNameEscaped := escapeMarkdownV2(member.FirstName)
				mention := fmt.Sprintf("[%s](tg://user?id=%d)", firstNameEscaped, member.ID)
				tags = append(tags, mention)
			}
		}

		// Send messages in batches of 5
		batchSize := 5
		for i := 0; i < len(tags); i += batchSize {
			end := i + batchSize
			if end > len(tags) {
				end = len(tags)
			}
			batch := tags[i:end]

			text := "Attention: " + strings.Join(batch, ", ")
			msg := tgbotapi.NewMessage(message.Chat.ID, text)
			msg.ParseMode = tgbotapi.ModeMarkdownV2
			bot.Send(msg)
		}

	case "manage":
		sendManageMenu(bot, message.Chat.ID)

	default:
		// Check if it's a dynamic team tag
		handleDynamicTeamTag(bot, ds, message)
	}
}

// sendManageMenu sends the inline keyboard for team management
func sendManageMenu(bot *tgbotapi.BotAPI, chatID int64) {
	// Send a message with an inline keyboard
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Create Team", "cmd_create_team"),
			tgbotapi.NewInlineKeyboardButtonData("Delete Team", "cmd_delete_team"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Join Team", "cmd_join_team"),
			tgbotapi.NewInlineKeyboardButtonData("Leave Team", "cmd_leave_team"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("List Teams", "cmd_list_teams"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, "Manage Teams:")
	msg.ReplyMarkup = keyboard

	bot.Send(msg)
}

// Helper to escape special characters for MarkdownV2
func escapeMarkdownV2(text string) string {
	specialChars := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	for _, char := range specialChars {
		text = strings.ReplaceAll(text, char, "\\"+char)
	}
	return text
}
