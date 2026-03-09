package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
