package main

import (
	"context"
	frn "github.com/the-monkeys/freerangenotify/sdk/go/freerangenotify"
	"log"
)

func main() {
	client := frn.New("YOUR_API_KEY", frn.WithBaseURL("https://notify.example.test/v1"))
	_, err := client.Notifications.Send(context.Background(), frn.NotificationSendParams{UserID: "YOUR_USER_ID", Channel: "email", TemplateID: "YOUR_TEMPLATE_ID", Data: map[string]interface{}{"name": "Alice"}})
	if err != nil {
		log.Fatal(err)
	}
}
