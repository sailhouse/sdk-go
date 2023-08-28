# Sailhouse SDK - Go
To get started, follow the steps in [Sailhouse Document](https://docs.sailhouse.dev/getting-started/setup) to create `topic`, `subscription` and `token`.  
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