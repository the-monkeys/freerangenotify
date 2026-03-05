# FreeRangeNotify Go SDK

Official Go client for the [FreeRangeNotify](https://github.com/the-monkeys/freerangenotify) notification service.

## Installation

```bash
go get github.com/the-monkeys/freerangenotify/sdk/go/freerangenotify
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    frn "github.com/the-monkeys/freerangenotify/sdk/go/freerangenotify"
)

func main() {
    client := frn.New("your-api-key",
        frn.WithBaseURL("http://localhost:8080/v1"),
    )

    // Send a notification using a template
    result, err := client.Send(context.Background(), frn.SendParams{
        To:       "user@example.com",
        Template: "welcome_email",
        Data:     map[string]interface{}{"name": "Alice", "product": "MyApp"},
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Sent: %s (status: %s)\n", result.NotificationID, result.Status)

    // Send a plain notification
    result, err = client.Send(context.Background(), frn.SendParams{
        To:      "user@example.com",
        Subject: "Hello!",
        Body:    "<h1>Hello, World!</h1>",
        Channel: "email",
    })
    if err != nil {
        log.Fatal(err)
    }

    // Broadcast to all users
    broadcast, err := client.Broadcast(context.Background(), frn.BroadcastParams{
        Template: "maintenance_notice",
        Data:     map[string]interface{}{"downtime": "2 hours"},
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Broadcast sent to %d users\n", broadcast.TotalSent)

    // Create a user
    user, err := client.CreateUser(context.Background(), frn.CreateUserParams{
        Email:    "alice@example.com",
        Timezone: "America/New_York",
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Created user: %s\n", user.UserID)
}
```

## Error Handling

```go
result, err := client.Send(ctx, params)
if err != nil {
    var apiErr *frn.APIError
    if errors.As(err, &apiErr) {
        fmt.Printf("API Error %d: %s\n", apiErr.StatusCode, apiErr.Body)
    }
}
```

## Options

| Option | Description |
|--------|-------------|
| `WithBaseURL(url)` | Set custom API base URL (default: `http://localhost:8080/v1`) |
| `WithHTTPClient(client)` | Provide a custom `*http.Client` for timeouts, transports, etc. |
