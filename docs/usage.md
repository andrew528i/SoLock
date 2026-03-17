# Usage

## First launch

1. Run `./solock`
2. Enter a master password - this deterministically generates your Solana keypair and encryption key
3. Fund the deployer address with ~3 SOL (shown on dashboard, press `p` to copy)
4. Press `d` to deploy the Solana program
5. Press `i` to initialize the vault
6. Start adding entries

## Screens

### Unlock

Enter your master password. The same password always produces the same keys on any machine.

### Dashboard

Overview of your vault state:

- **Deployer** - your Solana address (press `p` to copy)
- **Balance** - SOL available for transactions
- **Program** - deployed / not deployed
- **Vault** - initialized / not initialized
- **Entries** - count and estimated rent cost
- **Synced** - last sync timestamp

### Vault

List of all entries with search and sort.

| Key | Action |
|-----|--------|
| `j`/`k` or arrows | Navigate |
| `enter` or `l` | View entry |
| `/` | Search |
| `s` | Cycle sort mode |
| `a` | Add entry |
| `d` | Duplicate entry |
| `x` | Delete entry |
| `esc` or `h` | Back to dashboard |

### Entry view

Shows all fields of an entry. Sensitive fields are masked by default.

| Key | Action |
|-----|--------|
| `s` | Show/hide secrets |
| `c` | Copy password to clipboard |
| `t` | Copy TOTP code to clipboard |
| `e` | Edit entry |
| `x` | Delete entry |
| `esc` or `h` | Back to vault |

### Entry form

Tab-based form for adding/editing entries. Password fields support built-in generation.

| Key | Action |
|-----|--------|
| `tab` / `shift+tab` | Next / previous field |
| `enter` | Generate password (on password field) or save (on last field) |
| `right` | Open password generator options |
| `esc` | Cancel |

### Password generator

Inline panel for configuring generated passwords:

- **Length** - 8 to 64 characters (left/right arrows)
- **Uppercase** - A-Z toggle
- **Digits** - 0-9 toggle
- **Special** - !@#$%^&* toggle
- Press `g` to generate and apply

### Dashboard keys

| Key | Action |
|-----|--------|
| `v` | Open vault |
| `a` | Add entry (shortcut) |
| `s` | Sync with Solana |
| `d` | Deploy / redeploy program |
| `i` | Initialize / reset vault |
| `n` | Switch network (devnet / mainnet-beta) |
| `r` | Refresh / retry failed sync |
| `p` | Copy deployer address |
| `x` | Clear local database |
| `c` | Config |
| `q` | Quit |

## Entry types

| Type | Fields |
|------|--------|
| **Password** | site, username, password, totp_secret, notes |
| **Note** | content |
| **Card** | cardholder, number, expiry, cvv, notes |

TOTP codes are generated automatically for password entries that have a `totp_secret` field filled in.

## Networks

SoLock supports devnet and mainnet-beta. Press `n` on the dashboard to switch. Network preference is saved in the local encrypted config.

- **devnet** - free SOL via faucet, for testing
- **mainnet-beta** - real SOL, production use

## Data storage

- **On-chain** - encrypted entry data on Solana blockchain (accessible from any machine)
- **Local** - SQLite cache at `~/.solock/vault.db` (all values encrypted with your master password derived key)

The local cache is a performance optimization. All data can be re-synced from the blockchain at any time.
