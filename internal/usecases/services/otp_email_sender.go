package services

import (
	"crypto/tls"
	"fmt"
	"net/smtp"

	"github.com/the-monkeys/freerangenotify/internal/config"
	"go.uber.org/zap"
)

// OTPEmailSender sends OTP verification emails directly via SMTP.
type OTPEmailSender struct {
	host      string
	port      int
	username  string
	password  string
	fromEmail string
	fromName  string
	logger    *zap.Logger
}

// NewOTPEmailSender creates a new OTP email sender from SMTP config.
func NewOTPEmailSender(cfg config.SMTPConfig, logger *zap.Logger) *OTPEmailSender {
	if cfg.Port == 0 {
		cfg.Port = 587
	}
	return &OTPEmailSender{
		host:      cfg.Host,
		port:      cfg.Port,
		username:  cfg.Username,
		password:  cfg.Password,
		fromEmail: cfg.FromEmail,
		fromName:  cfg.FromName,
		logger:    logger,
	}
}

// SendOTP sends a verification OTP email to the given address.
func (s *OTPEmailSender) SendOTP(toEmail, otpCode string) error {
	if s.host == "" {
		s.logger.Warn("SMTP not configured, skipping OTP email",
			zap.String("to", toEmail),
			zap.String("otp", otpCode))
		return nil
	}

	subject := "Your FreeRangeNotify Verification Code"
	body := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 480px; margin: 0 auto; padding: 40px 20px;">
  <h2 style="color: #1a1a1a; margin-bottom: 8px;">Verify your email</h2>
  <p style="color: #555; font-size: 15px;">Enter this code to complete your registration:</p>
  <div style="background: #f4f4f5; border-radius: 8px; padding: 20px; text-align: center; margin: 24px 0;">
    <span style="font-size: 32px; font-weight: 700; letter-spacing: 6px; color: #18181b;">%s</span>
  </div>
  <p style="color: #888; font-size: 13px;">This code expires in 10 minutes. If you didn't request this, ignore this email.</p>
</body>
</html>`, otpCode)

	msg := s.buildMessage(toEmail, subject, body)
	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	var auth smtp.Auth
	if s.username != "" && s.password != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}

	// Use TLS for port 465, STARTTLS for others
	var err error
	if s.port == 465 {
		err = s.sendWithTLS(addr, auth, s.fromEmail, toEmail, msg)
	} else {
		err = smtp.SendMail(addr, auth, s.fromEmail, []string{toEmail}, msg)
	}

	if err != nil {
		s.logger.Error("Failed to send OTP email",
			zap.String("to", toEmail),
			zap.Error(err))
		return fmt.Errorf("failed to send verification email: %w", err)
	}

	s.logger.Info("OTP email sent", zap.String("to", toEmail))
	return nil
}

func (s *OTPEmailSender) sendWithTLS(addr string, auth smtp.Auth, from, to string, msg []byte) error {
	tlsConfig := &tls.Config{ServerName: s.host}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS dial failed: %w", err)
	}

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("SMTP client creation failed: %w", err)
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}

	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	return client.Quit()
}

func (s *OTPEmailSender) buildMessage(to, subject, body string) []byte {
	headers := fmt.Sprintf("From: %s <%s>\r\n", s.fromName, s.fromEmail)
	headers += fmt.Sprintf("To: %s\r\n", to)
	headers += fmt.Sprintf("Subject: %s\r\n", subject)
	headers += "MIME-Version: 1.0\r\n"
	headers += "Content-Type: text/html; charset=\"UTF-8\"\r\n"
	headers += "\r\n"

	return []byte(headers + body)
}
