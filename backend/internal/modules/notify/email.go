package notify

import (
	"context"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"

	notifyapi "github.com/portal/backend/internal/modules/notify/api"
)

// renderEmail turns a (type, data) pair into a subject + text body. Templates are
// intentionally plain text for v1 (Mailpit renders both); HTML is a later polish.
func renderEmail(typ, title, name, addr string, data map[string]any) EmailMessage {
	greeting := "Hello"
	if strings.TrimSpace(name) != "" {
		greeting = "Hello " + name
	}

	var subject, body string
	switch typ {
	case notifyapi.TypePasswordReset:
		resetURL := dataString(data, "reset_url")
		subject = "Reset your Portal password"
		body = fmt.Sprintf(
			"%s,\n\nWe received a request to reset your Portal password. "+
				"Use the link below to choose a new one. It expires shortly and can be used once.\n\n%s\n\n"+
				"If you didn't request this, you can safely ignore this email — your password won't change.\n",
			greeting, resetURL)
	case notifyapi.TypeSecurityAlert:
		subject = "Security alert on your Portal account"
		detail := dataString(data, "detail")
		if detail == "" {
			detail = "We revoked your active sessions as a precaution. If this wasn't you, reset your password now."
		}
		body = fmt.Sprintf("%s,\n\n%s\n", greeting, detail)
	case notifyapi.TypeMediaAssetReady:
		subject = "Your upload is ready"
		if strings.TrimSpace(title) != "" {
			subject = title
		}
		href := dataString(data, "href")
		body = fmt.Sprintf("%s,\n\nYour upload has finished processing and is ready to view.\n\n%s\n", greeting, href)
	default:
		subject = firstNonEmpty(title, "Notification from Portal")
		body = fmt.Sprintf("%s,\n\n%s\n", greeting, firstNonEmpty(title, "You have a new notification on Portal."))
	}

	return EmailMessage{To: addr, ToName: name, Subject: subject, TextBody: body}
}

// SMTPSender delivers over plain SMTP (dev: Mailpit on :1025, no auth). Prod may
// point SMTP_HOST/PORT at a relay or provider (§10).
type SMTPSender struct {
	addr string // host:port
	from string // From header, e.g. "Portal <no-reply@portal.localhost>"
}

// NewSMTPSender builds an SMTP transport. from is the RFC 5322 From value.
func NewSMTPSender(host string, port int, from string) *SMTPSender {
	return &SMTPSender{addr: net.JoinHostPort(host, strconv.Itoa(port)), from: from}
}

// Send transmits msg. No auth/STARTTLS for the dev Mailpit target; a prod relay
// that needs credentials is a §10 follow-up (auth arg is currently nil).
func (s *SMTPSender) Send(_ context.Context, msg EmailMessage) error {
	to := msg.To
	raw := buildRFC5322(s.from, to, msg)
	return smtp.SendMail(s.addr, nil, extractAddr(s.from), []string{to}, []byte(raw))
}

// LogSender writes the email to the log instead of sending it. Used in tests and
// as a no-SMTP dev fallback.
type LogSender struct{}

func (LogSender) Send(_ context.Context, msg EmailMessage) error {
	log.Info().Str("to", msg.To).Str("subject", msg.Subject).Msg("notify:email (log sink)")
	return nil
}

func buildRFC5322(from, to string, msg EmailMessage) string {
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + msg.Subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(strings.ReplaceAll(msg.TextBody, "\n", "\r\n"))
	return b.String()
}

// extractAddr pulls the bare address out of a "Name <addr>" From value for the
// SMTP envelope sender.
func extractAddr(from string) string {
	if i := strings.LastIndexByte(from, '<'); i >= 0 {
		if j := strings.IndexByte(from[i:], '>'); j > 0 {
			return from[i+1 : i+j]
		}
	}
	return from
}

func dataString(data map[string]any, key string) string {
	if data == nil {
		return ""
	}
	if v, ok := data[key].(string); ok {
		return v
	}
	return ""
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
