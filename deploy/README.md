# Deployment Files

This directory contains production deployment artifacts for running drynn on a DigitalOcean droplet (Ubuntu 24.04 LTS) with DigitalOcean Managed PostgreSQL.

For the full step-by-step guide, see [`../project/DEPLOYING.md`](../project/DEPLOYING.md).

## Contents

| File | Install Location | Description |
|------|-----------------|-------------|
| `drynn.service` | `/etc/systemd/system/drynn.service` | Systemd unit with full sandbox hardening |
| `drynn.env.example` | `/etc/drynn/drynn.env` | Optional environment overrides for systemd |
| `nginx-drynn.conf` | `/etc/nginx/sites-available/drynn` | Nginx reverse proxy with TLS, security headers, gzip |
| `deploy.sh` | Run from workstation | Build + upload + migrate + restart script |

## Quick Start

After initial host provisioning (see DEPLOYING.md §1–5):

```bash
# First deploy — install systemd unit and nginx config
scp deploy/drynn.service root@yourhost:/etc/systemd/system/drynn.service
scp deploy/nginx-drynn.conf root@yourhost:/etc/nginx/sites-available/drynn

# Subsequent deploys — build, upload, migrate, restart
./deploy/deploy.sh yourhost.example.com
```

## Architecture

```
        Internet
           |
           v
  +------------------+        +---------------------------+
  |  Nginx (443/80)  | -----> |  drynn (127.0.0.1:8080) |
  +------------------+        +---------------------------+
                                           |
                                           v
                          DigitalOcean Managed PostgreSQL
```

## Customization

Before deploying, replace the following placeholders:

- **`hobo.example.com`** in `nginx-drynn.conf` — your actual domain name
- **`ops@example.com`** in certbot commands — your email for Let's Encrypt notifications
- **Database credentials** in `drynn.env` or `server.json` — your managed PostgreSQL connection string

## Security Notes

- The Go binary binds to `127.0.0.1:8080` only — never exposed directly to the internet.
- The systemd unit runs as an unprivileged `drynn` user with strict sandboxing (`ProtectSystem=strict`, `NoNewPrivileges=true`, `MemoryDenyWriteExecute=true`, etc.).
- The config file (`server.json`) is `0600` owned by `drynn` because it contains `DATABASE_URL`.
- The environment file (`drynn.env`) is `0640` owned by `root:drynn`.
- TLS terminates at Nginx. The application never handles raw TLS.
