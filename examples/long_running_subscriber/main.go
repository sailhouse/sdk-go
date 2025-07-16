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
	// User lifecycle events
	subscriber.Subscribe("user.signup", "signup-processor", createEventHandler(stats, "USER", handleUserSignup))
	subscriber.Subscribe("user.email_verified", "email-verification-processor", createEventHandler(stats, "USER", handleEmailVerified))
	subscriber.Subscribe("user.trial_started", "trial-processor", createEventHandler(stats, "USER", handleTrialStarted))

	// Payment events
	subscriber.Subscribe("payment.succeeded", "payment-success-processor", createEventHandler(stats, "PAYMENT", handlePaymentSucceeded))
	subscriber.Subscribe("payment.failed", "payment-failure-processor", createEventHandler(stats, "PAYMENT", handlePaymentFailed))

	// Subscription events
	subscriber.Subscribe("subscription.created", "subscription-creation-processor", createEventHandler(stats, "SUBSCRIPTION", handleSubscriptionCreated))
	subscriber.Subscribe("subscription.cancelled", "subscription-cancellation-processor", createEventHandler(stats, "SUBSCRIPTION", handleSubscriptionCancelled))

	// Order events
	subscriber.Subscribe("order.accepted", "order-acceptance-processor", createEventHandler(stats, "ORDER", handleOrderAccepted))
	subscriber.Subscribe("order.processing", "order-processing-processor", createEventHandler(stats, "ORDER", handleOrderProcessing))
	subscriber.Subscribe("order.completed", "order-completion-processor", createEventHandler(stats, "ORDER", handleOrderCompleted))
	subscriber.Subscribe("order.refunded", "order-refund-processor", createEventHandler(stats, "ORDER", handleOrderRefunded))
}

// createEventHandler creates a reusable event handler wrapper
func createEventHandler(stats *ProcessingStats, category string, handler func(map[string]interface{}, map[string]interface{}) error) func(context.Context, *sailhouse.Event) error {
	return func(ctx context.Context, event *sailhouse.Event) error {
		stats.eventsProcessed.Add(1)

		fmt.Printf("[%s] Processing event %s\n", category, event.ID)

		var eventData map[string]interface{}
		err := event.As(&eventData)
		if err != nil {
			stats.handlerErrors.Add(1)
			return fmt.Errorf("failed to parse event data: %w", err)
		}

		return handler(eventData, event.Metadata)
	}
}

// Event handlers for user lifecycle

func handleUserSignup(data map[string]interface{}, metadata map[string]interface{}) error {
	userID := data["user_id"]
	email := data["email"]
	plan := data["plan"]
	fmt.Printf("    → User signup: ID=%v, Email=%v, Plan=%v\n", userID, email, plan)

	time.Sleep(50 * time.Millisecond)

	return nil
}

func handleEmailVerified(data map[string]interface{}, metadata map[string]interface{}) error {
	userID := data["user_id"]
	email := data["email"]
	fmt.Printf("    → Email verified: ID=%v, Email=%v\n", userID, email)

	time.Sleep(30 * time.Millisecond)

	return nil
}

func handleTrialStarted(data map[string]interface{}, metadata map[string]interface{}) error {
	userID := data["user_id"]
	trialDays := data["trial_days"]
	fmt.Printf("    → Trial started: User=%v, Days=%v\n", userID, trialDays)

	time.Sleep(40 * time.Millisecond)

	return nil
}

// Event handlers for payment processing

func handlePaymentSucceeded(data map[string]interface{}, metadata map[string]interface{}) error {
	paymentID := data["payment_id"]
	amount := data["amount"]
	currency := data["currency"]
	customerID := data["customer_id"]
	fmt.Printf("    → Payment succeeded: ID=%v, Amount=%v %v, Customer=%v\n", 
		paymentID, amount, currency, customerID)

	time.Sleep(75 * time.Millisecond)

	return nil
}

func handlePaymentFailed(data map[string]interface{}, metadata map[string]interface{}) error {
	paymentID := data["payment_id"]
	reason := data["failure_reason"]
	customerID := data["customer_id"]
	fmt.Printf("    → Payment failed: ID=%v, Reason=%v, Customer=%v\n", 
		paymentID, reason, customerID)

	time.Sleep(60 * time.Millisecond)

	return nil
}

func handleSubscriptionCreated(data map[string]interface{}, metadata map[string]interface{}) error {
	subscriptionID := data["subscription_id"]
	planID := data["plan_id"]
	customerID := data["customer_id"]
	fmt.Printf("    → Subscription created: ID=%v, Plan=%v, Customer=%v\n", 
		subscriptionID, planID, customerID)

	time.Sleep(80 * time.Millisecond)

	return nil
}

func handleSubscriptionCancelled(data map[string]interface{}, metadata map[string]interface{}) error {
	subscriptionID := data["subscription_id"]
	reason := data["cancellation_reason"]
	customerID := data["customer_id"]
	fmt.Printf("    → Subscription cancelled: ID=%v, Reason=%v, Customer=%v\n", 
		subscriptionID, reason, customerID)

	time.Sleep(65 * time.Millisecond)

	return nil
}

// Event handlers for order fulfillment

func handleOrderAccepted(data map[string]interface{}, metadata map[string]interface{}) error {
	orderID := data["order_id"]
	customerID := data["customer_id"]
	items := data["items"]
	fmt.Printf("    → Order accepted: ID=%v, Customer=%v, Items=%v\n", 
		orderID, customerID, items)

	time.Sleep(100 * time.Millisecond)

	return nil
}

func handleOrderProcessing(data map[string]interface{}, metadata map[string]interface{}) error {
	orderID := data["order_id"]
	status := data["status"]
	fmt.Printf("    → Order processing: ID=%v, Status=%v\n", orderID, status)

	time.Sleep(90 * time.Millisecond)

	return nil
}

func handleOrderCompleted(data map[string]interface{}, metadata map[string]interface{}) error {
	orderID := data["order_id"]
	deliveryDate := data["delivery_date"]
	fmt.Printf("    → Order completed: ID=%v, Delivered=%v\n", orderID, deliveryDate)

	time.Sleep(70 * time.Millisecond)

	return nil
}

func handleOrderRefunded(data map[string]interface{}, metadata map[string]interface{}) error {
	orderID := data["order_id"]
	refundAmount := data["refund_amount"]
	reason := data["reason"]
	fmt.Printf("    → Order refunded: ID=%v, Amount=%v, Reason=%v\n", 
		orderID, refundAmount, reason)

	time.Sleep(85 * time.Millisecond)

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

