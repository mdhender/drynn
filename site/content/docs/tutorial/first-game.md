---
title: "Your first game"
weight: 10
---

This tutorial walks you through setting up Drynn from scratch,
creating an admin account, inviting your first player, and
confirming everything works end-to-end. By the end you will have a
running server with one admin and one invited user.

## What you will need

- Go 1.22 or later
- PostgreSQL 15 or later, running and accessible
- [Atlas CLI](https://atlasgo.io/getting-started/) for database migrations
- A terminal and a web browser

## 1. Clone and build

```sh
git clone https://github.com/mdhender/drynn.git
cd drynn
go build ./...
```

This compiles both binaries (`server` and `db`) and verifies the
code builds cleanly. The binaries land in your `$GOBIN` or the
current directory depending on your Go configuration.

## 2. Create the database

Create a PostgreSQL database for the app:

```sh
createdb drynn
```

Set the connection string as an environment variable — every command
below reads it:

```sh
export DATABASE_URL="postgres://localhost:5432/drynn?sslmode=disable"
```

Adjust the host, port, and credentials to match your local setup.

## 3. Apply migrations

Run the Atlas migrations to create all tables:

```sh
atlas migrate apply --dir file://db/migrations --url "$DATABASE_URL"
```

You should see output confirming each migration file was applied.

## 4. Initialize the config file

The server reads its settings from a JSON config file. Generate the
default one:

```sh
go run ./cmd/db init-config -database-url "$DATABASE_URL" -base-url "http://localhost:8080"
```

This writes `data/var/drynn/server.json` with sensible defaults:
port 8080, 15-minute access tokens, 7-day refresh tokens, insecure
cookies (fine for local development).

## 5. Create JWT signing keys

The server refuses to start without active signing keys for both
access and refresh tokens. Create them:

```sh
go run ./cmd/db jwt-key create -type access
go run ./cmd/db jwt-key create -type refresh
```

Each command prints the new key's UUID. You do not need to save
these — the server loads them from the database at startup.

## 6. Seed the admin account

Create your bootstrap administrator:

```sh
go run ./cmd/db seed-admin \
  -handle admin \
  -email admin@example.com \
  -password changeme123
```

Pick a real password if this is anything beyond a throwaway local
instance. The handle must be lowercase alphanumeric (3–32 characters).

## 7. Start the server

```sh
go run ./cmd/server
```

The server starts on `http://localhost:8080`. Open that URL in your
browser — you should see the landing page.

## 8. Sign in as admin

Navigate to `/signin` and enter the admin credentials you created in
step 6. After signing in you land on `/app/profile`, which shows
your handle and email.

## 9. Invite a player

As an admin you can send invitations from the admin panel:

1. Go to `/app/admin/invitations`.
2. Click **Send Invitation** and enter the invitee's email address.
3. The app generates an invitation code. If Mailgun is configured,
   the invitee receives an email with a registration link. For local
   development without email, copy the invitation code from the
   admin list.

## 10. Register as the invited user

Open a private/incognito browser window so you are not signed in as
admin.

1. Navigate to `/register?code=<the-invitation-code>`.
2. Fill in a handle, confirm the email matches the invitation, and
   choose a password.
3. Submit. You are signed in automatically and redirected to
   `/app/profile`.

## 11. Verify the setup

Back in the admin window:

- `/app/admin/users` should list both the admin and the new user.
- The invitation on `/app/admin/invitations` should show as **Used**.

You now have a working Drynn instance with an admin and an invited
user — the minimum viable setup for running a game.

## Next steps

- [How to seed tester accounts](/docs/how-to/seed-tester-accounts/)
  for bulk user creation during development.
- [How to rotate JWT keys](/docs/how-to/rotate-jwt-keys/) before
  going to production.
- [Architecture overview](/docs/explanation/architecture/) for a
  deeper look at how the pieces fit together.
