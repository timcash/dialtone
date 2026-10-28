#!/bin/sh
# Install dialtone and join a channel with an invite (macOS arm64, Linux x86_64/arm64).
#
#   curl -fsSL https://raw.githubusercontent.com/timcash/dialtone/main/install.sh | sh -s -- '<invite>' [node-name]
#
# Downloads the latest release binary to ~/.dialtone/bin, checks its SHA-256, joins with the
# invite and installs the node so it starts at login. No build tools needed.
# With DIALTONE_STATE_DIR set (a second node on one machine, or a trial), everything goes in
# that folder and the node is started but not installed at login; set DIALTONE_LISTEN too if
# another node already uses 127.0.0.1:8788.
set -eu

INVITE="${1:-${DIALTONE_INVITE:-}}"
NODE="${2:-${DIALTONE_NODE:-}}"
LISTEN="${DIALTONE_LISTEN:-127.0.0.1:8788}"
REPO="https://github.com/timcash/dialtone/releases/latest/download"

case "$INVITE" in
  dialtone-invite-v2:*) ;;
  *) echo "usage: install.sh '<dialtone-invite-v2:...>' [node-name]" >&2; exit 2 ;;
esac

case "$(uname -s)-$(uname -m)" in
  Darwin-arm64) ASSET=dialtone-macos-arm64 ;;
  Linux-x86_64 | Linux-amd64) ASSET=dialtone-linux-x86_64 ;;
  Linux-aarch64 | Linux-arm64) ASSET=dialtone-linux-arm64 ;;
  *) echo "no dialtone release for $(uname -s) $(uname -m)" >&2; exit 1 ;;
esac

STATE="${DIALTONE_STATE_DIR:-$HOME/.dialtone}"
BIN_DIR="$STATE/bin"
BIN="$BIN_DIR/dialtone"
mkdir -p "$BIN_DIR"
echo "Downloading $ASSET ..."
curl -fsSL "$REPO/$ASSET" -o "$BIN.new"
curl -fsSL "$REPO/$ASSET.sha256" -o "$BIN.sha256"
WANT=$(cut -d' ' -f1 < "$BIN.sha256")
if command -v sha256sum >/dev/null 2>&1; then GOT=$(sha256sum "$BIN.new" | cut -d' ' -f1)
else GOT=$(shasum -a 256 "$BIN.new" | cut -d' ' -f1); fi
[ "$WANT" = "$GOT" ] || { echo "checksum mismatch for $ASSET" >&2; rm -f "$BIN.new"; exit 1; }
chmod 755 "$BIN.new"
mv -f "$BIN.new" "$BIN"
rm -f "$BIN.sha256"

[ -n "$NODE" ] || NODE=$(hostname -s 2>/dev/null || hostname)
NODE=$(printf '%s' "$NODE" | tr 'A-Z' 'a-z' | tr -c 'a-z0-9._-' '-' | cut -c1-64)

echo "Joining as '$NODE' ..."
"$BIN" join "$INVITE" --node "$NODE" --listen "$LISTEN" --once
if [ -n "${DIALTONE_STATE_DIR:-}" ]; then
  nohup "$BIN" up --node "$NODE" --listen "$LISTEN" > "$STATE/daemon.log" 2>&1 &
  echo "Done. '$NODE' is joining the mesh (started now, not installed at login)."
else
  "$BIN" daemon install --node "$NODE" --listen "$LISTEN" > /dev/null
  if [ "$(uname -s)" = Linux ]; then
    systemctl --user restart dialtone.service 2>/dev/null \
      || { nohup "$BIN" up --node "$NODE" --listen "$LISTEN" > "$STATE/daemon.log" 2>&1 & }
  fi
  echo "Done. '$NODE' is joining the mesh; it starts again at login."
fi
echo "  HUD:    http://$LISTEN"
echo "  Status: $BIN status"
