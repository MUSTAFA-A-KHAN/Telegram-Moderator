package handlers

import (
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"telegram-team-bot/internal/db"
	"telegram-team-bot/internal/models"
	"telegram-team-bot/internal/utils"
)

// HandleInlineQuery processes inline mode queries
func HandleInlineQuery(bot *tgbotapi.BotAPI, ds *db.DataStore, query *tgbotapi.InlineQuery) {
	text := strings.TrimSpace(query.Query)
	creatorID := query.From.ID

	// Track the creator just in case
	creatorUser := models.User{
		ID:        query.From.ID,
		Username:  query.From.UserName,
		FirstName: query.From.FirstName,
	}
	// Store in a special "global/inline" group namespace (0) or just creator namespace
	_ = ds.TrackUser(creatorID, creatorUser)

	var results []interface{}

	if text == "" {
		// Just show their existing teams they created
		teams, _ := ds.GetTeams(creatorID)
		for _, t := range teams {
			result := buildSendTagsResult(ds, creatorID, t)
			results = append(results, result)
		}
	} else {
		// Search for a team or propose to create one
		teamName := strings.Split(text, " ")[0]
		team, _ := ds.GetTeam(creatorID, teamName)

		if team != nil {
			// Found the team, allow them to send the tags
			results = append(results, buildSendTagsResult(ds, creatorID, *team))
		} else {
			// Not found, allow them to create a shareable Join button
			createBtn := tgbotapi.NewInlineQueryResultArticle(query.ID, "Create & Share Team: "+teamName, fmt.Sprintf("Join the *%s* team!", utils.EscapeMarkdownV2(teamName)))
			createBtn.Description = "Send a button to let people join " + teamName

			// Setup the inline keyboard that will be attached to the message
			btn := tgbotapi.NewInlineKeyboardButtonData("Join "+teamName, fmt.Sprintf("join_inline:%d:%s", creatorID, teamName))
			markup := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btn))
			createBtn.ReplyMarkup = &markup

			// Needs parse mode for the text
			createBtn.InputMessageContent = tgbotapi.InputTextMessageContent{
				Text:      fmt.Sprintf("Join the *%s* team!", utils.EscapeMarkdownV2(teamName)),
				ParseMode: tgbotapi.ModeMarkdownV2,
			}

			results = append(results, createBtn)
		}
	}

	inlineConf := tgbotapi.InlineConfig{
		InlineQueryID: query.ID,
		IsPersonal:    true,
		CacheTime:     0,
		Results:       results,
	}

	if _, err := bot.Request(inlineConf); err != nil {
		fmt.Printf("Failed to answer inline query: %v\n", err)
	}
}

func buildSendTagsResult(ds *db.DataStore, creatorID int64, team models.Team) tgbotapi.InlineQueryResultArticle {
	title := "Send tags for " + team.TeamName
	desc := fmt.Sprintf("%d members", len(team.Members))

	groupMembers, _ := ds.GetGroupMembers(creatorID)

	var tags []string
	for _, memberID := range team.Members {
		if member, exists := groupMembers[memberID]; exists {
			firstNameEscaped := utils.EscapeMarkdownV2(member.FirstName)
			mention := fmt.Sprintf("[%s](tg://user?id=%d)", firstNameEscaped, member.ID)
			tags = append(tags, mention)
		}
	}

	var text string
	if len(tags) == 0 {
		text = fmt.Sprintf("The *%s* team is empty.", utils.EscapeMarkdownV2(team.TeamName))
	} else {
		text = fmt.Sprintf("Calling team *%s*\\:\n%s", utils.EscapeMarkdownV2(team.TeamName), strings.Join(tags, ", "))
	}

	res := tgbotapi.NewInlineQueryResultArticle(team.TeamName, title, text)
	res.Description = desc
	res.InputMessageContent = tgbotapi.InputTextMessageContent{
		Text:      text,
		ParseMode: tgbotapi.ModeMarkdownV2,
	}
	return res
}
