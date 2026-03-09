package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"

	"telegram-team-bot/internal/db"
	"telegram-team-bot/internal/handlers"
	"telegram-team-bot/internal/models"
)

func main() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file, relying on environment variables")
	}

	// Initialize MongoDB
	err = db.InitMongoDB()
	if err != nil {
		log.Fatalf("Database initialization failed: %v", err)
	}
	defer func() {
		if err := db.Disconnect(); err != nil {
			log.Fatalf("Error disconnecting from MongoDB: %v", err)
		}
	}()

	database := db.GetDB()
	dataStore := db.NewDataStore(database)

	err = dataStore.EnsureIndexes()
	if err != nil {
		log.Fatalf("Error ensuring database indexes: %v", err)
	}

	// Initialize Telegram Bot
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN environment variable is not set")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("Failed to initialize bot: %v", err)
	}

	bot.Debug = false // Set to true for detailed logs
	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	// State management for conversational flows (e.g., waiting for team name)
	// Key is fmt.Sprintf("%d:%d", chatID, userID)
	userStates := make(map[string]string)

	for update := range updates {
		if update.Message != nil {
			// Track user who sent the message
			if update.Message.Chat.IsGroup() || update.Message.Chat.IsSuperGroup() {
				user := models.User{
					ID:        update.Message.From.ID,
					Username:  update.Message.From.UserName,
					FirstName: update.Message.From.FirstName,
				}
				err := dataStore.TrackUser(update.Message.Chat.ID, user)
				if err != nil {
					log.Printf("Failed to track user %d in chat %d: %v", user.ID, update.Message.Chat.ID, err)
				}

				// Also track users joining the chat
				if len(update.Message.NewChatMembers) > 0 {
					for _, newMember := range update.Message.NewChatMembers {
						nu := models.User{
							ID:        newMember.ID,
							Username:  newMember.UserName,
							FirstName: newMember.FirstName,
						}
						err := dataStore.TrackUser(update.Message.Chat.ID, nu)
						if err != nil {
							log.Printf("Failed to track new member %d in chat %d: %v", newMember.ID, update.Message.Chat.ID, err)
						}
					}
				}
			}

			stateKey := fmt.Sprintf("%d:%d", update.Message.Chat.ID, update.Message.From.ID)

			// Handle commands
			if update.Message.IsCommand() {
				// If waiting for team name and they type /cancel, cancel the flow
				if state, exists := userStates[stateKey]; exists && state == "WAITING_FOR_TEAM_NAME" && update.Message.Command() == "cancel" {
					delete(userStates, stateKey)
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Team creation cancelled.")
					bot.Send(msg)
					continue
				}

				// Check if the command itself was typed as the team name
				if state, exists := userStates[stateKey]; exists && state == "WAITING_FOR_TEAM_NAME" {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Invalid team name. Team names cannot start with a slash (/) or be a command. Try again or type /cancel.")
					bot.Send(msg)
					continue
				}

				handlers.HandleCommand(bot, dataStore, update.Message)
				continue
			}

			// Handle user state (e.g., waiting for team name)
			if state, exists := userStates[stateKey]; exists && state == "WAITING_FOR_TEAM_NAME" {
				// We expect the message text to be the new team name
				teamName := strings.TrimSpace(update.Message.Text)

				// Basic validation
				if len(teamName) < 3 || strings.Contains(teamName, " ") || strings.HasPrefix(teamName, "/") {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Invalid team name. Must be at least 3 characters, contain no spaces, and not start with '/'. Try again or type /cancel.")
					bot.Send(msg)
					continue
				}

				err := dataStore.CreateTeam(update.Message.Chat.ID, teamName)
				if err != nil {
					log.Printf("Failed to create team %s: %v", teamName, err)
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Failed to create team. It might already exist.")
					bot.Send(msg)
				} else {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Team '"+teamName+"' created successfully!")
					bot.Send(msg)
				}

				// Clear state
				delete(userStates, stateKey)
				continue
			}
		}

		// Handle callback queries (button clicks)
		if update.CallbackQuery != nil {
			handlers.HandleCallbackQuery(bot, dataStore, update.CallbackQuery, userStates)
		}
	}
}
