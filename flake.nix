{
  description = "SoLock - Decentralized password manager on Solana";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        solanaVersion = "2.3.13";
        platformToolsVersion = "v1.54";

        platformArch = if system == "x86_64-linux" then "linux-x86_64"
                       else if system == "aarch64-linux" then "linux-aarch64"
                       else if system == "x86_64-darwin" then "macos-x86_64"
                       else if system == "aarch64-darwin" then "macos-aarch64"
                       else throw "Unsupported system: ${system}";
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "solock";
          version = "0.1.0";
          src = ./app;
          vendorHash = pkgs.lib.fakeHash;
          CGO_ENABLED = 1;
          buildInputs = with pkgs; [ sqlite ];
          nativeBuildInputs = with pkgs; [ pkg-config ];
          subPackages = [ "cmd/solock" ];
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            golangci-lint

            rustc
            cargo
            rustfmt
            clippy

            nodejs_20
            nodePackages.yarn
            nodePackages.typescript

            pkg-config
            openssl
            sqlite

            jq
            curl
            git
            wget
            bzip2
          ] ++ pkgs.lib.optionals pkgs.stdenv.isLinux [
            libudev-zero
            gcc
            binutils
          ];

          shellHook = ''
            echo "SoLock Development Environment"
            echo ""

            export PROJECT_ROOT="$(pwd)"
            export SOLANA_CACHE="$PROJECT_ROOT/.solock-dev"
            mkdir -p "$SOLANA_CACHE"

            export GOPATH="$PROJECT_ROOT/.solock-dev/go"
            mkdir -p "$GOPATH/bin"

            export CARGO_HOME="$SOLANA_CACHE/cargo"
            mkdir -p "$CARGO_HOME"

            export XDG_CACHE_HOME="$SOLANA_CACHE/cache"
            mkdir -p "$XDG_CACHE_HOME"

            export SOLANA_INSTALL_DIR="$SOLANA_CACHE/install"
            mkdir -p "$SOLANA_INSTALL_DIR"

            TOOLS_VERSION="${platformToolsVersion}"
            PLATFORM_TOOLS="$SOLANA_CACHE/cache/solana/$TOOLS_VERSION/platform-tools"

            export PATH="$PLATFORM_TOOLS/llvm/bin:$PLATFORM_TOOLS/rust/bin:$SOLANA_CACHE/install/active_release/bin:$CARGO_HOME/bin:$GOPATH/bin:$PATH"

            if [ -d "$PLATFORM_TOOLS/rust/bin" ]; then
              export RUSTC="$PLATFORM_TOOLS/rust/bin/rustc"
            fi

            if [ ! -f "$CARGO_HOME/bin/anchor" ]; then
              echo "Installing Anchor CLI (project-local)..."
              cargo install --git https://github.com/coral-xyz/anchor anchor-cli --locked --root "$CARGO_HOME" 2>/dev/null || true
            fi

            if [ ! -f "$SOLANA_CACHE/install/active_release/bin/solana" ]; then
              echo "Installing Solana CLI tools v${solanaVersion} (project-local)..."
              ARCH="${platformArch}"
              if echo "$ARCH" | grep -q "linux"; then
                SUFFIX="unknown-linux-gnu"
              else
                SUFFIX="apple-darwin"
              fi
              if echo "$ARCH" | grep -q "x86_64"; then
                FULL_ARCH="x86_64-$SUFFIX"
              else
                FULL_ARCH="aarch64-$SUFFIX"
              fi
              curl -sSfL "https://release.anza.xyz/v${solanaVersion}/solana-release-$FULL_ARCH.tar.bz2" -o /tmp/solana-release.tar.bz2 && \
              tar xjf /tmp/solana-release.tar.bz2 -C /tmp && \
              mkdir -p "$SOLANA_CACHE/install/active_release" && \
              cp -r /tmp/solana-release/* "$SOLANA_CACHE/install/active_release/" && \
              rm -rf /tmp/solana-release /tmp/solana-release.tar.bz2 && \
              echo "Solana CLI tools installed." || echo "Warning: Failed to install Solana CLI tools"
            fi

            echo ""
            echo "Tools:"
            echo "  go:      $(go version 2>/dev/null | cut -d' ' -f3)"
            echo "  solana:  $(solana --version 2>/dev/null || echo 'not installed')"
            echo "  anchor:  $(anchor --version 2>/dev/null || echo 'not installed')"
            echo ""
            echo "Commands:"
            echo "  make build        - Build SoLock TUI"
            echo "  make test         - Run tests"
            echo "  make anchor-build - Build Solana program"
            echo "  make run          - Run SoLock"
            echo ""
          '';

          RUST_BACKTRACE = "1";
          CGO_ENABLED = "1";
        };
      }
    );
}
