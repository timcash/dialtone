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
          runtimeScript = { name, text, inputs ? [ ] }:
            pkgs.writeShellApplication {
              inherit name text;
              runtimeInputs = with pkgs; [ bash git go_1_24 ] ++ inputs;
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
              exec go run ./src/cli.go repl v1 "$@"
            '';
          };
        in
        {
          packages = {
            robot-server = robotServer;
            camera-service = cameraService;
            mavlink-service = mavlinkService;
            repl-service = replService;
            repl-v1 = replModV1;
          };
          apps = {
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
          };
          devShells = {
            default = pkgs.mkShell {
              packages = with pkgs; [
                bash curl git gh openssh go_1_24 gnumake nodejs bun tmux zsh cloudflared
              ] ++ (pkgs.lib.optionals pkgs.stdenv.isDarwin (with pkgs.darwin.apple_sdk.frameworks; [
                Security
                CoreFoundation
                IOKit
              ]));
              shellHook = ''
                export DIALTONE_REPO_ROOT=$(pwd)
                export PATH="$DIALTONE_REPO_ROOT/bin:$PATH"
                export DIALTONE_SSH_CONFIG="$DIALTONE_REPO_ROOT/env/ssh_config"

                # Initialize completions and set up cycling if in ZSH
                if [ -n "$ZSH_VERSION" ]; then
                  fpath=($ZSH/functions $ZSH/scripts $fpath)
                  autoload -U compinit && compinit -u
                  zstyle ':completion:*' menu select
                  setopt MENU_COMPLETE
                  bindkey '^I' expand-or-complete
                fi

                export DIALTONE_NIX_ACTIVE=1
                echo "DIALTONE> nix-shell active"
              '';
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
