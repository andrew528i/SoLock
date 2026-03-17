# Building

## Prerequisites

- Go 1.24+
- Docker (for Solana program compilation)
- Solana CLI (for deploying the program)

Optional: [Nix](https://nixos.org/) for reproducible dev environment (`nix develop`).

## Build the Go application

```bash
make build
```

This produces `./solock` binary with the Solana program already embedded.

## Build the Solana program

The Solana program (.so) is compiled inside a Docker container targeting linux/amd64:

```bash
make program-build
```

This runs two steps:

1. `program-docker-build` - builds the Docker image with Solana CLI 2.3.13 and platform-tools v1.54
2. Runs the container to compile the Anchor program
3. Places `solock.so` in `app/internal/repository/adapter/program/`

After rebuilding the .so, run `make build` again to embed the new binary.

## Run tests

```bash
make test
```

## Run

```bash
make run
# or directly:
./solock
```

## Development environment

With Nix installed:

```bash
nix develop
```

This provides Go, Rust, Solana CLI, Anchor, Node.js and all build dependencies.

## Makefile targets

| Target | Description |
|--------|-------------|
| `build` | Compile Go app to `./solock` |
| `run` | Build and run |
| `test` | Run all Go tests |
| `generate` | Regenerate mock files |
| `program-docker-build` | Build Docker image for program compilation |
| `program-build` | Build Solana program and place .so for embedding |
| `clean` | Remove binary |

## Embedded program binary

The compiled Solana program is embedded into the Go binary at compile time via `//go:embed`. The .so file contains a placeholder `declare_id` that gets patched at runtime with the user's actual program ID during deploy.

This means end users don't need Docker, Rust, or any build tools - everything is in the single `solock` binary.
