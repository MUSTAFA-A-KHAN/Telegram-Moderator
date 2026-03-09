package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"telegram-team-bot/internal/models"
)

var (
	mongoClient *mongo.Client
	dbName      = "telegram_team_bot"
)

// InitMongoDB initializes the connection to MongoDB
func InitMongoDB() error {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		return fmt.Errorf("MONGO_URI environment variable is not set")
	}

	clientOptions := options.Client().ApplyURI(uri)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	mongoClient = client
	log.Println("Connected to MongoDB successfully!")
	return nil
}

// GetDB returns the database instance
func GetDB() *mongo.Database {
	return mongoClient.Database(dbName)
}

// Disconnect cleans up the MongoDB connection
func Disconnect() error {
	if mongoClient != nil {
		return mongoClient.Disconnect(context.Background())
	}
	return nil
}

// DataStore handles database operations
type DataStore struct {
	db *mongo.Database
}

// NewDataStore creates a new DataStore
func NewDataStore(db *mongo.Database) *DataStore {
	return &DataStore{db: db}
}

// EnsureIndexes creates necessary indexes on collections
func (ds *DataStore) EnsureIndexes() error {
	ctx := context.Background()

	teamsColl := ds.db.Collection("teams")
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "chat_id", Value: 1},
			{Key: "team_name", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}
	_, err := teamsColl.Indexes().CreateOne(ctx, indexModel)
	return err
}

// TrackUser adds or updates a user in the group's tracked members list
func (ds *DataStore) TrackUser(chatID int64, user models.User) error {
	ctx := context.Background()
	coll := ds.db.Collection("group_members")

	filter := bson.M{"_id": chatID}
	update := bson.M{
		"$set": bson.M{
			fmt.Sprintf("members.%d", user.ID): user,
		},
	}
	opts := options.Update().SetUpsert(true)

	_, err := coll.UpdateOne(ctx, filter, update, opts)
	return err
}

// GetGroupMembers retrieves all tracked members for a group
func (ds *DataStore) GetGroupMembers(chatID int64) (map[int64]models.User, error) {
	ctx := context.Background()
	coll := ds.db.Collection("group_members")

	var group models.GroupMembers
	err := coll.FindOne(ctx, bson.M{"_id": chatID}).Decode(&group)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return make(map[int64]models.User), nil
		}
		return nil, err
	}

	if group.Members == nil {
		return make(map[int64]models.User), nil
	}
	return group.Members, nil
}

// CreateTeam creates a new team in the chat
func (ds *DataStore) CreateTeam(chatID int64, teamName string) error {
	ctx := context.Background()
	coll := ds.db.Collection("teams")

	teamNameLower := strings.ToLower(teamName)

	team := models.Team{
		ChatID:   chatID,
		TeamName: teamNameLower,
		Members:  []int64{},
	}

	_, err := coll.InsertOne(ctx, team)
	return err
}

// DeleteTeam removes a team from the chat
func (ds *DataStore) DeleteTeam(chatID int64, teamName string) error {
	ctx := context.Background()
	coll := ds.db.Collection("teams")

	teamNameLower := strings.ToLower(teamName)

	_, err := coll.DeleteOne(ctx, bson.M{"chat_id": chatID, "team_name": teamNameLower})
	return err
}

// GetTeams retrieves all teams for a chat
func (ds *DataStore) GetTeams(chatID int64) ([]models.Team, error) {
	ctx := context.Background()
	coll := ds.db.Collection("teams")

	cursor, err := coll.Find(ctx, bson.M{"chat_id": chatID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var teams []models.Team
	if err := cursor.All(ctx, &teams); err != nil {
		return nil, err
	}
	return teams, nil
}

// GetTeam retrieves a specific team in a chat
func (ds *DataStore) GetTeam(chatID int64, teamName string) (*models.Team, error) {
	ctx := context.Background()
	coll := ds.db.Collection("teams")

	teamNameLower := strings.ToLower(teamName)

	var team models.Team
	err := coll.FindOne(ctx, bson.M{"chat_id": chatID, "team_name": teamNameLower}).Decode(&team)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &team, nil
}

// AddMemberToTeam adds a user to a team
func (ds *DataStore) AddMemberToTeam(chatID int64, teamName string, userID int64) error {
	ctx := context.Background()
	coll := ds.db.Collection("teams")

	teamNameLower := strings.ToLower(teamName)

	filter := bson.M{"chat_id": chatID, "team_name": teamNameLower}
	update := bson.M{"$addToSet": bson.M{"members": userID}}

	result, err := coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("team not found")
	}
	return nil
}

// RemoveMemberFromTeam removes a user from a team
func (ds *DataStore) RemoveMemberFromTeam(chatID int64, teamName string, userID int64) error {
	ctx := context.Background()
	coll := ds.db.Collection("teams")

	teamNameLower := strings.ToLower(teamName)

	filter := bson.M{"chat_id": chatID, "team_name": teamNameLower}
	update := bson.M{"$pull": bson.M{"members": userID}}

	result, err := coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("team not found")
	}
	return nil
}
