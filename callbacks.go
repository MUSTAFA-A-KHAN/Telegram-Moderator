package main

import (
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func handleCallbackQuery(bot *tgbotapi.BotAPI, ds *DataStore, query *tgbotapi.CallbackQuery, userStates map[string]string) {
	callbackData := query.Data
	chatID := query.Message.Chat.ID
	userID := query.From.ID

	// Always acknowledge the callback query
	callback := tgbotapi.NewCallback(query.ID, "")
	if _, err := bot.Request(callback); err != nil {
		fmt.Printf("Failed to acknowledge callback query: %v\n", err)
	}

	switch {
	case callbackData == "cmd_create_team":
		// Set user state to waiting for team name
		stateKey := fmt.Sprintf("%d:%d", chatID, userID)
		userStates[stateKey] = "WAITING_FOR_TEAM_NAME"
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("@%s, please type the new team name (no spaces, e.g. werewolfTeam):", query.From.UserName))
		bot.Send(msg)

	case callbackData == "cmd_delete_team":
		teams, err := ds.GetTeams(chatID)
		if err != nil || len(teams) == 0 {
			bot.Send(tgbotapi.NewMessage(chatID, "No teams found to delete."))
			return
		}

		var keyboard [][]tgbotapi.InlineKeyboardButton
		for _, team := range teams {
			btn := tgbotapi.NewInlineKeyboardButtonData(team.TeamName, "delete_team:"+team.TeamName)
			keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(btn))
		}

		msg := tgbotapi.NewMessage(chatID, "Select a team to delete:")
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
		bot.Send(msg)

	case callbackData == "cmd_join_team":
		teams, err := ds.GetTeams(chatID)
		if err != nil || len(teams) == 0 {
			bot.Send(tgbotapi.NewMessage(chatID, "No teams available to join."))
			return
		}

		var keyboard [][]tgbotapi.InlineKeyboardButton
		for _, team := range teams {
			btn := tgbotapi.NewInlineKeyboardButtonData(team.TeamName, "join_team:"+team.TeamName)
			keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(btn))
		}

		msg := tgbotapi.NewMessage(chatID, "Select a team to join:")
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
		bot.Send(msg)

	case callbackData == "cmd_leave_team":
		teams, err := ds.GetTeams(chatID)
		if err != nil || len(teams) == 0 {
			bot.Send(tgbotapi.NewMessage(chatID, "No teams available."))
			return
		}

		var keyboard [][]tgbotapi.InlineKeyboardButton
		for _, team := range teams {
			// Check if user is in this team
			inTeam := false
			for _, memberID := range team.Members {
				if memberID == userID {
					inTeam = true
					break
				}
			}
			if inTeam {
				btn := tgbotapi.NewInlineKeyboardButtonData(team.TeamName, "leave_team:"+team.TeamName)
				keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(btn))
			}
		}

		if len(keyboard) == 0 {
			bot.Send(tgbotapi.NewMessage(chatID, "You are not in any teams."))
			return
		}

		msg := tgbotapi.NewMessage(chatID, "Select a team to leave:")
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
		bot.Send(msg)

	case callbackData == "cmd_list_teams":
		teams, err := ds.GetTeams(chatID)
		if err != nil || len(teams) == 0 {
			bot.Send(tgbotapi.NewMessage(chatID, "No teams exist yet."))
			return
		}

		var response string
		for _, team := range teams {
			response += fmt.Sprintf("- *%s* (%d members)\n", escapeMarkdownV2(team.TeamName), len(team.Members))
		}

		msg := tgbotapi.NewMessage(chatID, "Teams in this group:\n"+response)
		msg.ParseMode = tgbotapi.ModeMarkdownV2
		bot.Send(msg)

	case strings.HasPrefix(callbackData, "delete_team:"):
		teamName := strings.TrimPrefix(callbackData, "delete_team:")
		err := ds.DeleteTeam(chatID, teamName)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "Failed to delete team."))
		} else {
			bot.Send(tgbotapi.NewMessage(chatID, "Team "+teamName+" deleted successfully."))
		}

	case strings.HasPrefix(callbackData, "join_team:"):
		teamName := strings.TrimPrefix(callbackData, "join_team:")
		err := ds.AddMemberToTeam(chatID, teamName, userID)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "Failed to join team. You might already be in it."))
		} else {
			bot.Send(tgbotapi.NewMessage(chatID, "You have joined "+teamName+"!"))
		}

	case strings.HasPrefix(callbackData, "leave_team:"):
		teamName := strings.TrimPrefix(callbackData, "leave_team:")
		err := ds.RemoveMemberFromTeam(chatID, teamName, userID)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "Failed to leave team."))
		} else {
			bot.Send(tgbotapi.NewMessage(chatID, "You have left "+teamName+"."))
		}
	}
}
