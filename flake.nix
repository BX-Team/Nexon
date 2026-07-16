{
  description = "Nexon — Xray control-plane (subscriptions, nodes, traffic)";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        nexon = pkgs.buildGoModule {
          pname = "nexon";
          version = "1.2.0";
          src = ./.;
          vendorHash = "sha256-t9oYPmkJJyQWXHdW1RV+CTrHoIN3r/nTr5jjq9vHmZ0=";
          subPackages = [ "cmd/nexon" ];
          ldflags = [ "-s" "-w" "-X" "github.com/BX-Team/Nexon/internal/cli.Version=1.2.0" ];
        };
      in
      {
        packages.default = nexon;
        packages.nexon = nexon;

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [ go gopls sqlite ];
        };
      })
    // {
      nixosModules.nexon = import ./nix/nixosModules/nexon.nix self;
      nixosModules.default = self.nixosModules.nexon;
    };
}
