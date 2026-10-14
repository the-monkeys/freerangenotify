package channelconnection

import (
	"crypto/sha256"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"strings"
)

const DeliveryMetadataKey = "channel_delivery"

type PreparedDelivery struct {
	Snapshot     *Snapshot                  `json:"snapshot"`
	PayloadKind  string                     `json:"payload_kind"`
	Notification *notification.Notification `json:"-"`
	User         *user.User                 `json:"-"`
}

// Window identity excludes token/revision: rotating a key for the same sender
// does not invalidate an upstream conversation, moving accounts does.
func WindowKey(app, env, provider, account, sender, contact string) string {
	if env == "" {
		env = "default"
	}
	canonical := func(v string) string { return strings.TrimPrefix(strings.TrimPrefix(v, "whatsapp:"), "+") }
	return fmt.Sprintf("csw2:%x", sha256.Sum256([]byte(strings.Join([]string{app, env, provider, account, canonical(sender), canonical(contact)}, "\x00"))))
}
