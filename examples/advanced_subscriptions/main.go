package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/sailhouse/sdk-go/sailhouse"
)

func main() {
	// Parse command line flags
	token := flag.String("token", "", "Sailhouse API token")
	topic := flag.String("topic", "", "Topic name")
	subscription := flag.String("subscription", "", "Subscription name")
	endpoint := flag.String("endpoint", "", "Webhook endpoint URL")
	flag.Parse()

	if *token == "" || *topic == "" || *subscription == "" || *endpoint == "" {
		fmt.Println("Usage: go run main.go -token=<token> -topic=<topic> -subscription=<sub> -endpoint=<url>")
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// Initialize clients
	client := sailhouse.NewSailhouseClient(*token)
	adminClient := sailhouse.NewAdminClient(client)

	fmt.Println("Advanced Subscription Configuration Examples")
	fmt.Println("==========================================")

	// Example 1: Simple boolean filter
	fmt.Println("\n1. Registering subscription with simple boolean filter (enabled)...")
	err := registerWithSimpleFilter(ctx, adminClient, *topic, *subscription+"_simple", *endpoint)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Println("✓ Successfully registered with simple filter")
	}

	// Example 2: Complex filter with multiple conditions
	fmt.Println("\n2. Registering subscription with complex filter (premium users with high-value orders)...")
	err = registerWithComplexFilter(ctx, adminClient, *topic, *subscription+"_complex", *endpoint)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Println("✓ Successfully registered with complex filter")
	}

	// Example 3: Rate limiting
	fmt.Println("\n3. Registering subscription with rate limiting (10 events per minute)...")
	err = registerWithRateLimit(ctx, adminClient, *topic, *subscription+"_ratelimited", *endpoint)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Println("✓ Successfully registered with rate limiting")
	}

	// Example 4: Deduplication
	fmt.Println("\n4. Registering subscription with deduplication (5 minute window)...")
	err = registerWithDeduplication(ctx, adminClient, *topic, *subscription+"_deduped", *endpoint)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Println("✓ Successfully registered with deduplication")
	}

	// Example 5: All features combined
	fmt.Println("\n5. Registering subscription with all advanced features...")
	err = registerWithAllFeatures(ctx, adminClient, *topic, *subscription+"_advanced", *endpoint)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Println("✓ Successfully registered with all advanced features")
	}

	fmt.Println("\nAll subscriptions registered successfully!")
	fmt.Println("You can now send test events to see the filtering and rate limiting in action.")
}

func registerWithSimpleFilter(ctx context.Context, admin *sailhouse.AdminClient, topic, subscription, endpoint string) error {
	options := sailhouse.NewRegisterOptions(
		sailhouse.WithFilter(sailhouse.NewSimpleFilter(true)),
	)

	result, err := admin.RegisterPushSubscription(ctx, topic, subscription, endpoint, options)
	if err != nil {
		return err
	}

	fmt.Printf("   Result: %s\n", result.Outcome)
	return nil
}

func registerWithComplexFilter(ctx context.Context, admin *sailhouse.AdminClient, topic, subscription, endpoint string) error {
	// Create a complex filter: (user_type = "premium" AND order_amount > 100) OR priority = "urgent"
	complexFilter := sailhouse.NewComplexFilter("OR",
		sailhouse.NewFilterCondition("data.user.type", "equals", "premium"),
		sailhouse.NewFilterCondition("data.order.amount", "greater_than", "100"),
		sailhouse.NewFilterCondition("data.priority", "equals", "urgent"),
	)

	options := sailhouse.NewRegisterOptions(
		sailhouse.WithFilter(complexFilter),
	)

	result, err := admin.RegisterPushSubscription(ctx, topic, subscription, endpoint, options)
	if err != nil {
		return err
	}

	fmt.Printf("   Result: %s\n", result.Outcome)
	fmt.Printf("   Filter: %d conditions with '%s' operator\n", len(complexFilter.Filters), complexFilter.Operator)
	return nil
}

func registerWithRateLimit(ctx context.Context, admin *sailhouse.AdminClient, topic, subscription, endpoint string) error {
	options := sailhouse.NewRegisterOptions(
		sailhouse.WithRateLimit("10/1m"), // 10 events per minute
	)

	result, err := admin.RegisterPushSubscription(ctx, topic, subscription, endpoint, options)
	if err != nil {
		return err
	}

	fmt.Printf("   Result: %s\n", result.Outcome)
	fmt.Printf("   Rate Limit: 10 events per minute\n")
	return nil
}

func registerWithDeduplication(ctx context.Context, admin *sailhouse.AdminClient, topic, subscription, endpoint string) error {
	options := sailhouse.NewRegisterOptions(
		sailhouse.WithDeduplication("5m"), // 5 minute deduplication window
	)

	result, err := admin.RegisterPushSubscription(ctx, topic, subscription, endpoint, options)
	if err != nil {
		return err
	}

	fmt.Printf("   Result: %s\n", result.Outcome)
	fmt.Printf("   Deduplication: 5 minute window\n")
	return nil
}

func registerWithAllFeatures(ctx context.Context, admin *sailhouse.AdminClient, topic, subscription, endpoint string) error {
	// Complex filter for high-priority events
	filter := sailhouse.NewComplexFilter("AND",
		sailhouse.NewFilterCondition("data.environment", "equals", "production"),
		sailhouse.NewFilterCondition("data.severity", "in", "high,critical"),
	)

	options := sailhouse.NewRegisterOptions(
		sailhouse.WithFilter(filter),
		sailhouse.WithRateLimit("50/1h"),    // 50 events per hour
		sailhouse.WithDeduplication("10m"),  // 10 minute deduplication
	)

	result, err := admin.RegisterPushSubscription(ctx, topic, subscription, endpoint, options)
	if err != nil {
		return err
	}

	fmt.Printf("   Result: %s\n", result.Outcome)
	fmt.Printf("   Filter: Production environment + high/critical severity\n")
	fmt.Printf("   Rate Limit: 50 events per hour\n")
	fmt.Printf("   Deduplication: 10 minute window\n")
	return nil
}