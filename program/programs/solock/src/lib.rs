use anchor_lang::prelude::*;

declare_id!("6cCyHuQS1M6FV821Vp7zG5Er14aZ7TYnQyQXeL49EjDQ");

const FREE_SLOTS_CAPACITY: usize = 200;
const VAULT_BASE_SPACE: usize = 8 + 32 + 4 + 4 + 1 + 4 + FREE_SLOTS_CAPACITY * 4;

#[program]
pub mod solock {
    use super::*;

    pub fn initialize_vault(ctx: Context<InitializeVault>) -> Result<()> {
        let vault = &mut ctx.accounts.vault;
        vault.owner = ctx.accounts.owner.key();
        vault.next_index = 0;
        vault.entry_count = 0;
        vault.bump = ctx.bumps.vault;
        vault.free_slots = Vec::new();
        Ok(())
    }

    pub fn reset_vault(ctx: Context<ResetVault>) -> Result<()> {
        let vault = &mut ctx.accounts.vault;
        vault.owner = ctx.accounts.owner.key();
        vault.next_index = 0;
        vault.entry_count = 0;
        vault.bump = ctx.bumps.vault;
        vault.free_slots = Vec::new();
        Ok(())
    }

    pub fn add_entry(ctx: Context<AddEntry>, index: u32, encrypted_data: Vec<u8>) -> Result<()> {
        let vault = &mut ctx.accounts.vault;
        let entry = &mut ctx.accounts.entry;
        let clock = Clock::get()?;
        require!(vault.owner == ctx.accounts.owner.key(), SoLockError::Unauthorized);
        require!(!encrypted_data.is_empty(), SoLockError::EmptyData);

        if let Some(pos) = vault.free_slots.iter().position(|&s| s == index) {
            vault.free_slots.remove(pos);
        }

        entry.owner = ctx.accounts.owner.key();
        entry.index = index;
        entry.encrypted_data = encrypted_data;
        entry.created_at = clock.unix_timestamp;
        entry.updated_at = clock.unix_timestamp;
        entry.bump = ctx.bumps.entry;
        vault.entry_count = vault.entry_count.checked_add(1).unwrap();
        if index >= vault.next_index {
            vault.next_index = index.checked_add(1).unwrap();
        }
        Ok(())
    }

    pub fn update_entry(ctx: Context<UpdateEntry>, encrypted_data: Vec<u8>, expected_updated_at: i64) -> Result<()> {
        let vault = &ctx.accounts.vault;
        let entry = &mut ctx.accounts.entry;
        let clock = Clock::get()?;
        require!(vault.owner == ctx.accounts.owner.key(), SoLockError::Unauthorized);
        require!(!encrypted_data.is_empty(), SoLockError::EmptyData);
        if expected_updated_at != 0 {
            require!(entry.updated_at == expected_updated_at, SoLockError::ConflictDetected);
        }
        entry.encrypted_data = encrypted_data;
        entry.updated_at = clock.unix_timestamp;
        Ok(())
    }

    pub fn delete_entry(ctx: Context<DeleteEntry>) -> Result<()> {
        let vault = &mut ctx.accounts.vault;
        let index = ctx.accounts.entry.index;
        require!(vault.owner == ctx.accounts.owner.key(), SoLockError::Unauthorized);
        require!(vault.free_slots.len() < 200, SoLockError::FreeSlotsFull);
        vault.entry_count = vault.entry_count.saturating_sub(1);
        vault.free_slots.push(index);
        Ok(())
    }
}

#[derive(Accounts)]
pub struct InitializeVault<'info> {
    #[account(mut)]
    pub owner: Signer<'info>,
    #[account(init_if_needed, payer = owner, space = VAULT_BASE_SPACE, seeds = [b"vault_v2", owner.key().as_ref()], bump)]
    pub vault: Account<'info, VaultMeta>,
    pub system_program: Program<'info, System>,
}

#[derive(Accounts)]
pub struct ResetVault<'info> {
    #[account(mut)]
    pub owner: Signer<'info>,
    #[account(
        mut,
        realloc = VAULT_BASE_SPACE,
        realloc::payer = owner,
        realloc::zero = true,
        seeds = [b"vault_v2", owner.key().as_ref()],
        bump
    )]
    pub vault: Account<'info, VaultMeta>,
    pub system_program: Program<'info, System>,
}

