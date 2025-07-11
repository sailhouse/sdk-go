package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/sailhouse/sdk-go/sailhouse"
)

// Stats for tracking processing
type ProcessingStats struct {
	eventsProcessed  atomic.Int64
	processingErrors atomic.Int64
	handlerErrors    atomic.Int64
}

func (s *ProcessingStats) String() string {
	return fmt.Sprintf("Events: %d, Processing Errors: %d, Handler Errors: %d",
		s.eventsProcessed.Load(),
		s.processingErrors.Load(),
		s.handlerErrors.Load())
}

func main() {
	// Parse command line flags
	token := flag.String("token", "", "Sailhouse API token")
	processors := flag.Int("processors", 2, "Number of processors per subscription")
	flag.Parse()

	if *token == "" {
		log.Fatal("Usage: go run main.go -token=<token> [-processors=<count>]")
	}

	// Create context that cancels on interrupt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create Sailhouse client
	client := sailhouse.NewSailhouseClient(*token)

	// Create processing stats
	stats := &ProcessingStats{}

	// Create subscriber with custom options
	options := &sailhouse.SubscriberOptions{
		PerSubscriptionProcessors: *processors,
		PollInterval:              500 * time.Millisecond,
		MaxRetries:                3,
		RetryDelay:                2 * time.Second,
		ErrorHandler: func(err error) {
			stats.processingErrors.Add(1)
			log.Printf("Processing error: %v", err)
		},
	}

	subscriber := client.NewSubscriber(options)

	// Register subscription handlers
	registerHandlers(subscriber, stats)

	// Start the subscriber
	fmt.Printf("Starting subscriber with %d processors per subscription...\n", *processors)
	err := subscriber.Start(ctx)
	if err != nil {
		log.Fatalf("Failed to start subscriber: %v", err)
	}

	fmt.Println("Subscriber started successfully!")
	fmt.Println("Registered subscriptions:")
	for _, sub := range subscriber.GetSubscriptions() {
		fmt.Printf("  - %s/%s\n", sub.Topic, sub.Subscription)
	}
	fmt.Println()

	// Start stats reporting
	go reportStats(ctx, stats)

	// Wait for shutdown signal
	select {
	case sig := <-sigChan:
		fmt.Printf("\nReceived signal %v, shutting down gracefully...\n", sig)
	case <-ctx.Done():
		fmt.Println("\nContext cancelled, shutting down...")
	}

	// Graceful shutdown
	fmt.Println("Stopping subscriber...")
	subscriber.Stop()
	fmt.Println("Subscriber stopped.")
	fmt.Printf("Final stats: %s\n", stats)
}

func registerHandlers(subscriber *sailhouse.SailhouseSubscriber, stats *ProcessingStats) {
	// Handler for user events
	subscriber.Subscribe("user-events", "user-processor", func(ctx context.Context, event *sailhouse.Event) error {
		stats.eventsProcessed.Add(1)

		fmt.Printf("[USER] Processing event %s\n", event.ID)

		// Parse event data
		var eventData map[string]interface{}
		err := event.As(&eventData)
		if err != nil {
			stats.handlerErrors.Add(1)
			return fmt.Errorf("failed to parse event data: %w", err)
		}

		// Handle different user event types
		eventType, ok := eventData["type"].(string)
		if !ok {
			stats.handlerErrors.Add(1)
			return fmt.Errorf("missing or invalid event type")
		}

		switch eventType {
		case "user.created":
			return handleUserCreated(eventData, event.Metadata)
		case "user.updated":
			return handleUserUpdated(eventData, event.Metadata)
		case "user.deleted":
			return handleUserDeleted(eventData, event.Metadata)
		default:
			fmt.Printf("[USER] Unknown event type: %s\n", eventType)
			return nil // Don't fail on unknown events
		}
	})

	// Handler for order events
	subscriber.Subscribe("order-events", "order-processor", func(ctx context.Context, event *sailhouse.Event) error {
		stats.eventsProcessed.Add(1)

		fmt.Printf("[ORDER] Processing event %s\n", event.ID)

		var eventData map[string]interface{}
		err := event.As(&eventData)
		if err != nil {
			stats.handlerErrors.Add(1)
			return fmt.Errorf("failed to parse event data: %w", err)
		}

		eventType, ok := eventData["type"].(string)
		if !ok {
			stats.handlerErrors.Add(1)
			return fmt.Errorf("missing or invalid event type")
		}

		switch eventType {
		case "order.created":
			return handleOrderCreated(eventData, event.Metadata)
		case "order.paid":
			return handleOrderPaid(eventData, event.Metadata)
		case "order.shipped":
			return handleOrderShipped(eventData, event.Metadata)
		case "order.delivered":
			return handleOrderDelivered(eventData, event.Metadata)
		default:
			fmt.Printf("[ORDER] Unknown event type: %s\n", eventType)
			return nil
		}
	})

	// Handler for notification events (demonstrates error handling)
	subscriber.Subscribe("notifications", "notification-processor", func(ctx context.Context, event *sailhouse.Event) error {
		stats.eventsProcessed.Add(1)

		fmt.Printf("[NOTIFICATION] Processing event %s\n", event.ID)

		var eventData map[string]interface{}
		err := event.As(&eventData)
		if err != nil {
			stats.handlerErrors.Add(1)
			return fmt.Errorf("failed to parse event data: %w", err)
		}

		// Simulate occasional failures for demonstration
		if priority, ok := eventData["priority"].(string); ok && priority == "test-failure" {
			stats.handlerErrors.Add(1)
			return fmt.Errorf("simulated processing failure for testing")
		}

		return handleNotification(eventData, event.Metadata)
	})
}

