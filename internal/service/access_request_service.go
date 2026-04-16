package service

import (
	"context"
	"fmt"
	"html"
	"strings"

	"github.com/mdhender/drynn/internal/email"
)

const maxAccessRequestReasonLen = 2000

type AccessRequestService struct {
	mailgun    email.MailgunConfig
	adminEmail string
}

func NewAccessRequestService(mailgun email.MailgunConfig, adminEmail string) *AccessRequestService {
	return &AccessRequestService{mailgun: mailgun, adminEmail: strings.TrimSpace(adminEmail)}
}

type AccessRequestInput struct {
	Email  string
	Reason string
	IP     string
}

func (s *AccessRequestService) Send(ctx context.Context, input AccessRequestInput) error {
	if s.adminEmail == "" {
		return ErrAccessRequestsDisabled
	}
	if !mailgunConfigured(s.mailgun) {
		return ErrMailgunNotConfigured
	}

	normalizedEmail, err := normalizeEmail(input.Email)
	if err != nil {
		return err
	}

	reason := strings.TrimSpace(input.Reason)
	if len(reason) > maxAccessRequestReasonLen {
		reason = reason[:maxAccessRequestReasonLen]
	}

	subject := fmt.Sprintf("Access request from %s", normalizedEmail)
	body := buildAccessRequestBody(normalizedEmail, reason, strings.TrimSpace(input.IP))

	if err := email.Send(ctx, s.mailgun, s.adminEmail, subject, body); err != nil {
		return fmt.Errorf("send access request email: %w", err)
	}

	return nil
}

func buildAccessRequestBody(email, reason, ip string) string {
	var b strings.Builder
	b.WriteString("<p>A visitor has requested access to Hobo.</p>")
	fmt.Fprintf(&b, "<p><strong>Email:</strong> %s</p>", html.EscapeString(email))
	if ip != "" {
		fmt.Fprintf(&b, "<p><strong>Client IP:</strong> %s</p>", html.EscapeString(ip))
	}
	if reason != "" {
		b.WriteString("<p><strong>Reason:</strong></p>")
		fmt.Fprintf(&b, "<p>%s</p>", strings.ReplaceAll(html.EscapeString(reason), "\n", "<br>"))
	}
	b.WriteString("<p>Review the request and, if appropriate, send an invitation from the admin console.</p>")
	return b.String()
}
