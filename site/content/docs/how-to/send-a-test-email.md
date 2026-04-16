---
title: Send a test email
weight: 70
---

Use the `email` CLI to verify that Mailgun is configured correctly
before relying on invitation or password-reset emails in production.

## Before you begin

Mailgun credentials must be present in your config file or
environment. The required settings are:

- `MAILGUN_API_KEY` — your Mailgun private API key
- `MAILGUN_SENDING_DOMAIN` — the domain Mailgun sends from
  (e.g. `mg.example.com`)
- `MAILGUN_FROM_ADDRESS` — the sender address
  (e.g. `noreply@example.com`)

Set these either in `server.json` (via `init-config`) or as
environment variables. The `email` CLI reads the same config file as
the server.

## Send a test message

```sh
go run ./cmd/email send \
  -to you@example.com \
  -subject "Drynn test email" \
  -body "<h1>It works</h1><p>Mailgun is configured correctly.</p>"
```

On success, the command prints:

```
sent message to you@example.com
```

## Send from a file

For longer HTML bodies, use `-body-file` instead of `-body`:

```sh
go run ./cmd/email send \
  -to you@example.com \
  -subject "Drynn test email" \
  -body-file test-email.html
```

The flags `-body` and `-body-file` are mutually exclusive.

## Use a non-default config path

If your config file is not at the default location:

```sh
go run ./cmd/email send \
  -config /etc/drynn/server.json \
  -to you@example.com \
  -subject "Test" \
  -body "<p>Hello</p>"
```

## Troubleshooting

| Symptom | Cause |
|---------|-------|
| `mailgun is not configured` | One or more of the three required Mailgun settings is empty. Check your config file. |
| `send email: ...` | Mailgun rejected the request. Verify the API key and sending domain are correct and that the domain is verified in the Mailgun dashboard. |
| Email sent but not received | Check spam folders. Verify DNS records (SPF, DKIM, MX) for your sending domain in the Mailgun console. |
