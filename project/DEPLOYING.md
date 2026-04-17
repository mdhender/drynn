# Deploying Hobo

Production deployment runbook for Hobo on an Ubuntu 24.04 LTS droplet at DigitalOcean, backed by DigitalOcean Managed PostgreSQL and fronted by Nginx with Let's Encrypt TLS.

This guide assumes a single-droplet install. Commands marked with `sudo` run as root on the droplet; commands without it can run as an unprivileged admin user with `sudo` rights.

## Target architecture

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

- The Go binary listens only on `127.0.0.1:8080`; all public traffic flows through Nginx.
- TLS terminates at Nginx. The application never sees plain TLS sockets.
- Postgres runs as a DigitalOcean Managed Database; the droplet connects over TLS (`sslmode=require`).
- Systemd runs the service as an unprivileged system user.

## Prerequisites

Before starting you need:

- A DigitalOcean droplet: Ubuntu 24.04 LTS, 1 GB RAM minimum, in the same region as your managed database.
- A DigitalOcean Managed PostgreSQL cluster (any supported version).
- A DNS A record pointing your domain (`hobo.example.com`) at the droplet's public IP.
- SSH access to the droplet as root (or a sudoer).
- The drynn source tree on your workstation, at the commit you want to deploy.
- Local `go`, `atlas`, and `sqlc` installed (we build on the workstation and ship the binary).

## 1. Base droplet hardening

Log in as `root`, then:

```bash
apt update
apt upgrade -y
apt install -y ufw ca-certificates curl gnupg lsb-release
timedatectl set-timezone UTC
```

Enable the firewall. We allow SSH, HTTP, and HTTPS only:

```bash
ufw allow OpenSSH
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable
ufw status
```

## 2. Create the service user

Hobo runs as a dedicated non-login system user. It owns its runtime state directory but does not own the release directory, so a compromised process cannot overwrite the binary or templates.

```bash
sudo adduser \
  --system \
  --group \
  --home /var/lib/drynn \
  --no-create-home \
  --shell /usr/sbin/nologin \
  drynn
```

Verify:

```bash
id drynn
# uid=998(drynn) gid=998(drynn) groups=998(drynn)
```

## 3. Create the directory layout

Hobo uses three top-level directories:

| Path | Owner | Mode | Purpose |
|------|-------|------|---------|
| `/opt/drynn` | `root:root` | `0755` | Immutable release: binary, web assets, migrations. Read-only to the service. |
| `/var/lib/drynn` | `drynn:drynn` | `0750` | Writable runtime state: config file, data directory. |
| `/etc/drynn` | `root:drynn` | `0750` | Optional environment file for systemd. |

Create them:

```bash
sudo install -d -o root    -g root    -m 0755 /opt/drynn
sudo install -d -o root    -g root    -m 0755 /opt/drynn/bin
sudo install -d -o root    -g root    -m 0755 /opt/drynn/web
sudo install -d -o root    -g root    -m 0755 /opt/drynn/db
sudo install -d -o drynn -g drynn -m 0750 /var/lib/drynn
sudo install -d -o drynn -g drynn -m 0750 /var/lib/drynn/config
sudo install -d -o drynn -g drynn -m 0750 /var/lib/drynn/data
sudo install -d -o root    -g drynn -m 0750 /etc/drynn
```

> **Note:** The server loads templates via `os.DirFS(".")` with paths like `web/templates/...`. The systemd unit sets `WorkingDirectory=/opt/drynn`, so the `web/` tree must live at `/opt/drynn/web/`.

## 4. Install the Atlas CLI on the droplet

Atlas runs the database migrations against the managed cluster. Install the static release binary:

```bash
curl -sSf https://atlasgo.sh | sh
# Installs /usr/local/bin/atlas
atlas version
```

## 5. Provision the managed database

In the DigitalOcean control panel, on your Managed PostgreSQL cluster:

