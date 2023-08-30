# Sailhouse SDK - Go
To get started, follow the steps in the [Sailhouse documentation](https://docs.sailhouse.dev/getting-started/setup). You'll need to create an application and a token before using the SDK.
## Publish
```go
client := sailhouse.NewSailhouseClient("YOUR_TOKEN")

data := map[string]interface{}{
    "greeting": "hello world!",
}
err := client.Publish(context.Background(), "topic", data)
```

## Get events
```go
client := sailhouse.NewSailhouseClient("YOUR_TOKEN")

response, err := client.GetEvents(ctx, "TOPIC_NAME", "SUBSCRIPTION_NAME", sailhouse.WithTimeWindow(time.Hour*6))
if err != nil {
    panic(err)
}

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

```