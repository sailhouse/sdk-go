package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/sailhouse/sdk-go/sailhouse"
)

func main() {
	client := sailhouse.NewSailhouseClient("token")
	ack := flag.Bool("ack", false, "acknowledge messages")
	token := flag.String("token", "", "token")
	sendSimple := flag.Bool("send-simple", false, "send simple message")
	flag.Parse()

	fmt.Printf("Token: %s\nAck: %v\n", *token, *ack)

	if *token != "" {
		client = sailhouse.NewSailhouseClient(*token)
	}

	events, err := client.GetEvents("load-test", "load-test-pull-thing", sailhouse.WithTimeWindow(time.Hour*6))
	if err != nil {
		panic(err)
	}

	for _, event := range events.Events {
		var data map[string]interface{}
		err := event.As(&data)
		if err != nil {
			panic(err)
		}

		fmt.Printf("Event: %v\n", data)

		if *ack {
			event.Ack()
		}
	}

	if *sendSimple {
		data := map[string]interface{}{
			"hello": "world",
		}
		err := client.Publish("load-test", data)
		if err != nil {
			panic(err)
		}
	}
}
