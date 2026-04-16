#!/usr/bin/env bash
#
# deploy.sh — Build, upload, and deploy drynn to a DigitalOcean droplet.
#
# Usage:
#   ./deploy/deploy.sh <hostname>
#
# Prerequisites:
#   - SSH access to the target host as a sudoer.
#   - The target host has already been provisioned (see DEPLOYING.md §1–5).
#   - Go toolchain installed locally (for cross-compilation).
#   - atlas CLI installed on the target host (for migrations).
#
# What this script does:
#   1. Cross-compiles the server and db binaries for linux/amd64.
#   2. Uploads binaries, web assets, and migration files to the target.
#   3. Installs the payload into /opt/drynn on the target.
#   4. Applies any pending database migrations.
#   5. Restarts the drynn systemd service.
#
# What this script does NOT do:
#   - Initial host provisioning (users, directories, firewall, nginx, certs).
#   - Config file creation (run drynn-db init-config on first deploy).
#   - JWT key or admin account seeding (one-time setup tasks).

set -euo pipefail

if [[ $# -lt 1 ]]; then
    echo "Usage: $0 <hostname>" >&2
    exit 1
fi

HOST="$1"
BUILD_DIR="$(mktemp -d)"
trap 'rm -rf "$BUILD_DIR"' EXIT

echo "==> Building binaries..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
    go build -trimpath -ldflags='-s -w' -o "$BUILD_DIR/drynn-server" ./cmd/server
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
    go build -trimpath -ldflags='-s -w' -o "$BUILD_DIR/drynn-db" ./cmd/db

echo "==> Uploading binaries..."
scp "$BUILD_DIR/drynn-server" "$BUILD_DIR/drynn-db" "$HOST":/tmp/

echo "==> Uploading web assets..."
rsync -a --delete web/ "$HOST":/tmp/drynn-web/

echo "==> Uploading migrations..."
rsync -a --delete db/migrations/ "$HOST":/tmp/drynn-migrations/

echo "==> Installing on target..."
# shellcheck disable=SC2087
ssh "$HOST" bash <<'REMOTE'
set -euo pipefail

sudo install -o root -g root -m 0755 /tmp/drynn-server /opt/drynn/bin/server
sudo install -o root -g root -m 0755 /tmp/drynn-db     /opt/drynn/bin/drynn-db

sudo rsync -a --delete /tmp/drynn-web/        /opt/drynn/web/
sudo rsync -a --delete /tmp/drynn-migrations/ /opt/drynn/db/migrations/

sudo chown -R root:root /opt/drynn
sudo find /opt/drynn -type d -exec chmod 0755 {} \;
sudo find /opt/drynn -type f -exec chmod 0644 {} \;
sudo chmod 0755 /opt/drynn/bin/server /opt/drynn/bin/drynn-db

rm -f /tmp/drynn-server /tmp/drynn-db
rm -rf /tmp/drynn-web /tmp/drynn-migrations

echo "==> Applying migrations..."
sudo -u drynn atlas migrate apply \
    --dir file:///opt/drynn/db/migrations \
    --url "$(sudo -u drynn grep -oP '"database_url"\s*:\s*"\K[^"]+' /var/lib/drynn/config/server.json)"

echo "==> Restarting service..."
sudo systemctl restart drynn.service
sleep 2
sudo systemctl is-active drynn.service
REMOTE

echo "==> Deploy complete."
