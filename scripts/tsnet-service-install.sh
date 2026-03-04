#!/usr/bin/env bash
set -euo pipefail

SERVICE_NAME="${SERVICE_NAME:-dialtone_tsnet}"
REPO_ROOT="${1:-$HOME/dialtone}"
RUN_CMD="${RUN_CMD:-./dialtone.sh tsnet src_v1 up}"
LOG_DIR="$HOME/.dialtone/logs"
mkdir -p "$LOG_DIR"

if [[ ! -x "$REPO_ROOT/dialtone.sh" ]]; then
  echo "missing dialtone.sh at: $REPO_ROOT/dialtone.sh" >&2
  exit 1
fi

install_linux_user_service() {
  local unit_dir="$HOME/.config/systemd/user"
  local unit_path="$unit_dir/${SERVICE_NAME}.service"
  mkdir -p "$unit_dir"

  cat >"$unit_path" <<EOF
[Unit]
Description=Dialtone tsnet service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=$REPO_ROOT
ExecStart=/bin/bash -lc 'cd "$REPO_ROOT" && $RUN_CMD'
Restart=always
RestartSec=2
StandardOutput=append:$LOG_DIR/${SERVICE_NAME}.log
StandardError=append:$LOG_DIR/${SERVICE_NAME}.log

[Install]
WantedBy=default.target
EOF

  systemctl --user daemon-reload
  systemctl --user enable --now "${SERVICE_NAME}.service"
  systemctl --user is-active "${SERVICE_NAME}.service"
  echo "installed: $unit_path"
}

install_macos_launch_agent() {
  local label="dev.dialtone.${SERVICE_NAME}"
  local plist="$HOME/Library/LaunchAgents/${label}.plist"
  mkdir -p "$HOME/Library/LaunchAgents"

  cat >"$plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>${label}</string>
  <key>ProgramArguments</key>
  <array>
    <string>/bin/bash</string>
    <string>-lc</string>
    <string>cd "$REPO_ROOT" && $RUN_CMD</string>
  </array>
  <key>WorkingDirectory</key>
  <string>$REPO_ROOT</string>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>$LOG_DIR/${SERVICE_NAME}.log</string>
  <key>StandardErrorPath</key>
  <string>$LOG_DIR/${SERVICE_NAME}.log</string>
</dict>
</plist>
EOF

  launchctl bootout "gui/$(id -u)/${label}" >/dev/null 2>&1 || true
  launchctl bootstrap "gui/$(id -u)" "$plist"
  launchctl enable "gui/$(id -u)/${label}"
  launchctl kickstart -k "gui/$(id -u)/${label}"
  launchctl print "gui/$(id -u)/${label}" >/dev/null
  echo "installed: $plist"
}

case "$(uname -s)" in
  Linux)
    install_linux_user_service
    ;;
  Darwin)
    install_macos_launch_agent
    ;;
  *)
    echo "unsupported OS: $(uname -s)" >&2
    exit 1
    ;;
esac
