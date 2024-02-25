package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/sailhouse/sdk-go/sailhouse"
	"time"
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
		fmt.Println(fmt.Sprintf("Event: %v", data))
		err = event.Ack(ctx)
		if err != nil {
			panic(err)
		}
	}

}
