{
  description = "Dialtone dev shell (nix-first bootstrap)";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.11";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            bash
            curl
            git
            gh
            openssh
            go
            gnumake
            gcc
            cmake
            ninja
            pkg-config
            nodejs
          ];
          shellHook = ''
            export DIALTONE_USE_NIX=1
            echo "DIALTONE> nix-shell active"
          '';
        };
      }
    );
}

