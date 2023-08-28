# Sailhouse SDK - Go

## Publish
```go
client := sailhouse.NewSailhouseClient("YOUR_TOKEN")

data := map[string]interface{}{
    "greeting": "hello world!",
}
err := client.Publish(context.Background(), "topic", data)
```
