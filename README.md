# SoLock

Decentralized password manager that stores encrypted secrets on the Solana blockchain. Single master password, no accounts, no servers - your vault lives on-chain and is accessible from any machine.

## Screenshots

<p align="center">
  <img src="docs/screen-unlock.png" width="700" alt="Unlock screen" />
</p>

<p align="center">
  <img src="docs/screen-dashboard.png" width="700" alt="Dashboard" />
</p>

<p align="center">
  <img src="docs/screen-vault.png" width="700" alt="Vault list" />
</p>

<p align="center">
  <img src="docs/screen-entry-note.png" width="700" alt="Note entry" />
</p>

## How it works

```mermaid
flowchart LR
    PWD[Master password] --> KEYS[Deterministic<br/>key derivation]
    KEYS --> DEPLOY[Deployer keypair]
    KEYS --> PROG[Program keypair]
    KEYS --> ENC[Encryption key]
    DEPLOY --> SOL[Solana blockchain]
    PROG --> SOL
    ENC --> LOCAL[Local SQLite cache]
    ENC --> SOL
    SOL <-->|sync| LOCAL
```

One password derives everything: your Solana keypair, program ID, and encryption key. Entries are encrypted locally with AES-256-CBC, stored on-chain as opaque blobs, and cached in SQLite for offline access.

## Features

- **Single binary** - Solana program embedded, no external dependencies at runtime
- **Deterministic keys** - same password = same vault on any machine
- **End-to-end encrypted** - data is encrypted before leaving your machine
- **Local-first** - works offline, syncs when connected
- **Terminal UI** - keyboard-driven interface built with [Bubbletea](https://github.com/charmbracelet/bubbletea)
- **TOTP support** - 2FA code generation with live countdown timer
- **Password generator** - configurable length, uppercase, digits, special characters
- **Clipboard integration** - copy passwords and TOTP codes with one key
- **Optimistic locking** - safe multi-client access with conflict detection
- **Slot reuse** - deleted entries free up on-chain space

## Supported secret types

| Type | Fields | Description |
|------|--------|-------------|
| **Password** | site, username, password, TOTP secret, notes | Website and app credentials with optional 2FA |
| **Note** | content | Free-form encrypted text for any sensitive information |
| **Card** | cardholder, number, expiry, CVV, notes | Payment card details |

Password entries with a TOTP secret automatically generate live 2FA codes with a countdown timer.

## Quick start

### Prerequisites

- Go 1.24+
- Solana CLI (for deploying)

### Build and run

```bash
make build
./solock
```

### First-time setup

1. Enter a master password
2. Copy the deployer address (`p` key) and fund it with ~3 SOL
3. Deploy the program (`d` key)
4. Initialize the vault (`i` key)
5. Add your first entry (`a` key)

For devnet testing, get free SOL from [faucet.solana.com](https://faucet.solana.com).

## Building the Solana program

The compiled program is already embedded in the Go binary. To recompile:

```bash
make program-build   # requires Docker
make build           # rebuild Go binary with new .so
```

See [docs/building.md](docs/building.md) for details.

## Documentation

- [Architecture](docs/architecture.md) - system design, data flow, on-chain storage model
- [Key Derivation](docs/key-derivation.md) - how master password becomes Solana keys
- [Building](docs/building.md) - build instructions, Makefile targets
- [Usage](docs/usage.md) - screens, keybindings, entry types
- [Solana Program](program/README.md) - on-chain instructions, accounts, conflict detection
- [Go Application](app/README.md) - code structure, dependencies

## Tech stack

| Component | Technology |
|-----------|-----------|
| TUI | Go + [Bubbletea](https://github.com/charmbracelet/bubbletea) / [Lipgloss](https://github.com/charmbracelet/lipgloss) |
| Blockchain | Solana + [Anchor](https://www.anchor-lang.com/) (Rust) |
| Encryption | AES-256-CBC |
| Key derivation | SHA256 + BIP39 + SLIP-0010 |
| Local storage | SQLite (encrypted) |
| TOTP | RFC 6238 |
| Dev environment | Nix |

## Security model

- **No registration** - no email, no phone, no cloud accounts
- **No servers** - SoLock talks directly to Solana RPC
- **No key files** - keys are derived from password on every launch, never stored
- **Memory safety** - keys zeroed on exit
- **Encrypted at rest** - local SQLite values encrypted with AES-256-CBC
- **Encrypted on-chain** - Solana stores opaque encrypted blobs, not plaintext
- **Your program** - each user deploys their own Solana program instance

## Roadmap

- [x] Solana program (Anchor/Rust) with vault and entry management
- [x] Deterministic key derivation (SHA256 + BIP39 + SLIP-0010)
- [x] AES-256-CBC encryption for local and on-chain data
- [x] Terminal UI with Bubbletea
- [x] Password, note and card entry types
- [x] TOTP 2FA code generation
- [x] Bidirectional sync with Solana
- [x] Embedded program binary with runtime patching
- [x] Built-in password generator
- [ ] Browser extension (Chrome / Firefox) with autofill
- [ ] Desktop app for macOS
- [ ] Desktop app for Linux
- [ ] SSH key storage
- [ ] Secure file attachments
- [ ] Shared vaults (multi-user access via on-chain permissions)

## License

MIT
