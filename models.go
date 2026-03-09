package main

import (
	"context"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// User represents a Telegram user inside a group
type User struct {
	ID        int64  `bson:"_id"` // Telegram User ID
	Username  string `bson:"username"`
	FirstName string `bson:"first_name"`
}

// GroupMembers represents all tracked users in a specific group
type GroupMembers struct {
	ChatID  int64          `bson:"_id"` // Telegram Chat ID
	Members map[int64]User `bson:"members"`
}

// Team represents a sub-group (team) within a main chat
type Team struct {
	ChatID   int64   `bson:"chat_id"`
	TeamName string  `bson:"team_name"`
	Members  []int64 `bson:"members"` // List of User IDs
}

// DataStore handles database operations
type DataStore struct {
	db *mongo.Database
}

func NewDataStore(db *mongo.Database) *DataStore {
	return &DataStore{db: db}
}

// EnsureIndexes creates necessary indexes on collections
func (ds *DataStore) EnsureIndexes() error {
	ctx := context.Background()

	// Ensure unique index on Teams by chat_id and team_name
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
func (ds *DataStore) TrackUser(chatID int64, user User) error {
	ctx := context.Background()
	coll := ds.db.Collection("group_members")

	// Update the specific user within the members map for the chat
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
func (ds *DataStore) GetGroupMembers(chatID int64) (map[int64]User, error) {
	ctx := context.Background()
	coll := ds.db.Collection("group_members")

	var group GroupMembers
	err := coll.FindOne(ctx, bson.M{"_id": chatID}).Decode(&group)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return make(map[int64]User), nil
		}
		return nil, err
	}

	if group.Members == nil {
		return make(map[int64]User), nil
	}
	return group.Members, nil
}

// CreateTeam creates a new team in the chat
func (ds *DataStore) CreateTeam(chatID int64, teamName string) error {
	ctx := context.Background()
	coll := ds.db.Collection("teams")

	teamNameLower := strings.ToLower(teamName)

	team := Team{
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
func (ds *DataStore) GetTeams(chatID int64) ([]Team, error) {
	ctx := context.Background()
	coll := ds.db.Collection("teams")

	cursor, err := coll.Find(ctx, bson.M{"chat_id": chatID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var teams []Team
	if err := cursor.All(ctx, &teams); err != nil {
		return nil, err
	}
	return teams, nil
}

// GetTeam retrieves a specific team in a chat
func (ds *DataStore) GetTeam(chatID int64, teamName string) (*Team, error) {
	ctx := context.Background()
	coll := ds.db.Collection("teams")

	teamNameLower := strings.ToLower(teamName)

	var team Team
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
