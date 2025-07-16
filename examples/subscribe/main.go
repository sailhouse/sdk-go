package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/sailhouse/sdk-go/sailhouse"
)

func main() {
	token := os.Getenv("SAILHOUSE_TOKEN")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	client := sailhouse.NewSailhouseClient(token)
	client.Subscribe(ctx, "example-topic", "example-subscription", func(ctx context.Context, e *sailhouse.Event) {
		message := fmt.Sprintf("Received event: %s", e.ID)

		var data map[string]interface{}
		err := e.As(&data)
		if err != nil {
			fmt.Println(err)
		}

		message += fmt.Sprintf("\nData: %v", data)

		fmt.Println(message)

		err = e.Ack(ctx)
		if err != nil {
			fmt.Println(err)
		}
	}, &sailhouse.SubscriptionOptions{
		OnError: func(err error) {
			fmt.Println(err)
			cancel()
		},
		ExitOnErr: true,
	})

	<-ctx.Done()
}
