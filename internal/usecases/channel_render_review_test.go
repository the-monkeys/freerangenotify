package usecases

import (
	"context"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/template"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"strings"
	"testing"
)

type renderReviewDispatch struct{ body string }

func (d *renderReviewDispatch) PrepareChannelDelivery(_ context.Context, n *notification.Notification, u *user.User) (*channelconnection.PreparedDelivery, error) {
	d.body = n.Content.Body
	return &channelconnection.PreparedDelivery{}, nil
}
func TestPhase3ReviewAdmissionControlValues(t *testing.T) {
	for _, payload := range []map[string]interface{}{nil, {"brand": "Override"}} {
		d := &renderReviewDispatch{}
		s := &NotificationService{templateRepo: &mockTemplateRepo{templates: []*template.Template{{ID: "00000000-0000-0000-0000-000000000001", AppID: "a", Body: "Hello {{.brand}} {{.default_value}} {{.unsubscribe_url}}", Controls: []template.TemplateControl{{Key: "brand", Default: "Default"}, {Key: "default_value", Default: "kept"}}, ControlValues: template.ControlValues{"brand": "Acme"}, Metadata: map[string]interface{}{"category": "newsletter"}}}}, channelDelivery: d}
		n := &notification.Notification{AppID: "a", UserID: "u", TemplateID: "00000000-0000-0000-0000-000000000001", Channel: notification.ChannelSMS, Content: notification.Content{Data: payload}}
		if err := s.prepareMessaging(context.Background(), n, &user.User{}); err != nil {
			t.Fatal(err)
		}
		want := "Acme"
		if payload != nil {
			want = "Override"
		}
		if !strings.Contains(d.body, "Hello "+want+" kept https://notify.example.com/v1/unsubscribe?user=u&app=a") {
			t.Fatalf("admitted body lost enrichment: %s", d.body)
		}
	}
}
