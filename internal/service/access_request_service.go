package service

import (
	"context"
	"fmt"
	"html/template"
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
	if !s.mailgun.Configured() {
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
	body, err := buildAccessRequestBody(normalizedEmail, reason, strings.TrimSpace(input.IP))
	if err != nil {
		return fmt.Errorf("render access request email: %w", err)
	}

	if err := email.Send(ctx, s.mailgun, s.adminEmail, subject, body); err != nil {
		return fmt.Errorf("send access request email: %w", err)
	}

	return nil
}

func buildAccessRequestBody(addr, reason, ip string) (string, error) {
	reasonHTML := template.HTML(strings.ReplaceAll(template.HTMLEscapeString(reason), "\n", "<br>"))
	return email.RenderTemplate("access_request.gohtml", struct {
		Email      string
		IP         string
		Reason     string
		ReasonHTML template.HTML
	}{
		Email:      addr,
		IP:         ip,
		Reason:     reason,
		ReasonHTML: reasonHTML,
	})
}
