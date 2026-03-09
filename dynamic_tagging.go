package main

import (
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func handleDynamicTeamTag(bot *tgbotapi.BotAPI, ds *DataStore, message *tgbotapi.Message) {
	// e.g. /werewolfTeam -> teamName = "werewolfTeam"
	// Also strip possible bot username e.g. /werewolfTeam@MyBotName

	command := strings.TrimPrefix(message.Text, "/")
	// Remove bot username if it exists
	if idx := strings.Index(command, "@"); idx != -1 {
		command = command[:idx]
	}

	// Sometimes text can have other words, so just get the first word.
	// But in Telegram, commands are clickable if they start with slash and no spaces.
	// We'll split by space and take the first item
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return
	}

	teamName := parts[0]

	team, err := ds.GetTeam(message.Chat.ID, teamName)
	if err != nil || team == nil {
		// Not a tracked team, ignore to not conflict with other bot commands
		return
	}

	if len(team.Members) == 0 {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("The %s team has no members yet.", team.TeamName))
		bot.Send(msg)
		return
	}

	// Fetch members info
	groupMembers, err := ds.GetGroupMembers(message.Chat.ID)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Error retrieving group members.")
		bot.Send(msg)
		return
	}

	var tags []string
	for _, memberID := range team.Members {
		if member, exists := groupMembers[memberID]; exists {
			if member.Username != "" {
				tags = append(tags, "@"+member.Username)
			} else {
				// MarkdownV2 mention
				firstNameEscaped := escapeMarkdownV2(member.FirstName)
				mention := fmt.Sprintf("[%s](tg://user?id=%d)", firstNameEscaped, member.ID)
				tags = append(tags, mention)
			}
		}
	}

	if len(tags) == 0 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "None of the team members are currently tracked in this group.")
		bot.Send(msg)
		return
	}

	// Send messages in batches of 5 to comply with Telegram limits
	batchSize := 5
	for i := 0; i < len(tags); i += batchSize {
		end := i + batchSize
		if end > len(tags) {
			end = len(tags)
		}
		batch := tags[i:end]

		text := fmt.Sprintf("Calling team *%s*:\n%s", escapeMarkdownV2(team.TeamName), strings.Join(batch, ", "))
		msg := tgbotapi.NewMessage(message.Chat.ID, text)
		msg.ParseMode = tgbotapi.ModeMarkdownV2
		bot.Send(msg)
	}
}
