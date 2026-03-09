package models

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