#[derive(Accounts)]
#[instruction(index: u32, encrypted_data: Vec<u8>)]
pub struct AddEntry<'info> {
    #[account(mut)]
    pub owner: Signer<'info>,
    #[account(mut, seeds = [b"vault_v2", owner.key().as_ref()], bump = vault.bump, has_one = owner)]
    pub vault: Account<'info, VaultMeta>,
    #[account(init, payer = owner, space = 8 + EntryAccount::base_space() + encrypted_data.len(), seeds = [b"entry", owner.key().as_ref(), &index.to_le_bytes()], bump)]
    pub entry: Account<'info, EntryAccount>,
    pub system_program: Program<'info, System>,
}

#[derive(Accounts)]
#[instruction(encrypted_data: Vec<u8>)]
pub struct UpdateEntry<'info> {
    #[account(mut)]
    pub owner: Signer<'info>,
    #[account(seeds = [b"vault_v2", owner.key().as_ref()], bump = vault.bump, has_one = owner)]
    pub vault: Account<'info, VaultMeta>,
    #[account(mut, realloc = 8 + EntryAccount::base_space() + encrypted_data.len(), realloc::payer = owner, realloc::zero = false, seeds = [b"entry", owner.key().as_ref(), &entry.index.to_le_bytes()], bump = entry.bump)]
    pub entry: Account<'info, EntryAccount>,
    pub system_program: Program<'info, System>,
}

#[derive(Accounts)]
pub struct DeleteEntry<'info> {
    #[account(mut)]
    pub owner: Signer<'info>,
    #[account(mut, seeds = [b"vault_v2", owner.key().as_ref()], bump = vault.bump, has_one = owner)]
    pub vault: Account<'info, VaultMeta>,
    #[account(mut, close = owner, seeds = [b"entry", owner.key().as_ref(), &entry.index.to_le_bytes()], bump = entry.bump, has_one = owner)]
    pub entry: Account<'info, EntryAccount>,
    pub system_program: Program<'info, System>,
}

#[account]
pub struct VaultMeta {
    pub owner: Pubkey,
    pub next_index: u32,
    pub entry_count: u32,
    pub bump: u8,
    pub free_slots: Vec<u32>,
}

#[account]
pub struct EntryAccount {
    pub owner: Pubkey,
    pub index: u32,
    pub encrypted_data: Vec<u8>,
    pub created_at: i64,
    pub updated_at: i64,
    pub bump: u8,
}

impl EntryAccount {
    pub const fn base_space() -> usize { 32 + 4 + 4 + 8 + 8 + 1 }
}

#[error_code]
pub enum SoLockError {
    #[msg("Unauthorized")]
    Unauthorized,
    #[msg("Empty data")]
    EmptyData,
    #[msg("Free slots full, add entries to reuse deleted slots first")]
    FreeSlotsFull,
    #[msg("Entry was modified by another client, sync first")]
    ConflictDetected,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn entry_base_space_matches_fields() {
        // owner(32) + index(4) + encrypted_data length prefix(4) + created_at(8) + updated_at(8) + bump(1)
        assert_eq!(EntryAccount::base_space(), 32 + 4 + 4 + 8 + 8 + 1);
    }

    #[test]
    fn vault_base_space_fits_free_slots() {
        // discriminator(8) + owner(32) + next_index(4) + entry_count(4) + bump(1) + vec length prefix(4) + 200 * u32(4)
        assert_eq!(VAULT_BASE_SPACE, 8 + 32 + 4 + 4 + 1 + 4 + FREE_SLOTS_CAPACITY * 4);
    }

    #[test]
    fn free_slots_capacity_is_200() {
        assert_eq!(FREE_SLOTS_CAPACITY, 200);
    }

    #[test]
    fn vault_base_space_value() {
        // 8 + 32 + 4 + 4 + 1 + 4 + 800 = 853
        assert_eq!(VAULT_BASE_SPACE, 853);
    }
}
