---
title: How to enable public access requests
weight: 20
---

This guide shows you how to turn on the opt-in `/request-access` form and
route submissions to an admin inbox. Use it when you want to let strangers
raise their hand without opening a public sign-up page.

For the reasoning behind the design — why the feature is opt-in, how it
relates to invitations, and what the defences do — see
[About public access requests](/docs/explanation/public-access-requests/).

## Before you begin

- Mailgun must already be configured on the deployment. The form will not
  send mail if `MAILGUN_API_KEY`, `MAILGUN_SENDING_DOMAIN`, and
  `MAILGUN_FROM_ADDRESS` are empty.
- Decide which mailbox will receive requests. A shared alias
  (`admins@example.com`) is usually a better choice than a single admin's
  personal address — it survives people going on leave or leaving the
  project.
- You need the ability to restart the Drynn server process. Both
  settings are read at startup; a running server will not notice a change
  until it is restarted.

## Configure the two settings

The feature only activates when *both* `request_access_enabled` is true
and `admin_contact_email` is non-empty. Pick whichever configuration path
matches your deployment.

### On a fresh install

Pass the flags to `cmd/db init-config` when you write the initial config
file:

```sh
go run ./cmd/db init-config \
  -database-url "$DATABASE_URL" \
  -base-url "https://drynn.example.com" \
  -request-access-enabled \
  -admin-contact-email admins@example.com
```

Omit `-request-access-enabled` to leave the feature off; the flag is a
bool and defaults to `false`.

### On an existing deployment

Export the env vars in whatever file the process manager sources
(`/etc/drynn/drynn.env`, a systemd drop-in, your shell `.env`, etc.):

```sh
REQUEST_ACCESS_ENABLED=true
ADMIN_CONTACT_EMAIL=admins@example.com
```

Env vars override values in `server.json`, so this works without touching
the on-disk config.

### By editing `server.json` directly

If you prefer the config file as the source of truth, add the two keys at
the top level:

```json
{
  "request_access_enabled": true,
  "admin_contact_email": "admins@example.com"
}
```

Leave the other fields alone. The server refuses to start on unknown
keys, so do not invent new ones.

## Restart the server

```sh
systemctl restart drynn    # or however you run it
```

## Verify

1. Open `/signin` in a private window. The footer should now read
   "Need an account? Request access." instead of the Discord fallback.
   If it still mentions Discord, the flag did not take effect — re-check
   the env vars and that the restart actually happened.
2. Follow the link (or go straight to `/request-access`) and submit a
   test entry with your own email and a short reason. The form should
   return a "thanks, we'll be in touch" page.
3. Check the admin inbox. A message titled `Access request from <email>`
   should arrive within a few seconds. If it does not, check the server
   logs for `request-access: send failed` — the form deliberately returns
   success even when the send fails, so the log is the only signal.

## Turning it off

Set `REQUEST_ACCESS_ENABLED=false` (or unset it) and restart. `/request-access`
will return 404 and the sign-in page will revert to the Discord fallback.
Clearing `ADMIN_CONTACT_EMAIL` has the same effect — the route refuses to
serve the form without a destination.
