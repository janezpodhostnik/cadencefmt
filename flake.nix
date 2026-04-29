{
  description = "cadencefmt — formatter for the Cadence smart contract language";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        version = self.shortRev or "dev";
      in
      {
        packages = rec {
          cadencefmt = pkgs.buildGoModule {
            pname = "cadencefmt";
            inherit version;
            src = self;
            vendorHash = null; # populate after first nix build
            subPackages = [ "cmd/cadencefmt" "cmd/cadencefmt-lsp" ];
            ldflags = [ "-s" "-w" "-X main.version=${version}" ];
            meta = with pkgs.lib; {
              description = "Formatter for the Cadence smart contract language";
              license = licenses.asl20;
              mainProgram = "cadencefmt";
              platforms = platforms.unix;
            };
          };
          default = cadencefmt;
        };

        apps = {
          cadencefmt = flake-utils.lib.mkApp {
            drv = self.packages.${system}.cadencefmt;
            name = "cadencefmt";
          };
          cadencefmt-lsp = flake-utils.lib.mkApp {
            drv = self.packages.${system}.cadencefmt;
            name = "cadencefmt-lsp";
          };
          default = self.apps.${system}.cadencefmt;
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go_1_25
            gopls
            gotools
            golangci-lint
            goreleaser
            git
            nixpkgs-fmt
          ];
        };

        checks.build = self.packages.${system}.cadencefmt;
        formatter = pkgs.nixpkgs-fmt;
      });
}
