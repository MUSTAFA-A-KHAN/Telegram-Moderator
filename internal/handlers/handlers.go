package handlers

import (
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"telegram-team-bot/internal/db"
	"telegram-team-bot/internal/utils"
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

// HandleCommand processes standard bot commands
func HandleCommand(bot *tgbotapi.BotAPI, ds *db.DataStore, message *tgbotapi.Message) {
	command := message.Command()

	switch command {
	case "tagAll", "tagall":
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
				firstNameEscaped := utils.EscapeMarkdownV2(member.FirstName)
				mention := fmt.Sprintf("[%s](tg://user?id=%d)", firstNameEscaped, member.ID)
				tags = append(tags, mention)
			}
		}

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
		HandleDynamicTeamTag(bot, ds, message)
	}
}

func sendManageMenu(bot *tgbotapi.BotAPI, chatID int64) {
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

// HandleCallbackQuery processes inline keyboard interactions
func HandleCallbackQuery(bot *tgbotapi.BotAPI, ds *db.DataStore, query *tgbotapi.CallbackQuery, userStates map[string]string) {
	callbackData := query.Data
	chatID := query.Message.Chat.ID
	userID := query.From.ID

	callback := tgbotapi.NewCallback(query.ID, "")
	if _, err := bot.Request(callback); err != nil {
		fmt.Printf("Failed to acknowledge callback query: %v\n", err)
	}

	switch {
	case callbackData == "cmd_create_team":
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
			response += fmt.Sprintf("- *%s* (%d members)\n", utils.EscapeMarkdownV2(team.TeamName), len(team.Members))
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

// HandleDynamicTeamTag processes ad-hoc tags like /werewolfTeam
func HandleDynamicTeamTag(bot *tgbotapi.BotAPI, ds *db.DataStore, message *tgbotapi.Message) {
	command := strings.TrimPrefix(message.Text, "/")
	if idx := strings.Index(command, "@"); idx != -1 {
		command = command[:idx]
	}

	parts := strings.Fields(command)
	if len(parts) == 0 {
		return
	}

	teamName := parts[0]

	team, err := ds.GetTeam(message.Chat.ID, teamName)
	if err != nil || team == nil {
		return
	}

	if len(team.Members) == 0 {
		msg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("The %s team has no members yet.", team.TeamName))
		bot.Send(msg)
		return
	}

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
				firstNameEscaped := utils.EscapeMarkdownV2(member.FirstName)
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

	batchSize := 5
	for i := 0; i < len(tags); i += batchSize {
		end := i + batchSize
		if end > len(tags) {
			end = len(tags)
		}
		batch := tags[i:end]

		text := fmt.Sprintf("Calling team *%s*:\n%s", utils.EscapeMarkdownV2(team.TeamName), strings.Join(batch, ", "))
		msg := tgbotapi.NewMessage(message.Chat.ID, text)
		msg.ParseMode = tgbotapi.ModeMarkdownV2
		bot.Send(msg)
	}
}
