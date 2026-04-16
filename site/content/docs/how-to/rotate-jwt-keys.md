---
title: Rotate JWT keys
weight: 60
---

JWT signing keys should be rotated periodically and after any
suspected compromise. Drynn manages key lifecycle through the `db`
CLI — the server loads keys from the database at startup and on each
request, so rotation requires no downtime.

## How key rotation works

Each token type (`access` and `refresh`) has exactly one **active**
signing key. When you create a new key, the previous active key is
**retired** with a verification grace period. During the grace
period, existing tokens signed by the old key are still accepted.
After the grace period, the old key stops verifying and those tokens
are effectively revoked.

## Rotate the access key

```sh
go run ./cmd/db jwt-key create -type access
```

This creates a new active key and retires the old one. The grace
period defaults to the access token TTL (15 minutes by default), so
all existing access tokens expire naturally before the old key stops
verifying.

## Rotate the refresh key

```sh
go run ./cmd/db jwt-key create -type refresh
```

The default grace period for refresh keys is the refresh token TTL
(7 days). Users with existing refresh tokens can still use them
during this window. After the grace period, they will need to sign
in again.

## Custom grace period

Override the default with `-verify-old-for`:

```sh
go run ./cmd/db jwt-key create -type access -verify-old-for 1h
```

Set it to `0` to immediately invalidate all existing tokens of that
type — useful after a suspected compromise.

## Force-expire a specific key

If you need to retire a key outside of normal rotation:

```sh
go run ./cmd/db jwt-key expire -id <key-uuid> -verify-for 30m
```

This retires the key and allows verification for 30 more minutes.
Omit `-verify-for` to stop verification immediately.

## Delete old keys

Retired keys that have passed their grace period can be removed:

```sh
go run ./cmd/db jwt-key delete -id <key-uuid>
```

Active keys cannot be deleted — create a replacement first.

## Recommended schedule

| Environment | Access key | Refresh key |
|-------------|-----------|-------------|
| Production  | Monthly   | Quarterly   |
| After incident | Immediately, with grace period `0` | Immediately, with grace period `0` |
| Development | As needed | As needed |
