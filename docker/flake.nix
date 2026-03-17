{
  description = "SoLock Solana program build environment (Linux)";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachSystem [ "x86_64-linux" ] (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            rustc
            cargo
            rustfmt
            pkg-config
            openssl
            libudev-zero
            gcc
            binutils
            curl
            wget
            bzip2
            jq
          ];

          RUST_BACKTRACE = "1";
        };
      }
    );
}
