{
  description = "anitui - a terminal anime watch-list";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "anitui";
          version = "0.1.0";
          src = ./.;
          proxyVendor = true;
          vendorHash = "sha256-Hr0iscn8diUB/+/80iDkoYGTMO0kzBZ/oa8jZo7w+DQ=";
        };

        devShells.default = pkgs.mkShell {
          buildInputs = [ pkgs.go ];
        };
      });
}
