package email

import (
	"context"
	"fmt"
	"time"

	"github.com/mailgun/mailgun-go/v4"
)

type MailgunConfig struct {
	APIKey        string
	SendingDomain string
	FromAddress   string
	FromName      string
}

func (c MailgunConfig) Configured() bool {
	return c.APIKey != "" && c.SendingDomain != "" && c.FromAddress != ""
}

func (c MailgunConfig) from() string {
	if c.FromName != "" {
		return fmt.Sprintf("%s <%s>", c.FromName, c.FromAddress)
	}
	return c.FromAddress
}

func Send(ctx context.Context, cfg MailgunConfig, to, subject, htmlBody string) error {
	mg := mailgun.NewMailgun(cfg.SendingDomain, cfg.APIKey)
	msg := mailgun.NewMessage(cfg.from(), subject, "", to)
	msg.SetHTML(htmlBody)

	sendCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if _, _, err := mg.Send(sendCtx, msg); err != nil {
		return fmt.Errorf("mailgun send: %w", err)
	}

	return nil
}
