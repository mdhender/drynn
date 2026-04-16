package service

import "github.com/mdhender/drynn/internal/email"

func mailgunConfigured(cfg email.MailgunConfig) bool {
	return cfg.APIKey != "" && cfg.SendingDomain != "" && cfg.FromAddress != ""
}
