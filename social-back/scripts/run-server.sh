#!/usr/bin/env bash
# Builds and runs the API server detached. Output goes to /tmp/gophersocial-server.log
set -euo pipefail
cd "$(dirname "$0")/.."
go build -o /tmp/gophersocial-server ./cmd/api
setsid nohup /tmp/gophersocial-server > /tmp/gophersocial-server.log 2>&1 < /dev/null &
echo "server pid: $!"
