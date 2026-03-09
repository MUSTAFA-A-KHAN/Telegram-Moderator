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

	teams, _ := ds.GetTeams(creatorID)

	if text == "" {
		// Just show their existing teams they created
		for _, t := range teams {
			resList := buildSendTagsResult(ds, creatorID, t)
			results = append(results, resList...)
		}
	} else {
		// Search for existing teams matching the prefix
		teamName := strings.Split(text, " ")[0]
		exactMatchFound := false

		for _, t := range teams {
			if strings.HasPrefix(strings.ToLower(t.TeamName), strings.ToLower(teamName)) {
				if strings.ToLower(t.TeamName) == strings.ToLower(teamName) {
					exactMatchFound = true
				}
				resList := buildSendTagsResult(ds, creatorID, t)
				results = append(results, resList...)
			}
		}

		// Only propose to create a team if the name is valid (>= 3 chars) and an exact match wasn't found
		if len(teamName) >= 3 && !exactMatchFound {
			createBtn := tgbotapi.NewInlineQueryResultArticle(query.ID, "Create & Share Team: "+teamName, fmt.Sprintf("Join the *%s* team\\!", utils.EscapeMarkdownV2(teamName)))
			createBtn.Description = "Send a button to let people join " + teamName

			// Setup the inline keyboard that will be attached to the message
			btn := tgbotapi.NewInlineKeyboardButtonData("Join "+teamName, fmt.Sprintf("join_inline:%d:%s", creatorID, teamName))
			markup := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btn))
			createBtn.ReplyMarkup = &markup

			// Needs parse mode for the text
			createBtn.InputMessageContent = tgbotapi.InputTextMessageContent{
				Text:      fmt.Sprintf("Join the *%s* team\\!", utils.EscapeMarkdownV2(teamName)),
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

func buildSendTagsResult(ds *db.DataStore, creatorID int64, team models.Team) []interface{} {
	groupMembers, _ := ds.GetGroupMembers(creatorID)

	var tags []string
	for _, memberID := range team.Members {
		if member, exists := groupMembers[memberID]; exists {
			firstNameEscaped := utils.EscapeMarkdownV2(member.FirstName)
			mention := fmt.Sprintf("[%s](tg://user?id=%d)", firstNameEscaped, member.ID)
			tags = append(tags, mention)
		}
	}

	if len(tags) == 0 {
		title := "Send tags for " + team.TeamName
		text := fmt.Sprintf("The *%s* team is empty.", utils.EscapeMarkdownV2(team.TeamName))
		res := tgbotapi.NewInlineQueryResultArticle(team.TeamName+"_empty", title, text)
		res.Description = "0 members"
		res.InputMessageContent = tgbotapi.InputTextMessageContent{
			Text:      text,
			ParseMode: tgbotapi.ModeMarkdownV2,
		}
		return []interface{}{res}
	}

	// Telegram limits mentions to ~5 per message to prevent spam blocking
	batchSize := 5
	var results []interface{}
	partNum := 1

	for i := 0; i < len(tags); i += batchSize {
		end := i + batchSize
		if end > len(tags) {
			end = len(tags)
		}
		batch := tags[i:end]

		text := fmt.Sprintf("Calling team *%s*\\:\n%s", utils.EscapeMarkdownV2(team.TeamName), strings.Join(batch, ", "))

		title := fmt.Sprintf("Send tags for %s", team.TeamName)
		if len(tags) > batchSize {
			title = fmt.Sprintf("Send tags for %s (Part %d)", team.TeamName, partNum)
		}

		res := tgbotapi.NewInlineQueryResultArticle(fmt.Sprintf("%s_part%d", team.TeamName, partNum), title, text)
		res.Description = fmt.Sprintf("Tags %d members", len(batch))
		res.InputMessageContent = tgbotapi.InputTextMessageContent{
			Text:      text,
			ParseMode: tgbotapi.ModeMarkdownV2,
		}
		results = append(results, res)
		partNum++
	}

	return results
}
