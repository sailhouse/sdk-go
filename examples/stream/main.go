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
	events, errs := client.StreamEvents(ctx, *topic, *subscription)

	for {
		select {
		case event := <-events:
			var data map[string]interface{}
			err := event.As(&data)
			if err != nil {
				panic(err)
			}
			fmt.Printf("Event: %v\n", data)
			err = event.Ack(ctx)
			if err != nil {
				panic(err)
			}
		case err := <-errs:
			panic(err)
		case <-ctx.Done():
			return
		}
	}
}
