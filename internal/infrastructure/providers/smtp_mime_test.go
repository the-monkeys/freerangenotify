package providers

import (
	"context"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/smtp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/attachment"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap/zaptest"
)

// buildSMTPMessage ------------------------------------------------------------

func TestBuildSMTPMessage_NoAttachments_PreservesLegacyShape(t *testing.T) {
	msg, err := buildSMTPMessage(smtpMessageOptions{
		From:     "from@example.com",
		FromName: "Sender",
		To:       "to@example.com",
		Subject:  "Hi",
		HTMLBody: "<p>hello</p>",
	})
	require.NoError(t, err)
	s := string(msg)

	// Legacy shape: flat text/html, no multipart, no Date/Message-ID added.
	assert.Contains(t, s, "From: Sender <from@example.com>")
	assert.Contains(t, s, "To: to@example.com")
	assert.Contains(t, s, "Subject: Hi")
	assert.Contains(t, s, "Content-Type: text/html; charset=\"UTF-8\"")
	assert.NotContains(t, s, "multipart/")
	assert.True(t, strings.HasSuffix(s, "<p>hello</p>"))
}

func TestBuildSMTPMessage_RegularAttachment_BuildsMultipartMixed(t *testing.T) {
	msg, err := buildSMTPMessage(smtpMessageOptions{
		From:     "from@example.com",
		FromName: "Sender",
		To:       "to@example.com",
		Subject:  "With PDF",
		HTMLBody: "<p>see attached</p>",
		Attachments: []*attachment.Resolved{
			{
				Filename:    "invoice.pdf",
				MIMEType:    "application/pdf",
				Disposition: "attachment",
				Bytes:       []byte("%PDF-1.4 fake pdf body"),
				Source:      attachment.SourceURL,
			},
		},
	})
	require.NoError(t, err)

	parsed := parseEmail(t, msg)
	assert.Equal(t, "With PDF", parsed.Header.Get("Subject"))
	mt, params, perr := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	require.NoError(t, perr)
	assert.Equal(t, "multipart/mixed", mt)
	require.NotEmpty(t, params["boundary"])

	parts := readParts(t, parsed.Body, params["boundary"])
	require.Len(t, parts, 2, "expected html body + attachment")

	// Part 0: html body
	htmlMT, _, _ := mime.ParseMediaType(parts[0].header.Get("Content-Type"))
	assert.Equal(t, "text/html", htmlMT)
	assert.Equal(t, "<p>see attached</p>", string(parts[0].body))

	// Part 1: attachment
	attMT, attParams, aerr := mime.ParseMediaType(parts[1].header.Get("Content-Type"))
	require.NoError(t, aerr)
	assert.Equal(t, "application/pdf", attMT)
	assert.Equal(t, "invoice.pdf", attParams["name"])
	assert.Equal(t, "base64", parts[1].header.Get("Content-Transfer-Encoding"))
	disp, dispParams, _ := mime.ParseMediaType(parts[1].header.Get("Content-Disposition"))
	assert.Equal(t, "attachment", disp)
	assert.Equal(t, "invoice.pdf", dispParams["filename"])
	// Body is the decoded base64 bytes (multipart.Reader does not decode
	// transfer-encoding; we just check the encoded form contains a known
	// base64 fragment of our payload).
	assert.Contains(t, string(parts[1].body), "JVBE") // "%PD..." base64-encoded prefix
}

