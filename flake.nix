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
              bash curl git gh openssh go_1_24 gnumake nodejs tmux zsh cloudflared codex
            ];
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
        }
      );
    };
}
