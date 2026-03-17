# SoLock Solana Program

Anchor-based Solana program that manages encrypted password vault storage on-chain.

## Instructions

| Instruction | Description |
|-------------|-------------|
| `initialize_vault` | Create a VaultMeta PDA for the owner |
| `reset_vault` | Clear vault state (realloc to base size) |
| `add_entry` | Create a new encrypted entry at a given slot index |
| `update_entry` | Update entry data with optimistic locking via `expected_updated_at` |
| `delete_entry` | Close entry account, return rent, add slot to free list |

## Accounts

### VaultMeta

PDA seeds: `["vault_v2", owner_pubkey]`

| Field | Type | Description |
|-------|------|-------------|
| owner | Pubkey | Vault owner |
| next_index | u32 | Next available slot |
| entry_count | u32 | Active entry count |
| bump | u8 | PDA bump seed |
| free_slots | Vec\<u32\> | Deleted slots available for reuse (max 200) |

### EntryAccount

PDA seeds: `["entry", owner_pubkey, index_le_bytes]`

| Field | Type | Description |
|-------|------|-------------|
| owner | Pubkey | Entry owner |
| index | u32 | Slot index |
| encrypted_data | Vec\<u8\> | AES-256-CBC encrypted entry JSON |
| created_at | i64 | Solana clock timestamp |
| updated_at | i64 | Solana clock timestamp (used for conflict detection) |
| bump | u8 | PDA bump seed |

## Conflict detection

`update_entry` accepts an `expected_updated_at` parameter. If non-zero and the on-chain `updated_at` doesn't match, the instruction fails with `ConflictDetected`. This implements optimistic locking for multi-client scenarios.

## Placeholder program ID

The source contains a placeholder `declare_id` that gets patched in the compiled .so binary at deploy time. The Go application embeds the .so and replaces the placeholder bytes with the user's actual program ID.

## Building

The program is compiled inside a Docker container:

```bash
# From project root
make program-build
```

This produces `app/internal/repository/adapter/program/solock.so`.

## Dependencies

- anchor-lang 0.30.1 (with `init-if-needed` feature)
