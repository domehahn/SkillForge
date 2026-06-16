package email

import (
	"fmt"
	"net/mail"
	"net/smtp"
	"strings"

	"github.com/skillforge/skill-registry/internal/config"
)

// Sender sends transactional registry emails.
type Sender struct {
	cfg config.EmailConfig
}

// NewSender creates an SMTP-backed sender.
func NewSender(cfg config.EmailConfig) *Sender {
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &Sender{cfg: cfg}
}

// Enabled reports whether SMTP delivery is configured.
func (s *Sender) Enabled() bool {
	return s != nil && s.cfg.SMTPEnabled
}

// VerificationURL builds the public verification URL for a token.
func (s *Sender) VerificationURL(token string) string {
	baseURL := s.cfg.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return baseURL + "/api/v1/auth/verify-email?token=" + token
}

// SendVerification sends an email verification link.
func (s *Sender) SendVerification(to, username, token string) error {
	if !s.Enabled() {
		return fmt.Errorf("SMTP email delivery is disabled")
	}
	if to == "" {
		return fmt.Errorf("recipient email is required")
	}
	subject := "Verify your SkillForge account"
	body := fmt.Sprintf("Hello %s,\n\nVerify your SkillForge account by opening this link:\n\n%s\n\nThis link expires in 24 hours.\n", username, s.VerificationURL(token))
	return s.send(to, subject, body)
}

func (s *Sender) send(to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.SMTPHost, s.cfg.SMTPPort)
	headers := []string{
		"From: " + s.cfg.From,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
	}
	message := strings.Join(headers, "\r\n") + "\r\n\r\n" + body

	var auth smtp.Auth
	if s.cfg.SMTPUsername != "" || s.cfg.SMTPPassword != "" {
		auth = smtp.PlainAuth("", s.cfg.SMTPUsername, s.cfg.SMTPPassword, s.cfg.SMTPHost)
	}
	from := s.cfg.From
	if parsed, err := mail.ParseAddress(s.cfg.From); err == nil {
		from = parsed.Address
	}
	return smtp.SendMail(addr, auth, from, []string{to}, []byte(message))
}
