# SoLock Go Application

Terminal-based password manager client that stores encrypted entries on Solana blockchain.

## Structure

```
app/
├── cmd/solock/main.go              # Entrypoint
└── internal/
    ├── domain/                      # Core entities and interfaces
    │   ├── service/                 # Key derivation (SLIP-0010), AES-256-CBC
    │   ├── entry.go                 # Entry entity with JSON serialization
    │   ├── entry_type.go            # Password, Note, Card, TOTP
    │   ├── entry_schema.go          # Field definitions per type
    │   ├── repositories.go          # EntryRepo, ConfigRepo, VaultRepo interfaces
    │   └── services.go              # KeyDeriver, CryptoService interfaces
    ├── usecase/                     # Business logic
    │   ├── unlock.go                # Derive keys from master password
    │   ├── add_entry.go             # Create entry with slot allocation
    │   ├── update_entry.go          # Update with optimistic locking
    │   ├── delete_entry.go          # Delete on-chain + local
    │   └── sync.go                  # Bidirectional sync (batch 100)
    ├── repository/
    │   ├── adapter/                 # Solana RPC client, embedded .so
    │   │   ├── solana.go            # PDA derivation, TX building, deploy
    │   │   ├── embed_provider.go    # Embedded program binary + patching
    │   │   └── program/solock.so    # Compiled Solana program (embedded)
    │   └── storage/                 # SQLite with AES encryption
    │       ├── sqlite.go            # DB init, encrypt/decrypt helpers
    │       ├── entry_repo.go        # Entry CRUD
    │       ├── config_repo.go       # Key-value config
    │       └── sync_repo.go         # Sync state persistence
    ├── application/app.go           # Dependency wiring, lifecycle
    ├── api/tui/                     # Bubbletea terminal UI
    │   ├── app.go                   # Screen state machine, event loop
    │   ├── views.go                 # Screen rendering
    │   ├── styles.go                # Colors, input boxes, help bar
    │   ├── passgen.go               # Password generator
    │   ├── totp.go                  # TOTP code generation
    │   └── vault_helpers.go         # Search, sort, entry stats
    └── mock/                        # Generated mocks (gomock)
```

## Key concepts

### Embedded program binary

The Solana program .so file is embedded via `//go:embed` at compile time. At deploy, the placeholder bytes (`SOLOCK_PLACEHOLDER_PROGRAM_V1___`) are replaced with the user's actual program ID. No external files needed.

### Local-first with sync

All operations save to local SQLite first. Entries are encrypted with AES-256-CBC before storage. Sync pushes/pulls to Solana in batches of 100 accounts.

### Optimistic locking

Entry updates pass `expected_updated_at` to the Solana program. If another client modified the entry, the TX fails and sync is required.

## Dependencies

Core:
- [bubbletea](https://github.com/charmbracelet/bubbletea) - terminal UI framework
- [lipgloss](https://github.com/charmbracelet/lipgloss) - terminal styling
- [solana-go](https://github.com/gagliardetto/solana-go) - Solana RPC client
- [go-bip39](https://github.com/tyler-smith/go-bip39) - BIP39 mnemonic generation
- [go-sqlite3](https://github.com/mattn/go-sqlite3) - SQLite driver (CGO)
- [otp](https://github.com/pquerna/otp) - TOTP generation
- [clipboard](https://github.com/atotto/clipboard) - system clipboard access
