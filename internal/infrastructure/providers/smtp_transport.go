package providers

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"time"
)

func smtpTLSConfig(cfg SMTPConfig) *tls.Config {
	config := &tls.Config{MinVersion: tls.VersionTLS12}
	if cfg.TLSConfig != nil {
		config = cfg.TLSConfig.Clone()
	}
	config.ServerName = cfg.Host
	return config
}

func smtpOperationError(ctx context.Context, stage, source string, err error) error {
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		err = ctx.Err()
	} else if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
		err = context.DeadlineExceeded
	}
	category := ""
	if stage == "auth" && classifyDeliveryCause("smtp", err) == ErrorTypeUnknown {
		// These exact local PlainAuth rejections cannot improve on reconnect.
		// Other unknown causes, especially EOF while reading an AUTH reply,
		// must keep their existing retry policy.
		switch err.Error() {
		case "unencrypted connection", "wrong host name":
			category = ErrorTypeAuth
		}
	}
	return newDeliveryError("smtp", stage, source, category, err)
}

// openSMTP owns a deadline-bound socket. Closing it on cancellation interrupts
// every net/smtp operation, including greeting reads and DATA writes.
func (p *SMTPProvider) openSMTP(ctx context.Context, cfg SMTPConfig, source string) (*smtp.Client, func(), error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(cfg.Host, fmt.Sprint(cfg.Port)))
	if err != nil {
		return nil, nil, smtpOperationError(ctx, "connect", source, err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	closeConn := func() { stop(); conn.Close() }
	if cfg.Port == 465 {
		secure := tls.Client(conn, smtpTLSConfig(cfg))
		err = secure.HandshakeContext(ctx)
		if err != nil {
			closeConn()
			return nil, nil, smtpOperationError(ctx, "tls", source, err)
		}
		client, err := smtp.NewClient(secure, cfg.Host)
		if err != nil {
			closeConn()
			return nil, nil, smtpOperationError(ctx, "health", source, err)
		}
		return client, closeConn, nil
	}
	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		closeConn()
		return nil, nil, smtpOperationError(ctx, "health", source, err)
	}
	return client, closeConn, nil
}

func (p *SMTPProvider) sendSMTP(ctx context.Context, cfg SMTPConfig, source string, auth smtp.Auth, to []string, msg []byte) error {
	client, closeConn, err := p.openSMTP(ctx, cfg, source)
	if err != nil {
		return err
	}
	defer closeConn()
	if err = client.Hello("localhost"); err != nil {
		return smtpOperationError(ctx, "connect", source, err)
	}
	if cfg.Port != 465 {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err = client.StartTLS(smtpTLSConfig(cfg)); err != nil {
				return smtpOperationError(ctx, "tls", source, err)
			}
		}
	}
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return smtpOperationError(ctx, "auth", source, err)
		}
	}
	if err = client.Mail(cfg.FromEmail); err != nil {
		return smtpOperationError(ctx, "mail", source, err)
	}
	for _, recipient := range to {
		if err = client.Rcpt(recipient); err != nil {
			return smtpOperationError(ctx, "rcpt", source, err)
		}
	}
	writer, err := client.Data()
	if err != nil {
		return smtpOperationError(ctx, "data", source, err)
	}
	if _, err = writer.Write(msg); err != nil {
		return smtpOperationError(ctx, "data", source, err)
	}
	if err = writer.Close(); err != nil {
		return smtpOperationError(ctx, "data", source, err)
	}
	// DATA acceptance is the delivery boundary; a later QUIT failure must not
	// trigger another send of an already accepted message.
	_ = client.Quit()
	return nil
}
