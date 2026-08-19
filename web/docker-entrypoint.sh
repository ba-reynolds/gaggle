#!/bin/sh
# Wraps nginx startup: guarantees the 443 listener has certificate files
# before nginx boots. If no real cert is mounted yet (fresh box / empty cert
# volume), a self-signed fallback is generated so the TLS server block can
# start. certbot certificates replace these paths once HTTPS_DOMAIN is
# provisioned (see deploy/README.md).
set -e

CERT_DIR=/etc/letsencrypt/live/gaggle
FULLCHAIN="$CERT_DIR/fullchain.pem"
PRIVKEY="$CERT_DIR/privkey.pem"

if [ ! -s "$FULLCHAIN" ] || [ ! -s "$PRIVKEY" ]; then
  echo ">> no TLS cert at $CERT_DIR, generating self-signed fallback"
  mkdir -p "$CERT_DIR"
  openssl req -x509 -nodes -newkey rsa:2048 -days 3650 \
    -keyout "$PRIVKEY" -out "$FULLCHAIN" \
    -subj "/CN=gaggle-selfsigned" >/dev/null 2>&1
fi

exec nginx -g 'daemon off;'