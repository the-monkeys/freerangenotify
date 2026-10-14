package handlers

import (
	"context"
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"go.uber.org/zap"
	"net/http/httptest"
	"strings"
	"testing"
)

type acceptedReplyService struct {
	whatsapp.Service
	environment string
	key         string
}

func (s *acceptedReplyService) Reply(ctx context.Context, app, contact, text, template string) error {
	return nil
}
func (s *acceptedReplyService) ReplyQueued(ctx context.Context, app, contact, text, template, key string) (*notification.Notification, error) {
	s.environment = channelconnection.EnvironmentID(ctx)
	s.key = key
	return &notification.Notification{NotificationID: "queued-reply-fixture", Status: notification.StatusQueued}, nil
}

func TestChannelInboxReplyAccountingHandler(t *testing.T) {
	svc := &acceptedReplyService{}
	handler := NewWhatsAppConversationHandler(svc, zap.NewNop())
	app := fiber.New()
	app.Post("/reply/:contact_id", func(c *fiber.Ctx) error {
		c.Locals("app_id", "a")
		c.Locals("environment_id", "prod")
		return handler.Reply(c)
	})
	req := httptest.NewRequest("POST", "/reply/919876543210?environment=prod", strings.NewReader(`{"text":"hello","idempotency_key":"fixture-key"}`))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var body map[string]interface{}
	json.NewDecoder(res.Body).Decode(&body)
	if res.StatusCode != 200 || body["notification_id"] != "queued-reply-fixture" || body["status"] != "queued" || body["message"] != "Reply queued" || svc.environment != "prod" || svc.key != "fixture-key" {
		t.Fatalf("false reply acceptance or lost scope: %+v env=%s key=%s", body, svc.environment, svc.key)
	}
}
