package providers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"strings"

	"github.com/the-monkeys/freerangenotify/internal/domain/attachment"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/providers/render"
	"go.uber.org/zap"
)

// Discord incoming-webhook attachment limits.
//   - https://discord.com/developers/docs/reference#uploading-files
//   - https://discord.com/developers/docs/resources/webhook#execute-webhook
//
// 10 files per message, 25 MiB total per message on unboosted servers. We
// enforce the count limit client-side (cheap) and surface byte-overruns by
// letting Discord reject with 413 — server-tier limits can lift the size cap
// and we don't want to over-restrict here.
const (
	discordMaxAttachmentCount = 10
	discordMaxAttachmentBytes = 25 * 1024 * 1024 // soft cap; informational
)

// resolveDiscordAttachments materialises notif.Content.Attachments using the
// AttachmentResolveFunc closure carried on ctx.
//
// URL-source attachments are deliberately EXCLUDED from multipart upload:
// the Discord render layer already emits them as embed.image / embed.video
// references and Discord fetches the public URL itself. Uploading them as
// files[N] would cause Discord to render the same image twice (once from
// the embed URL, once as a hosted attachment). Only file_id and inline
// (content_base64) sources need to ride the multipart body because they
// have no publicly fetchable URL Discord could reach.
//
// Returns:
//   - resolved: nil when there is nothing to upload, the slice otherwise.
//     CALLER MUST defer attachment.CloseAll(resolved).
//   - dropped: true when uploadable attachments exist but no resolver was
//     on ctx. The send proceeds without files — mirrors the email path.
//   - err: only when the resolver itself failed (file store miss / read
//     error). The send must abort in that case.
func resolveDiscordAttachments(
	ctx context.Context,
	notif *notification.Notification,
	logger *zap.Logger,
) (resolved []*attachment.Resolved, dropped bool, err error) {
	if notif == nil || len(notif.Content.Attachments) == 0 {
		return nil, false, nil
	}

	// Filter to attachments whose bytes must travel in the multipart body.
	// URL-source attachments are rendered via embed and skipped here.
	uploadable := make([]notification.Attachment, 0, len(notif.Content.Attachments))
	for _, a := range notif.Content.Attachments {
		if a.FileID != "" || a.ContentBase64 != "" {
			uploadable = append(uploadable, a)
		}
	}
	if len(uploadable) == 0 {
		return nil, false, nil
	}

	fn, ok := ctx.Value(AttachmentResolverKey).(AttachmentResolveFunc)
	if !ok || fn == nil {
		logger.Warn("Discord attachments dropped: no resolver on context",
			zap.String("notification_id", notif.NotificationID),
			zap.Int("attachment_count", len(uploadable)))
		return nil, true, nil
	}
	if len(uploadable) > discordMaxAttachmentCount {
		return nil, false, fmt.Errorf("discord: too many uploadable attachments (%d > %d)",
			len(uploadable), discordMaxAttachmentCount)
	}
	r, rErr := fn(ctx, uploadable)
	if rErr != nil {
		logger.Error("Failed to resolve discord attachments",
			zap.String("notification_id", notif.NotificationID),
			zap.Int("attachment_count", len(uploadable)),
			zap.Error(rErr))
		return nil, false, rErr
	}
	return r, false, nil
}

// buildDiscordMultipart packages payload_json + files[N] into a multipart
// body Discord's incoming-webhook endpoint accepts. The returned
// (contentType, body) pair must be passed verbatim to the HTTP POST so the
// boundary token matches.
//
// Side effects: each resolved attachment's Reader is drained (readResolvedBytes
// caches the bytes on the struct so a retry — e.g. Discord poll fallback —
// reuses them without re-fetching).
func buildDiscordMultipart(payloadJSON []byte, resolved []*attachment.Resolved) (string, []byte, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// 1) payload_json part. Discord requires Content-Type application/json
	//    on this part so it isn't treated as a generic form field.
	jsonHeader := make(textproto.MIMEHeader)
	jsonHeader.Set("Content-Disposition", `form-data; name="payload_json"`)
	jsonHeader.Set("Content-Type", "application/json")
	jsonPart, err := mw.CreatePart(jsonHeader)
	if err != nil {
		return "", nil, fmt.Errorf("create payload_json part: %w", err)
	}
	if _, err := jsonPart.Write(payloadJSON); err != nil {
		return "", nil, fmt.Errorf("write payload_json part: %w", err)
	}

	// 2) files[N] parts, one per resolved attachment.
	for i, ra := range resolved {
		if ra == nil {
			continue
		}
		body, err := readResolvedBytes(ra)
		if err != nil {
			return "", nil, fmt.Errorf("read attachment %d: %w", i, err)
		}
		fileHeader := make(textproto.MIMEHeader)
		fileHeader.Set("Content-Disposition", fmt.Sprintf(
			`form-data; name="files[%d]"; filename=%q`, i, coalesceFilename(ra.Filename)))
		fileHeader.Set("Content-Type", coalesceMIME(ra.MIMEType))
		filePart, err := mw.CreatePart(fileHeader)
		if err != nil {
			return "", nil, fmt.Errorf("create files[%d] part: %w", i, err)
		}
		if _, err := io.Copy(filePart, bytes.NewReader(body)); err != nil {
			return "", nil, fmt.Errorf("write files[%d] part: %w", i, err)
		}
	}

	if err := mw.Close(); err != nil {
		return "", nil, fmt.Errorf("close multipart writer: %w", err)
	}
	return mw.FormDataContentType(), buf.Bytes(), nil
}