1. **Create a database:** `drynn_prod`.
2. **Create a role:** `drynn_prod_user` with a strong password. Grant it ownership of `drynn_prod`.
3. **Trusted sources:** add your droplet (by name or public IP) so the cluster firewall accepts its connections.
4. **Download the CA certificate:** DigitalOcean provides a `ca-certificate.crt` for TLS verification. Upload it to the droplet:
   ```bash
   sudo install -d -o root -g root -m 0755 /etc/drynn/pki
   sudo install -o root -g drynn -m 0640 ca-certificate.crt /etc/drynn/pki/managed-pg-ca.crt
   ```

Construct the connection string. The cluster detail page shows the host, port (usually `25060`), and database name:

```
postgres://drynn_prod_user:STRONG_PASSWORD@db-drynn-nyc3-00000-do-user-000000-0.b.db.ondigitalocean.com:25060/drynn_prod?sslmode=verify-full&sslrootcert=/etc/drynn/pki/managed-pg-ca.crt
```

Export it in the shell you'll use for the bootstrap steps (do **not** put it in your shell history rc files):

```bash
export DATABASE_URL='postgres://drynn_prod_user:STRONG_PASSWORD@HOST:25060/drynn_prod?sslmode=verify-full&sslrootcert=/etc/drynn/pki/managed-pg-ca.crt'
```

Test connectivity:

```bash
psql "$DATABASE_URL" -c 'SELECT version();'
```