// Event handlers

func handleUserCreated(data map[string]interface{}, metadata map[string]interface{}) error {
	userID := data["user_id"]
	email := data["email"]
	fmt.Printf("    → New user created: ID=%v, Email=%v\n", userID, email)

	// Simulate processing time
	time.Sleep(50 * time.Millisecond)

	// Your business logic here:
	// - Send welcome email
	// - Update analytics
	// - Sync with CRM

	return nil
}

func handleUserUpdated(data map[string]interface{}, metadata map[string]interface{}) error {
	userID := data["user_id"]
	fmt.Printf("    → User updated: ID=%v\n", userID)

	time.Sleep(30 * time.Millisecond)

	// Your business logic here:
	// - Update search index
	// - Sync with external services

	return nil
}

func handleUserDeleted(data map[string]interface{}, metadata map[string]interface{}) error {
	userID := data["user_id"]
	fmt.Printf("    → User deleted: ID=%v\n", userID)

	time.Sleep(40 * time.Millisecond)

	// Your business logic here:
	// - Clean up user data
	// - Cancel subscriptions
	// - Update analytics

	return nil
}

func handleOrderCreated(data map[string]interface{}, metadata map[string]interface{}) error {
	orderID := data["order_id"]
	amount := data["amount"]
	fmt.Printf("    → Order created: ID=%v, Amount=%v\n", orderID, amount)

	time.Sleep(100 * time.Millisecond)

	// Your business logic here:
	// - Validate order
	// - Reserve inventory
	// - Send confirmation email

	return nil
}

func handleOrderPaid(data map[string]interface{}, metadata map[string]interface{}) error {
	orderID := data["order_id"]
	fmt.Printf("    → Order paid: ID=%v\n", orderID)

	time.Sleep(75 * time.Millisecond)

	// Your business logic here:
	// - Process payment
	// - Update order status
	// - Trigger fulfillment

	return nil
}

func handleOrderShipped(data map[string]interface{}, metadata map[string]interface{}) error {
	orderID := data["order_id"]
	trackingNumber := data["tracking_number"]
	fmt.Printf("    → Order shipped: ID=%v, Tracking=%v\n", orderID, trackingNumber)

	time.Sleep(60 * time.Millisecond)

	// Your business logic here:
	// - Send shipping notification
	// - Update tracking info
	// - Schedule delivery notifications

	return nil
}

func handleOrderDelivered(data map[string]interface{}, metadata map[string]interface{}) error {
	orderID := data["order_id"]
	fmt.Printf("    → Order delivered: ID=%v\n", orderID)

	time.Sleep(80 * time.Millisecond)

	// Your business logic here:
	// - Send delivery confirmation
	// - Request review
	// - Update customer satisfaction metrics

	return nil
}

func handleNotification(data map[string]interface{}, metadata map[string]interface{}) error {
	notificationType := data["type"]
	recipient := data["recipient"]
	fmt.Printf("    → Sending notification: Type=%v, To=%v\n", notificationType, recipient)

	time.Sleep(25 * time.Millisecond)

	// Your business logic here:
	// - Format notification
	// - Send via appropriate channel (email, SMS, push)
	// - Track delivery status

	return nil
}

func reportStats(ctx context.Context, stats *ProcessingStats) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fmt.Printf("[STATS] %s\n", stats)
		}
	}
}

/*
Example usage:

1. Start the subscriber:
   go run main.go -token="your-sailhouse-token" -processors=3

2. The subscriber will:
   - Connect to Sailhouse with your token
   - Register handlers for user-events, order-events, and notifications topics
   - Start 3 processors per subscription (9 total processors)
   - Process events concurrently with automatic retries
   - Report processing statistics every 10 seconds
   - Handle graceful shutdown on SIGINT/SIGTERM

3. Send test events to your topics to see the processing in action

4. Stop gracefully with Ctrl+C

Features demonstrated:
- Multiple topic subscriptions with different handlers
- Concurrent processing with configurable processor count
- Error handling with retries and error reporting
- Processing statistics and monitoring
- Graceful shutdown handling
- Event type routing within handlers
- Simulated processing times and failure scenarios

Production considerations:
- Add proper logging with structured logs
- Implement proper error tracking (e.g., Sentry)
- Add metrics collection (e.g., Prometheus)
- Configure appropriate retry counts and delays
- Implement circuit breakers for external service calls
- Add health check endpoints
- Use proper configuration management
- Implement dead letter queue handling for failed events
*/
