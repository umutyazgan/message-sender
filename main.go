package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func uuidToString(b bson.Binary) (string, error) {
	if b.Subtype != 4 {
		return "", fmt.Errorf("unexpected UUID subtype: %d", b.Subtype)
	}
	if len(b.Data) != 16 {
		return "", fmt.Errorf("invalid UUID length: %d", len(b.Data))
	}
	u, err := uuid.FromBytes(b.Data)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

type message struct {
	ID          bson.Binary   `bson:"_id" json:"_id"`
	Content     string        `bson:"content" json:"content"`
	PhoneNumber string        `bson:"phoneNumber" json:"phoneNumber"`
	CreatedAt   bson.DateTime `bson:"createdAt" json:"createdAt"`
	Sent        bool          `bson:"sent" json:"sent"`
	SentAt      bson.DateTime `bson:"sentAt" json:"sentAt"`
}

type processedMessage struct {
	ID          string        `bson:"_id" json:"_id"`
	Content     string        `bson:"content" json:"content"`
	PhoneNumber string        `bson:"phoneNumber" json:"phoneNumber"`
	CreatedAt   bson.DateTime `bson:"createdAt" json:"createdAt"`
	SentAt      bson.DateTime `bson:"sentAt" json:"sentAt"`
}

func processMessage(m message) processedMessage {
	decodedId, err := uuidToString(m.ID)
	if err != nil {
		panic(err)
	}

	decodedM := processedMessage{
		decodedId,
		m.Content[:128], // Character limit
		m.PhoneNumber,
		m.CreatedAt,
		m.SentAt,
	}
	return decodedM
}

func fetchSentMessages() []processedMessage {
	coll := client.Database(os.Getenv("DBNAME")).Collection("messages") // TODO move to a config
	ctx := context.TODO()
	filter := bson.D{{Key: "sent", Value: true}}
	cursor, err := coll.Find(ctx, filter) // TODO DON'T DO THIS! CONSUME ITERATIVELY INSTEAD!
	if err != nil {
		panic(err)
	}
	defer cursor.Close(ctx)
	var results []message
	err = cursor.All(ctx, &results)
	if err != nil {
		panic(err)
	}
	var decodedResults []processedMessage
	for _, result := range results {
		decodedResults = append(decodedResults, processMessage(result))
	}
	return decodedResults
}

func getSentMessages(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, fetchSentMessages())
}

func togglePause(c *gin.Context) {
	paused.Store(!paused.Load())
	c.IndentedJSON(http.StatusOK, gin.H{
		"message": "Toggled pause",
	})
}

var paused atomic.Bool
var client *mongo.Client

func main() {
	// mongodb setup
	os.Getenv("MONGO_URI")
	uri := os.Getenv("MONGO_URI") + "?directConnection=true&serverSelectionTimeoutMS=2000" // TODO move options to a config file
	var err error
	client, err = mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		panic(err)
	}

	defer func() {
		if err := client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	// Worker loop
	go func() {
		for range ticker.C {
			if paused.Load() {
				continue // skip while paused
			}
			doWork()
		}
	}()

	router := gin.Default()
	router.GET("/messages", getSentMessages)
	router.POST("/toggle", togglePause)

	router.Run(":8080")
}

func doWork() {
	coll := client.Database(os.Getenv("DBNAME")).Collection("messages") // TODO move to a config
	ctx := context.TODO()
	filter := bson.D{{Key: "sent", Value: false}}
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}).SetLimit(2)
	cursor, err := coll.Find(ctx, filter, opts)
	if err != nil {
		panic(err)
	}
	var results []message
	err = cursor.All(ctx, &results)
	if err != nil {
		panic(err)
	}
	ids := make([]bson.Binary, 0, len(results))
	for _, result := range results {
		ids = append(ids, result.ID)
		decodedId, err := uuidToString(result.ID)
		if err != nil {
			panic(err)
		}
		decodedResult := processedMessage{
			decodedId,
			result.Content,
			result.PhoneNumber,
			result.CreatedAt,
			result.SentAt,
		}
		var buf bytes.Buffer
		err = json.NewEncoder(&buf).Encode(decodedResult)
		if err != nil {
			panic(err)
		}
		resp, err := http.Post(os.Getenv("WEBHOOK_URI"), "application/json", &buf)
		fmt.Printf("%+v\n", resp)
	}
	filter = bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: ids}}}}
	update := bson.D{
		{
			Key:   "$set",
			Value: bson.D{{Key: "sent", Value: true}},
		}, {
			Key:   "$currentDate",
			Value: bson.D{{Key: "sentAt", Value: true}},
		},
	}
	result, err := coll.UpdateMany(ctx, filter, update)
	fmt.Printf("%+v\n", result)
}