(Install `postgresql-client` with apt if you don't already have `psql`.)

## 6. Build and upload the release

On your **workstation**, from the repo root:

```bash
# Cross-compile a stripped binary for the droplet.
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags='-s -w' -o build/drynn-server ./cmd/server

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags='-s -w' -o build/drynn-db ./cmd/db
```

Ship the binary, web assets, and migration files. Replace `hobo.example.com` with your droplet's host or DNS name:

```bash
HOST=hobo.example.com

# Binaries (to a staging location, then move into /opt/drynn as root).
scp build/drynn-server build/drynn-db "$HOST":/tmp/

# Web assets and migrations.
rsync -a --delete web/ "$HOST":/tmp/drynn-web/
rsync -a --delete db/migrations/ "$HOST":/tmp/drynn-migrations/
```

On the **droplet**, install the payload:

```bash
sudo install -o root -g root -m 0755 /tmp/drynn-server /opt/drynn/bin/server
sudo install -o root -g root -m 0755 /tmp/drynn-db     /opt/drynn/bin/drynn-db

sudo rsync -a --delete /tmp/drynn-web/        /opt/drynn/web/
sudo rsync -a --delete /tmp/drynn-migrations/ /opt/drynn/db/migrations/

sudo chown -R root:root /opt/drynn
sudo find /opt/drynn -type d -exec chmod 0755 {} \;
sudo find /opt/drynn -type f -exec chmod 0644 {} \;
sudo chmod 0755 /opt/drynn/bin/server /opt/drynn/bin/drynn-db

rm /tmp/drynn-server /tmp/drynn-db
rm -rf /tmp/drynn-web /tmp/drynn-migrations
```

## 7. Apply database migrations

Still on the droplet, with `DATABASE_URL` exported:

```bash
atlas migrate apply \
  --dir file:///opt/drynn/db/migrations \
  --url "$DATABASE_URL"
```

Atlas prints each migration it applies. Re-running is idempotent.

## 8. Write the server config file

Use `drynn-db init-config` to write `/var/lib/drynn/config/server.json`. Run it as the `drynn` user so the file lands with the right owner:

```bash
sudo -u drynn /opt/drynn/bin/drynn-db init-config \
  --config /var/lib/drynn/config/server.json \
  --database-url "$DATABASE_URL" \
  --base-url "https://drynn.example.com" \
  --data-dir /var/lib/drynn/data \
  --app-addr 127.0.0.1:8080 \
  --cookie-secure true \
  --jwt-access-ttl 15m \
  --jwt-refresh-ttl 168h

sudo chmod 0600 /var/lib/drynn/config/server.json
sudo chown drynn:drynn /var/lib/drynn/config/server.json
```

Key choices:

- `--app-addr 127.0.0.1:8080` — bind only to loopback. Nginx proxies to it.
- `--cookie-secure true` — sets the `Secure` flag on auth cookies. Required once TLS is in front.
- `--data-dir /var/lib/drynn/data` — writable runtime files live under the state directory.

## 9. Create JWT signing keys

The server refuses to start without one active signing key for each token type.

```bash
sudo -u drynn /opt/drynn/bin/drynn-db jwt-key create \
  --config /var/lib/drynn/config/server.json --type access

sudo -u drynn /opt/drynn/bin/drynn-db jwt-key create \
  --config /var/lib/drynn/config/server.json --type refresh
```

## 10. Seed the admin account

Use `seed-admin` once to create the first administrator. Pick a strong password and change it after first sign-in.

```bash
sudo -u drynn /opt/drynn/bin/drynn-db seed-admin \
  --config /var/lib/drynn/config/server.json \
  --handle admin \
  --email admin@example.com \
  --password 'CHANGE-ME-AFTER-SIGNIN'
```


## 11. Install the systemd service

Write `/etc/systemd/system/drynn.service`:

```ini
[Unit]
Description=Hobo web application
Documentation=https://github.com/mdhender/drynn
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=drynn
Group=drynn

WorkingDirectory=/opt/drynn
ExecStart=/opt/drynn/bin/server --config /var/lib/drynn/config/server.json

Restart=on-failure
RestartSec=5s
TimeoutStopSec=15s

# Logging: use the journal.
StandardOutput=journal
StandardError=journal
SyslogIdentifier=drynn

# Sandboxing.
NoNewPrivileges=true
PrivateTmp=true
PrivateDevices=true
ProtectSystem=strict
ProtectHome=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectKernelLogs=true
ProtectControlGroups=true
ProtectClock=true
ProtectHostname=true
ProtectProc=invisible
RestrictNamespaces=true
RestrictRealtime=true
RestrictSUIDSGID=true
LockPersonality=true
MemoryDenyWriteExecute=true
SystemCallArchitectures=native
SystemCallFilter=@system-service
SystemCallFilter=~@privileged @resources

# /var/lib/drynn must be writable; everything else is read-only.
ReadWritePaths=/var/lib/drynn

CapabilityBoundingSet=
AmbientCapabilities=

# Resource limits.
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

Enable and start it:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now drynn.service
sudo systemctl status drynn.service
```

Tail the logs while you sanity-check:

```bash
sudo journalctl -u drynn -f
```

Verify the service is listening on loopback:

```bash
ss -tlnp | grep 8080
curl -sI http://127.0.0.1:8080/
```

## 12. Install and configure Nginx

```bash
sudo apt install -y nginx
sudo systemctl enable --now nginx
```

Disable the default site:

```bash
sudo rm -f /etc/nginx/sites-enabled/default
```

Write `/etc/nginx/sites-available/drynn`:

```nginx
# HTTP: Let's Encrypt will rewrite this block when it installs the cert,
# or you can leave it as a redirect stub. Certbot handles the upgrade cleanly.
server {
    listen 80;
    listen [::]:80;
    server_name hobo.example.com;

    # ACME challenges are served from here by certbot's nginx plugin.
    location /.well-known/acme-challenge/ {
        root /var/www/html;
    }

    location / {
        return 301 https://$host$request_uri;
    }
}

server {
    listen 443 ssl;
    listen [::]:443 ssl;
    http2 on;
    server_name hobo.example.com;

    # These paths are populated by certbot after the first issuance.
    ssl_certificate     /etc/letsencrypt/live/hobo.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/hobo.example.com/privkey.pem;
    include /etc/letsencrypt/options-ssl-nginx.conf;
    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;

    # Security headers.
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "DENY" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;

    # Request body limit — bump if you add upload features.
    client_max_body_size 2m;

    # Gzip.
    gzip on;
    gzip_types text/plain text/css application/javascript application/json image/svg+xml;
    gzip_min_length 1024;

    access_log /var/log/nginx/drynn.access.log;
    error_log  /var/log/nginx/drynn.error.log;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;

        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host  $host;

        proxy_read_timeout  60s;
        proxy_send_timeout  60s;
        proxy_connect_timeout 5s;

        proxy_buffering on;
    }
}
```

Enable the site and reload:

```bash
sudo ln -s /etc/nginx/sites-available/drynn /etc/nginx/sites-enabled/drynn
sudo nginx -t
sudo systemctl reload nginx
```

At this point port 443 will fail — we haven't issued certificates yet. Certbot fixes that next.

## 13. Obtain Let's Encrypt certificates

Install certbot's nginx plugin:

```bash
sudo apt install -y certbot python3-certbot-nginx
```

Issue the certificate. Certbot reuses the nginx config you already wrote and populates the cert paths automatically:

```bash
sudo certbot --nginx \
  -d hobo.example.com \
  --non-interactive \
  --agree-tos \
  --email ops@example.com \
  --redirect
```

`--redirect` ensures the port-80 server block 301s to HTTPS. `--non-interactive` requires `--agree-tos` and `--email`.

Certbot installs a systemd timer that renews certificates automatically. Confirm:

```bash
systemctl list-timers | grep certbot
sudo certbot renew --dry-run
```

## 14. Smoke test

From your workstation:

```bash
curl -sI https://hobo.example.com/
# Expect: HTTP/2 200
```

Open `https://hobo.example.com/signin` in a browser and sign in as `admin@example.com` with the password you set in step 10. Change the password immediately via the profile page.

## 15. Operational notes

### Logs

```bash
sudo journalctl -u drynn -f           # Follow service logs
sudo journalctl -u drynn --since '1h ago'
sudo tail -f /var/log/nginx/drynn.access.log
sudo tail -f /var/log/nginx/drynn.error.log
```

### Deploying a new release

Repeat step 6 (build + upload + install) and step 7 (apply migrations), then restart the service:

```bash
sudo systemctl restart drynn.service
sudo journalctl -u drynn -n 50
```

The restart is near-instant but it does drop in-flight connections. For a zero-downtime upgrade you would need a second instance and an Nginx upstream with two backends — out of scope for the MVP.

### JWT key rotation

```bash
sudo -u drynn /opt/drynn/bin/drynn-db jwt-key create \
  --config /var/lib/drynn/config/server.json --type access
```

This generates a new active key and retires the previous one with a grace window equal to the current access TTL. Repeat for `--type refresh` on your preferred schedule.

### Mailgun settings

Outbound email (invitations, password reset) requires Mailgun credentials. Phase 3 of the burndown moves these into `server.json`; until then the email driver is stubbed and outbound email is non-functional.

### Backups

DigitalOcean Managed PostgreSQL performs daily backups automatically; verify retention on the cluster settings page. The droplet itself holds no durable state beyond `/var/lib/drynn/config/server.json` (which contains `DATABASE_URL` — the only non-recreatable secret on the host).

### Upgrading Ubuntu packages

```bash
sudo apt update && sudo apt upgrade -y
sudo systemctl restart drynn.service nginx
```

Unattended-upgrades is recommended for security patches; configure with `sudo dpkg-reconfigure --priority=low unattended-upgrades`.

### Known deployment gotchas

- **Working directory is load-bearing.** The template renderer resolves paths relative to the process working directory. Do not change `WorkingDirectory=` in the systemd unit without also moving `web/`.
- **Trusted proxy headers.** `requestBaseURL` honors `X-Forwarded-Proto` and `X-Forwarded-Host` for building outbound links in emails. These headers are only safe to trust because the service binds to `127.0.0.1:8080` and all traffic flows through the local Nginx. Do not expose the application port externally, or a client could spoof its own base URL for invitation and password-reset links.
- **Config file permissions.** `server.json` contains the raw `DATABASE_URL`. It must remain `0600` owned by `drynn`. Re-run `chmod 0600` after any edit.
- **Firewall.** Only 22, 80, and 443 should be reachable from the internet. The app port (`8080`) binds to loopback and must never be exposed via `ufw`.
