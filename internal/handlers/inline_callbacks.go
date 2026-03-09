package handlers

import (
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"telegram-team-bot/internal/db"
	"telegram-team-bot/internal/models"
	"telegram-team-bot/internal/utils"
)

// HandleInlineCallbackQuery processes inline mode buttons
func HandleInlineCallbackQuery(bot *tgbotapi.BotAPI, ds *db.DataStore, query *tgbotapi.CallbackQuery) {
	callbackData := query.Data
	userID := query.From.ID

	// Track the user
	user := models.User{
		ID:        userID,
		Username:  query.From.UserName,
		FirstName: query.From.FirstName,
	}

	callback := tgbotapi.NewCallback(query.ID, "")
	if _, err := bot.Request(callback); err != nil {
		fmt.Printf("Failed to acknowledge inline callback query: %v\n", err)
	}

	if strings.HasPrefix(callbackData, "join_inline:") {
		// Format: join_inline:creatorID:teamName
		parts := strings.SplitN(callbackData, ":", 3)
		if len(parts) != 3 {
			return
		}

		creatorID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return
		}

		teamName := parts[2]

		// Track user in the creator's namespace
		_ = ds.TrackUser(creatorID, user)

		// Create team if it doesn't exist
		team, _ := ds.GetTeam(creatorID, teamName)
		if team == nil {
			_ = ds.CreateTeam(creatorID, teamName)
			team, _ = ds.GetTeam(creatorID, teamName)
		}

		// Add user
		if team != nil {
			err = ds.AddMemberToTeam(creatorID, teamName, userID)

			// Optional: Alert the user they joined
			if err == nil {
				// Re-fetch members to update the count on the button
				team, _ = ds.GetTeam(creatorID, teamName)
				if team != nil {
					// Update the original inline message text to show member count
					text := fmt.Sprintf("Join the *%s* team\\! \\(%d members\\)", utils.EscapeMarkdownV2(teamName), len(team.Members))
					markup := tgbotapi.NewInlineKeyboardMarkup(
						tgbotapi.NewInlineKeyboardRow(
							tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("Join %s (%d)", teamName, len(team.Members)), callbackData),
						),
					)

					editMsg := tgbotapi.EditMessageTextConfig{
						BaseEdit: tgbotapi.BaseEdit{
							InlineMessageID: query.InlineMessageID,
							ReplyMarkup:     &markup,
						},
						Text:      text,
						ParseMode: tgbotapi.ModeMarkdownV2,
					}
					bot.Send(editMsg)

					alert := tgbotapi.NewCallbackWithAlert(query.ID, "You joined "+teamName+"!")
					bot.Request(alert)
				}
			} else {
				alert := tgbotapi.NewCallbackWithAlert(query.ID, "You are already in "+teamName+"!")
				bot.Request(alert)
			}
		}
	}
}