func TestBuildSMTPMessage_InlineOnly_BuildsMultipartRelated(t *testing.T) {
	msg, err := buildSMTPMessage(smtpMessageOptions{
		From:     "from@example.com",
		FromName: "Sender",
		To:       "to@example.com",
		Subject:  "Inline image",
		HTMLBody: `<p><img src="cid:logo"/></p>`,
		Attachments: []*attachment.Resolved{
			{
				Filename:    "logo.png",
				MIMEType:    "image/png",
				Disposition: "inline",
				ContentID:   "logo",
				Bytes:       []byte{0x89, 0x50, 0x4E, 0x47},
				Source:      attachment.SourceInline,
			},
		},
	})
	require.NoError(t, err)

	parsed := parseEmail(t, msg)
	mt, params, _ := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	assert.Equal(t, "multipart/related", mt)

	parts := readParts(t, parsed.Body, params["boundary"])
	require.Len(t, parts, 2)

	// Inline part must carry Content-ID with surrounding angle brackets.
	assert.Equal(t, "<logo>", parts[1].header.Get("Content-ID"))
	disp, _, _ := mime.ParseMediaType(parts[1].header.Get("Content-Disposition"))
	assert.Equal(t, "inline", disp)
}

func TestBuildSMTPMessage_InlinePlusRegular_NestsRelatedInsideMixed(t *testing.T) {
	msg, err := buildSMTPMessage(smtpMessageOptions{
		From:     "from@example.com",
		To:       "to@example.com",
		Subject:  "mixed",
		HTMLBody: `<p><img src="cid:logo"/></p>`,
		Attachments: []*attachment.Resolved{
			{Filename: "logo.png", MIMEType: "image/png", Disposition: "inline", ContentID: "logo", Bytes: []byte{1, 2, 3}},
			{Filename: "doc.pdf", MIMEType: "application/pdf", Disposition: "attachment", Bytes: []byte{4, 5, 6}},
		},
	})
	require.NoError(t, err)

	parsed := parseEmail(t, msg)
	mt, params, _ := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	assert.Equal(t, "multipart/mixed", mt)

	outer := readParts(t, parsed.Body, params["boundary"])
	require.Len(t, outer, 2, "outer mixed should have related + attachment")

	relMT, relParams, _ := mime.ParseMediaType(outer[0].header.Get("Content-Type"))
	assert.Equal(t, "multipart/related", relMT)
	inner := readParts(t, strings.NewReader(string(outer[0].body)), relParams["boundary"])
	require.Len(t, inner, 2, "inner related should have html + inline")

	attDisp, _, _ := mime.ParseMediaType(outer[1].header.Get("Content-Disposition"))
	assert.Equal(t, "attachment", attDisp)
}

func TestBuildSMTPMessage_Base64LineWrapping(t *testing.T) {
	// 200 bytes encodes to ~272 base64 chars; we expect 3 wrapped lines.
	payload := make([]byte, 200)
	for i := range payload {
		payload[i] = byte(i)
	}
	msg, err := buildSMTPMessage(smtpMessageOptions{
		From: "f@x.com", To: "t@x.com", Subject: "s", HTMLBody: "hi",
		Attachments: []*attachment.Resolved{{
			Filename: "blob.bin", MIMEType: "application/octet-stream",
			Disposition: "attachment", Bytes: payload,
		}},
	})
	require.NoError(t, err)

	parsed := parseEmail(t, msg)
	_, params, _ := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	parts := readParts(t, parsed.Body, params["boundary"])
	require.Len(t, parts, 2)

	// Every non-final line of the base64 payload should be exactly 76 chars.
	body := strings.TrimRight(string(parts[1].body), "\r\n")
	lines := strings.Split(body, "\r\n")
	require.GreaterOrEqual(t, len(lines), 2)
	for i := 0; i < len(lines)-1; i++ {
		assert.Equal(t, 76, len(lines[i]), "line %d not wrapped at 76 cols: %q", i, lines[i])
	}
}

// SMTPProvider.Send + AttachmentResolverKey -----------------------------------

