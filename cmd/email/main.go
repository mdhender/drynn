package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	drynn "github.com/mdhender/drynn"
	"github.com/mdhender/drynn/internal/config"
	"github.com/mdhender/drynn/internal/email"
)

func main() {
	log.SetFlags(0)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "send":
		err = runSend(ctx, os.Args[2:])
	case "version":
		fmt.Println(drynn.Version().Core())
		return
	case "help", "-h", "--help":
		usage()
		return
	default:
		usage()
		log.Fatalf("unknown command %q", os.Args[1])
	}

	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		log.Fatal(err)
	}
}

func runSend(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("send", flag.ContinueOnError)
	configPath := fs.String("config", config.DefaultPath(), "path to the server config file")
	to := fs.String("to", "", "recipient email address")
	subject := fs.String("subject", "", "message subject")
	body := fs.String("body", "", "HTML message body (mutually exclusive with -body-file)")
	bodyFile := fs.String("body-file", "", "path to a file containing the HTML message body")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: %s send [flags]\n", os.Args[0])
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	if strings.TrimSpace(*to) == "" {
		return fmt.Errorf("to is required")
	}
	if strings.TrimSpace(*subject) == "" {
		return fmt.Errorf("subject is required")
	}
	if *body != "" && *bodyFile != "" {
		return fmt.Errorf("specify only one of -body or -body-file")
	}

	htmlBody := *body
	if *bodyFile != "" {
		data, err := os.ReadFile(*bodyFile)
		if err != nil {
			return fmt.Errorf("read body file: %w", err)
		}
		htmlBody = string(data)
	}
	if strings.TrimSpace(htmlBody) == "" {
		return fmt.Errorf("body is required (use -body or -body-file)")
	}

	cfg, err := config.LoadPath(*configPath)
	if err != nil {
		return err
	}
	if !cfg.Mailgun.Configured() {
		return fmt.Errorf("mailgun is not configured in %s", cfg.ConfigPath)
	}

	mailgunCfg := email.MailgunConfig{
		APIKey:        cfg.Mailgun.APIKey,
		SendingDomain: cfg.Mailgun.SendingDomain,
		FromAddress:   cfg.Mailgun.FromAddress,
		FromName:      cfg.Mailgun.FromName,
	}

	if err := email.Send(ctx, mailgunCfg, *to, *subject, htmlBody); err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	fmt.Printf("sent message to %s\n", *to)
	return nil
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %s <command> [flags]\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "commands:")
	fmt.Fprintln(os.Stderr, "  send     send an HTML email via Mailgun")
	fmt.Fprintln(os.Stderr, "  version  print the build version")
}
