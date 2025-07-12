package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/sailhouse/sdk-go/sailhouse"
)

func main() {
	// Declare context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// Load token from flag
	token := flag.String("token", "", "token")
	topic := flag.String("topic", "", "topic")
	subscription := flag.String("subscription", "", "subscription")
	flag.Parse()

	// Init client
	client := sailhouse.NewSailhouseClient(*token)

	// Pull events
	response, err := client.GetEvents(ctx, *topic, *subscription, sailhouse.WithTimeWindow(time.Hour*6))
	if err != nil {
		panic(err)
	}

	// Parse and handle events
	for _, event := range response.Events {
		var data map[string]interface{}
		err := event.As(&data)
		if err != nil {
			panic(err)
		}

		fmt.Printf("Event ID: %s\n", event.ID)
		fmt.Printf("Event Data: %v\n", data)

		// Display metadata if present
		if len(event.Metadata) > 0 {
			fmt.Printf("Event Metadata: %v\n", event.Metadata)
		} else {
			fmt.Println("No metadata present")
		}

		fmt.Println("---")

		err = event.Ack(ctx)
		if err != nil {
			panic(err)
		}
	}

}
