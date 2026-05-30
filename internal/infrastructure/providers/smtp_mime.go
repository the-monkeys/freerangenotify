package providers

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"strings"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/attachment"
)

// ErrSMTPAttachmentReadFailed is returned when a Resolved.Reader source
// fails mid-read. It is wrapped, never returned bare.
var ErrSMTPAttachmentReadFailed = errors.New("smtp: failed to read attachment body")

// smtpMessageOptions captures the inputs to buildSMTPMessage. Keeping this
// in a struct rather than a long arg list lets us evolve the builder
// (e.g. plaintext alternative, custom headers) without rippling through
// the SMTP provider call sites.
type smtpMessageOptions struct {
	From        string // bare email
	FromName    string // display name (optional)
	To          string // bare email
	Subject     string
	HTMLBody    string
	Attachments []*attachment.Resolved
}

// buildSMTPMessage produces an RFC 5322 / 2045 message ready to hand to
// smtp.SendMail. It picks the simplest valid MIME structure for the input:
//
//	no attachments                 -> text/html
//	any attachment, no inline      -> multipart/mixed (html + parts)
//	inline only (cid: in HTML)     -> multipart/related (html + inline parts)
//	mixed inline + regular         -> multipart/mixed { multipart/related { html, inline }, attachments }
//
// All attachment bytes are base64-transfer-encoded with 76-column wrapping per
// RFC 2045. Inline parts emit a Content-ID header so HTML <img src="cid:..."/>
// resolves at the client. Callers MUST defer attachment.CloseAll on the
// provided slice — this function never closes readers itself.
func buildSMTPMessage(opts smtpMessageOptions) ([]byte, error) {
	// Partition attachments by disposition so we can pick the right structure.
	var inline, regular []*attachment.Resolved
	for _, a := range opts.Attachments {
		if a == nil {
			continue
		}
		if strings.EqualFold(a.Disposition, "inline") && a.ContentID != "" {
			inline = append(inline, a)
		} else {
			regular = append(regular, a)
		}
	}

	// Fast path: no attachments at all → preserve historical text/html shape
	// byte-for-byte. Existing callers and tests must see no change.
	if len(inline) == 0 && len(regular) == 0 {
		return buildSimpleHTMLMessage(opts), nil
	}

	var buf bytes.Buffer
	writeCommonHeaders(&buf, opts)

	switch {
	case len(regular) == 0:
		// inline only → top-level multipart/related
		if err := writeRelated(&buf, opts.HTMLBody, inline); err != nil {
			return nil, err
		}
	case len(inline) == 0:
		// regular only → top-level multipart/mixed with html as first part
		if err := writeMixed(&buf, opts.HTMLBody, nil, regular); err != nil {
			return nil, err
		}
	default:
		// both → multipart/mixed wrapping multipart/related + regular parts
		if err := writeMixed(&buf, opts.HTMLBody, inline, regular); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

// buildSimpleHTMLMessage matches the legacy, attachment-free shape so the
// no-attachment send path is byte-stable. We keep it close to the original
// implementation deliberately.
func buildSimpleHTMLMessage(opts smtpMessageOptions) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\n", formatFromHeader(opts.From, opts.FromName))
	fmt.Fprintf(&b, "To: %s\r\n", opts.To)
	fmt.Fprintf(&b, "Subject: %s\r\n", opts.Subject)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	b.WriteString("\r\n")
	b.WriteString(opts.HTMLBody)
	return b.Bytes()
}

func writeCommonHeaders(buf *bytes.Buffer, opts smtpMessageOptions) {
	fmt.Fprintf(buf, "From: %s\r\n", formatFromHeader(opts.From, opts.FromName))
	fmt.Fprintf(buf, "To: %s\r\n", opts.To)
	fmt.Fprintf(buf, "Subject: %s\r\n", opts.Subject)
	fmt.Fprintf(buf, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	fmt.Fprintf(buf, "Message-ID: <%s@freerangenotify>\r\n", randomToken(12))
	buf.WriteString("MIME-Version: 1.0\r\n")
}

// writeMixed writes a multipart/mixed body. If `inline` is non-empty the
// first sub-part is a nested multipart/related (HTML + inline parts);
// otherwise the first sub-part is the HTML body itself.
func writeMixed(buf *bytes.Buffer, htmlBody string, inline, regular []*attachment.Resolved) error {
	w := multipart.NewWriter(buf)
	fmt.Fprintf(buf, "Content-Type: multipart/mixed; boundary=%q\r\n\r\n", w.Boundary())

	// First part: HTML body OR nested multipart/related
	if len(inline) > 0 {
		relatedHdr := textproto.MIMEHeader{}
		var relatedBuf bytes.Buffer
		relatedWriter := multipart.NewWriter(&relatedBuf)
		relatedHdr.Set("Content-Type", fmt.Sprintf("multipart/related; boundary=%q", relatedWriter.Boundary()))
		nestedPart, err := w.CreatePart(relatedHdr)
		if err != nil {
			return err
		}
		if err := writeHTMLBodyPart(relatedWriter, htmlBody); err != nil {
			return err
		}
		for _, a := range inline {
			if err := writeAttachmentPart(relatedWriter, a, true); err != nil {
				return err
			}
		}
		if err := relatedWriter.Close(); err != nil {
			return err
		}
		if _, err := nestedPart.Write(relatedBuf.Bytes()); err != nil {
			return err
		}
	} else {
		if err := writeHTMLBodyPart(w, htmlBody); err != nil {
			return err
		}
	}

	for _, a := range regular {
		if err := writeAttachmentPart(w, a, false); err != nil {
			return err
		}
	}
	return w.Close()
}

// writeRelated writes a top-level multipart/related body (HTML + inline-only).
func writeRelated(buf *bytes.Buffer, htmlBody string, inline []*attachment.Resolved) error {
	w := multipart.NewWriter(buf)
	fmt.Fprintf(buf, "Content-Type: multipart/related; boundary=%q\r\n\r\n", w.Boundary())

	if err := writeHTMLBodyPart(w, htmlBody); err != nil {
		return err
	}
	for _, a := range inline {
		if err := writeAttachmentPart(w, a, true); err != nil {
			return err
		}
	}
	return w.Close()
}

func writeHTMLBodyPart(w *multipart.Writer, htmlBody string) error {
	hdr := textproto.MIMEHeader{}
	hdr.Set("Content-Type", "text/html; charset=\"UTF-8\"")
	// 8bit keeps the body byte-stable for tests and is widely supported by
	// modern relays. A future change can swap in quoted-printable if a
	// stricter 7bit relay rejects us.
	hdr.Set("Content-Transfer-Encoding", "8bit")
	part, err := w.CreatePart(hdr)
	if err != nil {
		return err
	}
	_, err = io.WriteString(part, htmlBody)
	return err
}

// writeAttachmentPart writes a single part for either an inline or regular
// attachment. When `inline` is true the part emits Content-ID and
// Content-Disposition: inline; otherwise Content-Disposition: attachment.
func writeAttachmentPart(w *multipart.Writer, a *attachment.Resolved, inline bool) error {
	filename := a.Filename
	if filename == "" {
		filename = "attachment"
	}
	mimeType := a.MIMEType
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	hdr := textproto.MIMEHeader{}
	// RFC 2231 encoding for non-ASCII filenames is handled by mime.BEncoding.
	encodedName := mime.BEncoding.Encode("UTF-8", filename)
	hdr.Set("Content-Type", fmt.Sprintf("%s; name=%q", mimeType, encodedName))
	hdr.Set("Content-Transfer-Encoding", "base64")
	if inline {
		hdr.Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", encodedName))
		// Strip surrounding angle brackets if caller already wrapped the cid.
		cid := strings.TrimSpace(a.ContentID)
		cid = strings.TrimPrefix(cid, "<")
		cid = strings.TrimSuffix(cid, ">")
		hdr.Set("Content-ID", "<"+cid+">")
	} else {
		hdr.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", encodedName))
	}

	part, err := w.CreatePart(hdr)
	if err != nil {
		return err
	}

	body, err := attachmentBody(a)
	if err != nil {
		return err
	}
	return writeBase64Wrapped(part, body)
}

// attachmentBody returns the bytes for a Resolved attachment. When Bytes is
// populated we use it directly; otherwise we drain Reader. The caller still
// owns Close (see buildSMTPMessage contract).
func attachmentBody(a *attachment.Resolved) ([]byte, error) {
	if len(a.Bytes) > 0 {
		return a.Bytes, nil
	}
	if a.Reader == nil {
		return nil, fmt.Errorf("smtp: attachment %q has no bytes and no reader", a.Filename)
	}
	body, err := io.ReadAll(a.Reader)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSMTPAttachmentReadFailed, err)
	}
	return body, nil
}

// writeBase64Wrapped emits a base64 stream wrapped at 76 columns per RFC 2045.
// We avoid base64.NewEncoder because it does not wrap, and downstream relays
// (notably some hardened MTAs) reject lines longer than 998 octets.
func writeBase64Wrapped(w io.Writer, src []byte) error {
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(src)))
	base64.StdEncoding.Encode(encoded, src)

	const lineLen = 76
	for start := 0; start < len(encoded); start += lineLen {
		end := start + lineLen
		if end > len(encoded) {
			end = len(encoded)
		}
		if _, err := w.Write(encoded[start:end]); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\r\n"); err != nil {
			return err
		}
	}
	return nil
}

// formatFromHeader returns "Name <email>" or just "email" when no name.
func formatFromHeader(email, name string) string {
	if name == "" {
		return email
	}
	return fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("UTF-8", name), email)
}

// randomToken returns a hex token of n bytes. Used for Message-ID.
func randomToken(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		// Deterministic fallback — Message-ID uniqueness is best-effort.
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}
