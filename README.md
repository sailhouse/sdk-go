# Sailhouse SDK - Go

```go
client := sailhouse.NewClient("token")

data := map[string]string {
    "some-property": false,
}

client.Publish("topic", data)
```
