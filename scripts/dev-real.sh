#!/usr/bin/env bash
# Start the real GigE Vision backend and dashboard; never enables simulation.
set -Eeuo pipefail
stage='vérification des prérequis'
report_error() {
  local status="$1"
  echo "Veltryx Vision : échec pendant $stage (code $status, ligne $2)." >&2
}
trap 'report_error "$?" "$LINENO"' ERR
echo 'Veltryx Vision — préparation du démarrage matériel…' 
cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."

for dependency in go node npm pkg-config; do
  if ! command -v "$dependency" >/dev/null 2>&1; then
    echo "Missing dependency: $dependency. See README.md." >&2
    exit 1
  fi
done
if ! pkg-config --print-errors --exists aravis-0.10 x264; then
  echo 'Dépendances natives manquantes : pkg-config doit trouver aravis-0.10 et x264.' >&2
  if pkg-config --exists aravis-0.8; then
    echo 'Aravis 0.8 est détecté ; cet adaptateur cible les API du paquet pkg-config aravis-0.10.' >&2
  fi
  echo 'Installer les bibliothèques de développement correspondantes, ou définir PKG_CONFIG_PATH si elles sont installées dans un préfixe personnalisé. Voir README.md.' >&2
  exit 1
fi
if [[ ! -f apps/web/node_modules/next/dist/bin/next ]]; then
  stage="installation des dépendances du site"
  echo "Installation des dépendances du site…"
  npm ci --prefix apps/web
fi
stage="compilation du backend Aravis/x264"
echo "Compilation du backend Aravis/x264…"
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

stage="exécution des serveurs"
./bin/veltryx-vision -listen 127.0.0.1:8080 &
backend_pid=$!
BACKEND_URL=http://127.0.0.1:8080 node apps/web/node_modules/next/dist/bin/next dev apps/web --hostname 0.0.0.0 --port 3000 &
web_pid=$!
echo 'Veltryx Vision — site sur toutes les interfaces, port 3000 (Ctrl+C arrête les deux serveurs)'
node -e 'const {networkInterfaces}=require("node:os"); for(const addresses of Object.values(networkInterfaces())) for(const a of addresses || []) if(a.family==="IPv4" && !a.internal) console.log(`Réseau local : http://${a.address}:3000`);'
if wait -n "$backend_pid" "$web_pid"; then
  echo 'Un serveur s’est arrêté ; arrêt de l’autre serveur.' >&2
else
  status=$?
  echo "Un serveur s’est arrêté avec une erreur (code $status). Consulter les logs ci-dessus ; vérifier notamment que les ports 3000 et 8080 sont libres." >&2
  exit "$status"
fi
