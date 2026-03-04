#!/usr/bin/env bash
set -euo pipefail

BLOCK_START="# >>> dialtone nix path >>>"
BLOCK_END="# <<< dialtone nix path <<<"
BLOCK_BODY='export PATH="/nix/var/nix/profiles/default/bin:$HOME/.nix-profile/bin:$PATH"'

upsert_block() {
  local file="$1"
  [ -f "$file" ] || : > "$file"
  if grep -Fq "$BLOCK_START" "$file"; then
    return 0
  fi
  {
    echo ""
    echo "$BLOCK_START"
    echo "$BLOCK_BODY"
    echo "$BLOCK_END"
  } >> "$file"
}

upsert_block "$HOME/.profile"
upsert_block "$HOME/.bashrc"
upsert_block "$HOME/.zshrc"

echo "nix PATH block ensured in: ~/.profile ~/.bashrc ~/.zshrc"
