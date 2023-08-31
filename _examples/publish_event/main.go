package main

import (
	"context"
	"flag"
	"github.com/sailhouse/sdk-go/sailhouse"
	"time"
)

func main() {
	// Declare context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// Load token from flag
	token := flag.String("token", "", "token")
	topic := flag.String("topic", "topic", "topic")
	flag.Parse()

	// Init client
	client := sailhouse.NewSailhouseClient(*token)

	// Declare event data
	data := map[string]interface{}{
		"greeting": "hello world!",
	}

	// Publish
	err := client.Publish(ctx, *topic, data)
	if err != nil {
		panic(err)
	}
}
