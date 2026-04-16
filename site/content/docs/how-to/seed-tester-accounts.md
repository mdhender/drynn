---
title: Seed tester accounts
weight: 80
---

Tester accounts let you populate the app with fake users for
development and testing without sending real invitations. They are
created with a sentinel password hash that prevents sign-in until
you set a real password.

## Create tester accounts

```sh
go run ./cmd/db seed-testers -count 5
```

This creates accounts named `tester_1` through `tester_5` with
email addresses at the `drynn.test` domain (e.g.
`tester_1@drynn.test`). Each account is assigned the `user` and
`tester` roles.

The command is idempotent — if you later run it with `-count 10`,
it creates only the five missing accounts.

## Set passwords

Tester accounts cannot sign in until you set a real password.
Use the CLI:

```sh
go run ./cmd/db set-password \
  -email tester_1@drynn.test \
  -password testpass123
```

Or set passwords through the admin UI at
`/app/admin/users/:id/edit`.

## Sign in as a tester

Once a password is set, sign in normally at `/signin` with the
tester's email and password.

## Identify tester accounts

Tester accounts have two distinguishing features:

- The `tester` role, visible in the admin user list and on the
  profile page.
- Email addresses ending in `@drynn.test`, which is a reserved
  domain that does not receive real mail.

## Clean up

Tester accounts can be deleted through the admin panel at
`/app/admin/users`. There is no bulk-delete CLI command — delete
them individually through the admin UI or write a direct SQL
statement if you need to remove many at once.
