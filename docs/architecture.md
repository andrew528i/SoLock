# Architecture

SoLock follows a clean architecture pattern with clear separation between domain logic, use cases, infrastructure and UI.

## High-level overview

```mermaid
graph TD
    TUI[TUI - Bubbletea]
    APP[Application Layer]
    UC[Use Cases]
    DOM[Domain]
    SVC[Domain Services]
    SQL[SQLite Storage]
    SOL[Solana RPC Adapter]

    TUI --> APP
    APP --> UC
    UC --> DOM
    UC --> SVC
    UC --> SQL
    UC --> SOL
```

## Directory structure

```
solock/
├── program/                    # Solana program (Rust/Anchor)
│   ├── Anchor.toml
│   ├── Cargo.toml
│   └── programs/solock/src/
│       └── lib.rs              # Vault + entry CRUD instructions
├── app/                        # Go application
│   ├── cmd/solock/main.go      # CLI entrypoint
│   └── internal/
│       ├── domain/             # Core entities and interfaces
│       │   ├── service/        # Key derivation, AES encryption
│       │   ├── entry.go        # Entry entity
│       │   ├── repositories.go # Repository interfaces
│       │   └── services.go     # Service interfaces
│       ├── usecase/            # Business logic
│       │   ├── unlock.go       # Master password -> keys
│       │   ├── add_entry.go    # Create entries on-chain
│       │   ├── update_entry.go # Update with optimistic locking
│       │   ├── delete_entry.go # Delete on-chain + local
│       │   └── sync.go         # Bidirectional sync
│       ├── repository/
│       │   ├── adapter/        # Solana RPC + embedded binary
│       │   └── storage/        # SQLite with encryption
│       ├── application/        # Dependency wiring
│       └── api/tui/            # Bubbletea terminal UI
├── docker/                     # Program build environment
│   ├── Dockerfile
│   ├── build.sh
│   └── flake.nix
├── flake.nix                   # Dev environment
└── Makefile
```

## Data flow

### Unlock

```mermaid
flowchart LR
    P[Master password] --> S0["SHA256(pwd + ':0')"]
    P --> S1["SHA256(pwd + ':1')"]
    S0 --> M0[BIP39 mnemonic]
    S1 --> M1[BIP39 mnemonic]
    M0 --> SLIP0[SLIP-0010]
    M1 --> SLIP1[SLIP-0010]
    SLIP0 --> DK[Deployer keypair]
    SLIP1 --> PK[Program keypair]
    S0 -->|raw bytes| AES[AES-256-CBC key]
```

### Add entry

```mermaid
sequenceDiagram
    participant U as User
    participant UC as AddEntry
    participant V as Solana
    participant S as SQLite

    U->>UC: Fill form
    UC->>UC: JSON marshal + AES encrypt
    UC->>V: AddEntry TX (encrypted bytes)
    V-->>UC: Confirmation
    UC->>V: GetEntry (read back timestamps)
    V-->>UC: On-chain timestamps
    UC->>S: Save entry (with on-chain timestamps)
```

### Sync

```mermaid
sequenceDiagram
    participant S as Sync UC
    participant V as Solana
    participant DB as SQLite

    S->>V: GetMeta (next_index, entry_count)
    loop Batch of 100
        S->>V: GetMultipleAccounts(indices)
        V-->>S: EntryAccounts[]
        S->>S: Decrypt + unmarshal each
        S->>DB: Upsert entries
    end
    S->>DB: Delete entries missing on-chain
    S->>DB: Save sync state
```

## On-chain storage model

Each user gets their own vault derived from their deployer public key:

```mermaid
graph TD
    PROG[Solana Program] --> VAULT["VaultMeta PDA<br/>seeds: vault_v2, owner"]
    PROG --> E0["Entry PDA #0<br/>seeds: entry, owner, 0"]
    PROG --> E1["Entry PDA #1<br/>seeds: entry, owner, 1"]
    PROG --> EN["Entry PDA #N<br/>seeds: entry, owner, N"]

    VAULT -->|tracks| NI[next_index]
    VAULT -->|tracks| EC[entry_count]
    VAULT -->|tracks| FS["free_slots[]"]
```

**VaultMeta** fields:
- `owner` - deployer public key
- `next_index` - next unused slot
- `entry_count` - number of active entries
- `free_slots` - deleted slot indices available for reuse (max 200)

**EntryAccount** fields:
- `owner` - deployer public key
- `index` - slot number
- `encrypted_data` - AES-256-CBC encrypted JSON
- `created_at` / `updated_at` - Solana clock timestamps (used for conflict detection)

Slot reuse: when an entry is deleted, its index goes to `free_slots`. Next allocation picks from `free_slots` first, then `next_index`.
