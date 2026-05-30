package providers

import (
	"context"
	"fmt"
	"io"

	"go.uber.org/zap"

	"github.com/the-monkeys/freerangenotify/internal/domain/attachment"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
)

// resolveEmailAttachments materialises any binary attachments declared on the
// notification, using the AttachmentResolveFunc closure that the worker
// pre-binds onto the ctx. Returns:
//
//   - resolved: nil when there are no attachments to send, the resolved
//     slice otherwise. CALLER MUST defer attachment.CloseAll(resolved).
//   - dropped: true when the caller supplied attachments but no resolver
//     was on the ctx (partial-rollout safety net). Providers should log
//     loudly and proceed without the attachments rather than failing the
//     whole send — symmetric with the SMTP provider behaviour.
//   - err: a real resolver error (URL fetch failed, file_id missing,
//     oversize, etc.). Providers MUST propagate this as an Invalid result.
//
// This helper exists to keep the 6 email providers (SMTP + SES, SendGrid,
// Mailgun, Postmark, Resend) byte-identical on the resolver path so a bug
// fixed once is fixed everywhere.
func resolveEmailAttachments(
	ctx context.Context,
	notif *notification.Notification,
	logger *zap.Logger,
	providerName string,
) (resolved []*attachment.Resolved, dropped bool, err error) {
	if notif == nil || len(notif.Content.Attachments) == 0 {
		return nil, false, nil
	}
	fn, ok := ctx.Value(AttachmentResolverKey).(AttachmentResolveFunc)
	if !ok || fn == nil {
		logger.Warn("Email attachments dropped: no resolver on context",
			zap.String("provider", providerName),
			zap.String("notification_id", notif.NotificationID),
			zap.Int("attachment_count", len(notif.Content.Attachments)))
		return nil, true, nil
	}
	r, rErr := fn(ctx, notif.Content.Attachments)
	if rErr != nil {
		logger.Error("Failed to resolve email attachments",
			zap.String("provider", providerName),
			zap.String("notification_id", notif.NotificationID),
			zap.Int("attachment_count", len(notif.Content.Attachments)),
			zap.Error(rErr))
		return nil, false, rErr
	}
	return r, false, nil
}

// readResolvedBytes returns the raw bytes for a Resolved attachment, draining
// the Reader once if Bytes is empty. Used by JSON-shaped email providers
// (SendGrid, Postmark, Resend, Mailgun) that must base64-encode or
// multipart-stream the payload before HTTP.
func readResolvedBytes(ra *attachment.Resolved) ([]byte, error) {
	if ra == nil {
		return nil, fmt.Errorf("nil resolved attachment")
	}
	if len(ra.Bytes) > 0 {
		return ra.Bytes, nil
	}
	if ra.Reader == nil {
		return nil, fmt.Errorf("attachment %q has no bytes and no reader", ra.Filename)
	}
	body, err := io.ReadAll(ra.Reader)
	if err != nil {
		return nil, fmt.Errorf("read attachment %q: %w", ra.Filename, err)
	}
	// Cache so a second pass through the same provider doesn't re-drain.
	ra.Bytes = body
	return body, nil
}

func coalesceMIME(s string) string {
	if s == "" {
		return "application/octet-stream"
	}
	return s
}

func coalesceFilename(s string) string {
	if s == "" {
		return "attachment"
	}
	return s
}
