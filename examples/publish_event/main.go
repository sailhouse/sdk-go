package main

import (
	"context"
	"flag"
	"os"
	"time"

	"github.com/sailhouse/sdk-go/sailhouse"
)

func main() {
	token := os.Getenv("SAILHOUSE_TOKEN")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	topic := flag.String("topic", "example-topic", "topic")
	flag.Parse()

	// Init client
	client := sailhouse.NewSailhouseClient(token)

	// Declare event data
	data := map[string]interface{}{
		"message": "hello world!",
	}

	// Publish
	_, err := client.Publish(
		ctx,
		*topic,
		data,
		sailhouse.WithScheduledTime(time.Now().Add(time.Hour*24*5)),
	)
	if err != nil {
		panic(err)
	}
}
