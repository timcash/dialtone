#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FLAKE_PATH="$SCRIPT_DIR/flake.nix"

# 1. Ensure nix is in PATH
if ! command -v nix &> /dev/null; then
    # Try common Nix locations
    NIX_PATHS=(
        "/nix/var/nix/profiles/default/bin"
        "$HOME/.nix-profile/bin"
        "/nix/store/m21cgvq62rppvhq8yxlylh2gy6akclh4-user-environment/bin"
    )
    for p in "${NIX_PATHS[@]}"; do
        if [ -d "$p" ]; then
            export PATH="$p:$PATH"
            break
        fi
    done
fi

# 2. Ensure flake.nix exists
if [ ! -f "$FLAKE_PATH" ]; then
    cat > "$FLAKE_PATH" <<EOF
{
  description = "Dialtone dev shell";
  inputs = {
    nixpkgs.url = "https://github.com/NixOS/nixpkgs/archive/refs/heads/nixos-24.11.tar.gz";
  };
  outputs = { self, nixpkgs }:
    let
      supportedSystems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
    in
    {
      devShells = forAllSystems (system:
        let pkgs = import nixpkgs { inherit system; }; in
        {
          default = pkgs.mkShell {
            packages = with pkgs; [
              bash curl git gh openssh go_1_24 gnumake gcc cmake ninja pkg-config nodejs tmux zsh cloudflared
            ] ++ (if pkgs.stdenv.isLinux then [
              pkgs.musl.dev pkgs.glibc.static pkgs.pkgsStatic.gcc
            ] else [ ]);
            shellHook = ''
              export DIALTONE_REPO_ROOT=\$(pwd)
              export PATH="\$DIALTONE_REPO_ROOT/bin:\$PATH"
              
              if [ -n "\$ZSH_VERSION" ]; then
                fpath=(\$ZSH/functions \$ZSH/scripts \$fpath)
                autoload -U compinit && compinit -u
                zstyle ':completion:*' menu select
                setopt MENU_COMPLETE
                bindkey '^I' expand-or-complete
              fi

              export DIALTONE_NIX_ACTIVE=1
              echo "DIALTONE> nix-shell active"
            '';
          };
        }
      );
    };
}
EOF
fi

# 3. Command routing
NIX_FLAGS=(--extra-experimental-features "nix-command flakes")
TMUX_CONF="$SCRIPT_DIR/.tmux.conf"

# If first arg is 'ssh', we want to use the Nix ssh with our mesh config
if [ "$1" = "ssh" ]; then
    shift
    SSH_CONFIG="$SCRIPT_DIR/env/ssh_config"
    if [ -f "$SSH_CONFIG" ]; then
        exec nix "${NIX_FLAGS[@]}" develop "$SCRIPT_DIR" --command ssh -F "$SSH_CONFIG" "$@"
    else
        exec nix "${NIX_FLAGS[@]}" develop "$SCRIPT_DIR" --command ssh "$@"
    fi
fi

# Default: Enter Nix shell or run generic command
if [ $# -eq 0 ]; then
    if [ "$DIALTONE_NIX_ACTIVE" = "1" ]; then
        if [ -z "$TMUX" ]; then
            if [ -f "$TMUX_CONF" ]; then
                exec tmux -f "$TMUX_CONF" new-session -A -s dialtone -c "$SCRIPT_DIR"
            else
                exec tmux new-session -A -s dialtone -c "$SCRIPT_DIR"
            fi
        else
            echo "DIALTONE> Already inside Nix and tmux."
            exit 0
        fi
    fi
    echo "DIALTONE> Entering Nix dev shell..."
    if [ -f "$TMUX_CONF" ]; then
        exec nix "${NIX_FLAGS[@]}" develop "$SCRIPT_DIR" -c tmux -f "$TMUX_CONF" new-session -A -s dialtone -c "$SCRIPT_DIR" zsh
    else
        exec nix "${NIX_FLAGS[@]}" develop "$SCRIPT_DIR" -c tmux new-session -A -s dialtone -c "$SCRIPT_DIR" zsh
    fi
else
    exec nix "${NIX_FLAGS[@]}" develop "$SCRIPT_DIR" --command "$@"
fi
