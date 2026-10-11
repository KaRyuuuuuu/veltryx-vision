#!/usr/bin/env bash
# Forward Basler RTSP through the timing server, without changing its services.
set -euo pipefail

SSH_TARGET="${1:-${VISION_SSH_TARGET:-karyuu@192.168.103.49}}"
CAMERA_IP="${VISION_CAMERA_IP:-192.168.101.90}"
LOCAL_PORT="${VISION_RTSP_PORT:-8554}"
REMOTE_PORT="${VISION_CAMERA_RTSP_PORT:-554}"

if ! [[ "$LOCAL_PORT" =~ ^[0-9]+$ && "$REMOTE_PORT" =~ ^[0-9]+$ ]] ||
   (( LOCAL_PORT < 1 || LOCAL_PORT > 65535 || REMOTE_PORT < 1 || REMOTE_PORT > 65535 )); then
  echo "Ports invalides" >&2
  exit 2
fi

if ! command -v ssh >/dev/null; then
  echo "OpenSSH manquant (sudo pacman -S openssh)" >&2
  exit 2
fi

if command -v ss >/dev/null && ss -H -ltn "( sport = :$LOCAL_PORT )" | grep -q .; then
  echo "Le port local $LOCAL_PORT est déjà utilisé. Tunnel existant ?" >&2
  echo "Vérifie avec : ss -ltnp '( sport = :$LOCAL_PORT )'" >&2
  exit 1
fi

echo "Tunnel RTSP : 127.0.0.1:$LOCAL_PORT -> $CAMERA_IP:$REMOTE_PORT via $SSH_TARGET"
echo "Flux local : rtsp://127.0.0.1:$LOCAL_PORT/h264"
echo "Ctrl+C pour arrêter. Aucun changement sur le serveur distant."
exec ssh -N -T \
  -o ExitOnForwardFailure=yes \
  -o ServerAliveInterval=15 \
  -o ServerAliveCountMax=3 \
  -L "127.0.0.1:$LOCAL_PORT:$CAMERA_IP:$REMOTE_PORT" \
  "$SSH_TARGET"
