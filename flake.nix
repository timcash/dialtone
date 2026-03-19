{
  description = "Dialtone dev shell";
  inputs = {
    nixpkgs.url = "https://github.com/NixOS/nixpkgs/archive/refs/heads/nixos-24.11.tar.gz";
  };
  outputs = { self, nixpkgs }:
    let
      supportedSystems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
      mkPerSystem = system:
        let
          pkgs = import nixpkgs { inherit system; };
          baseDevPackages = with pkgs; [
            bash curl git gh go_1_24 gnumake nodejs bun tmux zsh cloudflared
          ] ++ (pkgs.lib.optionals pkgs.stdenv.isDarwin (with pkgs.darwin.apple_sdk.frameworks; [
            Security
            CoreFoundation
            IOKit
          ]));
          mkShellHook = shellName: ''
            export DIALTONE_REPO_ROOT="''${DIALTONE_REPO_ROOT:-$(pwd)}"
            export DIALTONE_NIX_ACTIVE=1
            export DIALTONE_NIX_SHELL="${shellName}"
            export DIALTONE_SSH_CONFIG="$DIALTONE_REPO_ROOT/env/ssh_config"
            export DIALTONE_NIX_BASE_PATH="$PATH"
            export DIALTONE_GO_BIN="$(PATH="$DIALTONE_NIX_BASE_PATH" command -v go)"
            if PATH="$DIALTONE_NIX_BASE_PATH" command -v ssh >/dev/null 2>&1; then
              export DIALTONE_SSH_BIN="$(PATH="$DIALTONE_NIX_BASE_PATH" command -v ssh)"
            else
              unset DIALTONE_SSH_BIN || true
            fi
            export PATH="$DIALTONE_REPO_ROOT/bin:$PATH"

            if [ -n "$ZSH_VERSION" ]; then
              fpath=($ZSH/functions $ZSH/scripts $fpath)
              autoload -U compinit && compinit -u
              zstyle ':completion:*' menu select
              setopt MENU_COMPLETE
              bindkey '^I' expand-or-complete
            fi

            echo "DIALTONE> nix-shell active (${shellName})"
          '';
          mkDevShell = { shellName, extraPackages ? [ ] }:
            pkgs.mkShell {
              packages = baseDevPackages ++ extraPackages;
              shellHook = mkShellHook shellName;
            };
          runtimeScript = { name, text, inputs ? [ ] }:
            pkgs.writeShellApplication {
              inherit name text;
              runtimeInputs = with pkgs; [ bash git go_1_24 ] ++ inputs;
            };
          dialtoneMod = runtimeScript {
            name = "dialtone-mod";
            text = ''
              set -euo pipefail
              repo_root="''${DIALTONE_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
              cd "$repo_root"
              exec ./dialtone_mod "$@"
            '';
          };
          robotServer = runtimeScript {
            name = "dialtone-robot-server";
            text = ''
              set -euo pipefail
              repo_root="''${DIALTONE_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
              cd "$repo_root/src"
              exec go run ./plugins/robot/src_v2/cmd/server/main.go "$@"
            '';
          };
          cameraService = runtimeScript {
            name = "dialtone-camera-service";
            text = ''
              set -euo pipefail
              repo_root="''${DIALTONE_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
              cd "$repo_root/src"
              exec go run ./plugins/camera/src_v1/cmd/main.go "$@"
            '';
          };
          mavlinkService = runtimeScript {
            name = "dialtone-mavlink-service";
            text = ''
              set -euo pipefail
              repo_root="''${DIALTONE_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
              cd "$repo_root/src"
              exec go run ./plugins/mavlink/src_v1/cmd/main.go "$@"
            '';
          };
          replService = runtimeScript {
            name = "dialtone-repl-service";
            text = ''
              set -euo pipefail
              repo_root="''${DIALTONE_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
              cd "$repo_root/src"
              exec go run ./plugins/repl/src_v1/cmd/repld/main.go "$@"
            '';
          };
          replModV1 = runtimeScript {
            name = "dialtone-repl-v1";
            text = ''
              set -euo pipefail
              repo_root="''${DIALTONE_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
              cd "$repo_root"
              exec go run ./src/mods.go repl v1 "$@"
            '';
          };
          sshModV1 = runtimeScript {
            name = "dialtone-ssh-v1";
            text = ''
              set -euo pipefail
              repo_root="''${DIALTONE_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
              cd "$repo_root"
              exec go run ./src/mods.go ssh v1 "$@"
            '';
          };
        in
        {
          packages = {
            dialtone-mod = dialtoneMod;
            robot-server = robotServer;
            camera-service = cameraService;
            mavlink-service = mavlinkService;
            repl-service = replService;
            repl-v1 = replModV1;
            ssh-v1 = sshModV1;
          };
          apps = {
            dialtone-mod = {
              type = "app";
              program = "${dialtoneMod}/bin/dialtone-mod";
            };
            robot-server = {
              type = "app";
              program = "${robotServer}/bin/dialtone-robot-server";
            };
            camera-service = {
              type = "app";
              program = "${cameraService}/bin/dialtone-camera-service";
            };
            mavlink-service = {
              type = "app";
              program = "${mavlinkService}/bin/dialtone-mavlink-service";
            };
            repl-service = {
              type = "app";
              program = "${replService}/bin/dialtone-repl-service";
            };
            repl-v1 = {
              type = "app";
              program = "${replModV1}/bin/dialtone-repl-v1";
            };
            ssh-v1 = {
              type = "app";
              program = "${sshModV1}/bin/dialtone-ssh-v1";
            };
          };
          devShells = {
            default = mkDevShell {
              shellName = "default";
              extraPackages = [ pkgs.openssh ];
            };
            repl-v1 = mkDevShell {
              shellName = "repl-v1";
            };
            ssh-v1 = mkDevShell {
              shellName = "ssh-v1";
              extraPackages = [ pkgs.openssh ];
            };
          };
        };
    in
    {
      packages = forAllSystems (system: (mkPerSystem system).packages);
      apps = forAllSystems (system: (mkPerSystem system).apps);
      devShells = forAllSystems (system: (mkPerSystem system).devShells);
    };
}
