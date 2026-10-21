#!/usr/bin/env bash
# Start the real GigE Vision backend and dashboard; never enables simulation.
set -euo pipefail
cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."

for dependency in go node npm pkg-config; do
  if ! command -v "$dependency" >/dev/null 2>&1; then
    echo "Missing dependency: $dependency. See README.md." >&2
    exit 1
  fi
done
pkg-config --exists aravis-0.10 x264
if [[ ! -f apps/web/node_modules/next/dist/bin/next ]]; then
  npm ci --prefix apps/web
fi
mkdir -p bin
CGO_ENABLED=1 go build -tags 'aravis x264' -o bin/veltryx-vision ./apps/backend

backend_pid=''
web_pid=''
cleanup() {
  trap - EXIT INT TERM
  if [[ -n "$web_pid" ]]; then kill -TERM "$web_pid" 2>/dev/null || true; fi
  if [[ -n "$backend_pid" ]]; then kill -TERM "$backend_pid" 2>/dev/null || true; fi
  if [[ -n "$web_pid" ]]; then wait "$web_pid" 2>/dev/null || true; fi
  if [[ -n "$backend_pid" ]]; then wait "$backend_pid" 2>/dev/null || true; fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

./bin/veltryx-vision -listen 127.0.0.1:8080 &
backend_pid=$!
BACKEND_URL=http://127.0.0.1:8080 node apps/web/node_modules/next/dist/bin/next dev apps/web --hostname 0.0.0.0 --port 3000 &
web_pid=$!
echo 'Veltryx Vision — site sur toutes les interfaces, port 3000 (Ctrl+C arrête les deux serveurs)'
node -e 'const {networkInterfaces}=require("node:os"); for(const addresses of Object.values(networkInterfaces())) for(const a of addresses || []) if(a.family==="IPv4" && !a.internal) console.log(`Réseau local : http://${a.address}:3000`);'
wait -n "$backend_pid" "$web_pid"