func TestSMTPProvider_Send_ResolverInjected_AttachmentInMessage(t *testing.T) {
	logger := zaptest.NewLogger(t)
	provider, err := NewSMTPProvider(SMTPConfig{
		Config:    Config{Timeout: time.Second, MaxRetries: 0, RetryDelay: 0},
		Host:      "smtp.example.com",
		Port:      587,
		FromEmail: "from@example.com",
		FromName:  "Sender",
	}, logger)
	require.NoError(t, err)
	smtpProv := provider.(*SMTPProvider)

	var captured []byte
	smtpProv.sender = func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
		captured = msg
		return nil
	}

	// Resolver closure pre-bound (as the worker would do) — returns a
	// canned PDF byte slice regardless of input.
	resolver := AttachmentResolveFunc(func(_ context.Context, atts []notification.Attachment) ([]*attachment.Resolved, error) {
		return []*attachment.Resolved{{
			Filename:    "report.pdf",
			MIMEType:    "application/pdf",
			Disposition: "attachment",
			Bytes:       []byte("PDF-bytes-here"),
			Source:      attachment.SourceURL,
		}}, nil
	})

	ctx := context.WithValue(context.Background(), AttachmentResolverKey, resolver)
	notif := &notification.Notification{
		NotificationID: "n-1",
		Content: notification.Content{
			Title:       "Your report",
			Body:        "<p>Attached</p>",
			Attachments: []notification.Attachment{{URL: "https://example.com/r.pdf"}},
		},
	}
	usr := &user.User{UserID: "u1", Email: "user@example.com"}

	result, sendErr := provider.Send(ctx, notif, usr)
	require.NoError(t, sendErr)
	require.True(t, result.Success)
	require.NotNil(t, captured)

	s := string(captured)
	assert.Contains(t, s, "multipart/mixed")
	assert.Contains(t, s, `filename="report.pdf"`)
	assert.Contains(t, s, "Content-Type: application/pdf")
}

func TestSMTPProvider_Send_NoResolver_DropsAttachmentsLoudly(t *testing.T) {
	logger := zaptest.NewLogger(t)
	provider, _ := NewSMTPProvider(SMTPConfig{
		Config: Config{Timeout: time.Second}, Host: "h", FromEmail: "f@x.com",
	}, logger)
	smtpProv := provider.(*SMTPProvider)

	var captured []byte
	smtpProv.sender = func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
		captured = msg
		return nil
	}
	notif := &notification.Notification{
		Content: notification.Content{
			Title: "s", Body: "b",
			Attachments: []notification.Attachment{{URL: "https://x/y.pdf"}},
		},
	}
	usr := &user.User{Email: "user@example.com"}

	// No resolver on ctx; send must succeed and produce a flat text/html
	// message (legacy fast path).
	result, err := provider.Send(context.Background(), notif, usr)
	require.NoError(t, err)
	require.True(t, result.Success)
	assert.NotContains(t, string(captured), "multipart/")
}

// test helpers ----------------------------------------------------------------

type rawPart struct {
	header mail.Header
	body   []byte
}

func parseEmail(t *testing.T, raw []byte) *mail.Message {
	t.Helper()
	m, err := mail.ReadMessage(strings.NewReader(string(raw)))
	require.NoError(t, err)
	return m
}

func readParts(t *testing.T, body interface{}, boundary string) []rawPart {
	t.Helper()
	var reader *multipart.Reader
	switch v := body.(type) {
	case *strings.Reader:
		reader = multipart.NewReader(v, boundary)
	default:
		// io.Reader
		if r, ok := body.(interface{ Read([]byte) (int, error) }); ok {
			reader = multipart.NewReader(r, boundary)
		} else {
			t.Fatalf("readParts: unsupported body type %T", body)
		}
	}
	var out []rawPart
	for {
		p, err := reader.NextPart()
		if err != nil {
			break
		}
		buf := make([]byte, 0, 1024)
		tmp := make([]byte, 1024)
		for {
			n, rErr := p.Read(tmp)
			if n > 0 {
				buf = append(buf, tmp[:n]...)
			}
			if rErr != nil {
				break
			}
		}
		out = append(out, rawPart{
			header: mail.Header(p.Header),
			body:   buf,
		})
	}
	return out
}
