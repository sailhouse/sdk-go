package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/sailhouse/sdk-go/sailhouse"
)

// This example demonstrates how to use wait groups to publish multiple events
// and wait for all of them to be processed as a group
func main() {
	// Get API key from environment
	apiKey := os.Getenv("SAILHOUSE_API_KEY")
	if apiKey == "" {
		log.Fatal("SAILHOUSE_API_KEY environment variable is required")
	}

	// Create a new client
	client := sailhouse.NewSailhouseClient(apiKey)

	// Define the events to publish in the wait group
	events := []sailhouse.WaitEvent{
		{
			Topic: "user-events",
			Body: map[string]interface{}{
				"type":    "user_created",
				"user_id": "user-123",
				"data": map[string]interface{}{
					"name":  "Example User",
					"email": "user@example.com",
				},
			},
			Metadata: map[string]interface{}{
				"source": "wait-group-example",
			},
		},
		{
			Topic: "notification-events",
			Body: map[string]interface{}{
				"type":    "email_notification",
				"user_id": "user-123",
				"data": map[string]interface{}{
					"template": "welcome_email",
					"to":       "user@example.com",
				},
			},
			Metadata: map[string]interface{}{
				"source": "wait-group-example",
			},
		},
	}

	// Define the wait group options
	ttl := "5m" // 5 minutes TTL

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Call Wait to create a wait group and publish all events
	err := client.Wait(ctx, "onboarding-flow", events, sailhouse.WithTTL(ttl))
	if err != nil {
		log.Fatalf("Failed to wait for events: %v", err)
	}

	fmt.Println("Successfully published all events in the wait group")
}