package handlers

import (
	"context"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"strings"
)

// A Content SID belongs to the account of its local template binding. The
// callback's claimed AccountSid is not a credential selector by itself.
func (h *TwilioContentStatusHandler) SetVerifier(apps application.Repository, templates interface {
	FindByTwilioContentSID(context.Context, string) (*whatsapp.RichTemplate, error)
}, account, secret, publicURL string) {
	h.publicURL = publicURL
	h.resolveSigning = func(ctx context.Context, sid, claimedAccount string) (string, string, error) {
		tpl, err := templates.FindByTwilioContentSID(ctx, sid)
		if err != nil || tpl == nil {
			return "", "", fmt.Errorf("content binding unavailable")
		}
		app, err := apps.GetByID(ctx, tpl.AppID)
		if err != nil || app == nil {
			return "", "", fmt.Errorf("content application unavailable")
		}
		actualAccount, token := account, secret
		if cfg := app.Settings.WhatsApp; cfg != nil && cfg.AccountSID != "" {
			actualAccount, token = cfg.AccountSID, cfg.AuthToken
		}
		if token == "" || actualAccount == "" || claimedAccount != "" && claimedAccount != actualAccount {
			return "", "", fmt.Errorf("content account mismatch")
		}
		return token, tpl.AppID, nil
	}
}
func canonicalCallbackURL(publicURL, path string) string {
	return strings.TrimRight(publicURL, "/") + path
}