// discordAttachmentsTotalBytes is a debug helper for log lines — sums the
// (already-cached) bytes across resolved attachments so the worker log shows
// the upload weight at INFO level. Returns 0 on nil slice.
func discordAttachmentsTotalBytes(resolved []*attachment.Resolved) int {
	total := 0
	for _, ra := range resolved {
		if ra == nil {
			continue
		}
		total += len(ra.Bytes)
	}
	return total
}

// isDiscordEmptyMessage and friends intentionally omitted: render package
// guarantees a non-empty Discord payload; if it ever did not, Discord's own
// 400 response would surface in the existing error path.

// buildDiscordAttachmentRefs aligns resolved uploads back to the original
// Content.Attachments by index. URL-source attachments get an empty ref
// (renderer falls through to a.URL); file_id / inline get the resolved
// filename so the renderer can emit `attachment://<filename>`.
//
// Invariant: `resolved` is in the order produced by resolveDiscordAttachments,
// which iterates `specs` and includes only entries with FileID or
// ContentBase64 set. Any deviation from that order would mis-pair the refs.
func buildDiscordAttachmentRefs(specs []notification.Attachment, resolved []*attachment.Resolved) []render.DiscordAttachmentRef {
	refs := make([]render.DiscordAttachmentRef, len(specs))
	ri := 0
	for i, a := range specs {
		if a.FileID == "" && a.ContentBase64 == "" {
			continue
		}
		if ri >= len(resolved) || resolved[ri] == nil {
			ri++
			continue
		}
		refs[i].UploadedFilename = resolved[ri].Filename
		ri++
	}
	return refs
}

// assignUniqueFilenames returns a per-resolved slice of safe, unique filenames
// suitable for Discord's `files[N]` parts and matching `attachment://`
// references in the embed.
//
// Rules:
//   - Empty or whitespace filenames become "attachment".
//   - Duplicate names get a numeric suffix BEFORE the extension:
//     ["photo.jpg", "photo.jpg"] → ["photo.jpg", "photo-2.jpg"]
//   - Order is preserved so the caller can write the values straight back
//     onto the resolved structs.
func assignUniqueFilenames(resolved []*attachment.Resolved) []string {
	out := make([]string, len(resolved))
	seen := make(map[string]int, len(resolved))
	for i, ra := range resolved {
		base := "attachment"
		if ra != nil {
			if trimmed := strings.TrimSpace(ra.Filename); trimmed != "" {
				base = trimmed
			}
		}
		name := base
		if n, dup := seen[base]; dup {
			n++
			seen[base] = n
			name = suffixBeforeExt(base, n)
		} else {
			seen[base] = 1
		}
		// Guarantee the suffixed name is itself unique (handles a "photo.jpg",
		// "photo-2.jpg", "photo.jpg" sequence correctly).
		for {
			if _, clash := seen[name]; !clash || name == base {
				break
			}
			seen[base]++
			name = suffixBeforeExt(base, seen[base])
		}
		seen[name] = 1
		out[i] = name
	}
	return out
}

// suffixBeforeExt inserts "-N" before the final extension of name. Names
// without a dot get "-N" appended at the end. Used by assignUniqueFilenames
// to de-duplicate without breaking the file extension that drives Discord's
// inline previewing decisions.
func suffixBeforeExt(name string, n int) string {
	idx := strings.LastIndex(name, ".")
	if idx <= 0 { // no extension or leading dot → just append
		return fmt.Sprintf("%s-%d", name, n)
	}
	return fmt.Sprintf("%s-%d%s", name[:idx], n, name[idx:])
}
