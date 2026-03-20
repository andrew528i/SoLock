package usecase

import (
	"context"
	"fmt"

	"github.com/solock/solock/internal/domain"
)

type InitVaultUseCase struct {
	vault domain.VaultRepository
}

func NewInitVaultUseCase(vault domain.VaultRepository) *InitVaultUseCase {
	return &InitVaultUseCase{vault: vault}
}

func (uc *InitVaultUseCase) Execute(ctx context.Context) error {
	if uc.vault == nil {
		return fmt.Errorf("not connected")
	}
	return uc.vault.Initialize(ctx)
}

type ResetVaultUseCase struct {
	vault domain.VaultRepository
}

func NewResetVaultUseCase(vault domain.VaultRepository) *ResetVaultUseCase {
	return &ResetVaultUseCase{vault: vault}
}

func (uc *ResetVaultUseCase) Execute(ctx context.Context) error {
	if uc.vault == nil {
		return fmt.Errorf("not connected")
	}
	return uc.vault.Reset(ctx)
}

type CheckBalanceUseCase struct {
	vault domain.VaultRepository
}

func NewCheckBalanceUseCase(vault domain.VaultRepository) *CheckBalanceUseCase {
	return &CheckBalanceUseCase{vault: vault}
}

func (uc *CheckBalanceUseCase) Execute(ctx context.Context) (uint64, error) {
	if uc.vault == nil {
		return 0, nil
	}
	return uc.vault.GetBalance(ctx)
}

type CheckDeployStatusUseCase struct {
	vault domain.VaultRepository
}

func NewCheckDeployStatusUseCase(vault domain.VaultRepository) *CheckDeployStatusUseCase {
	return &CheckDeployStatusUseCase{vault: vault}
}

func (uc *CheckDeployStatusUseCase) Execute(ctx context.Context) (bool, error) {
	if uc.vault == nil {
		return false, nil
	}
	return uc.vault.IsProgramDeployed(ctx)
}

type CheckVaultStatusUseCase struct {
	vault domain.VaultRepository
}

func NewCheckVaultStatusUseCase(vault domain.VaultRepository) *CheckVaultStatusUseCase {
	return &CheckVaultStatusUseCase{vault: vault}
}

func (uc *CheckVaultStatusUseCase) Execute(ctx context.Context) (bool, error) {
	if uc.vault == nil {
		return false, nil
	}
	return uc.vault.Exists(ctx)
}

type ClearLocalDataUseCase struct {
	entries domain.EntryRepository
}

func NewClearLocalDataUseCase(entries domain.EntryRepository) *ClearLocalDataUseCase {
	return &ClearLocalDataUseCase{entries: entries}
}

func (uc *ClearLocalDataUseCase) Execute(ctx context.Context) error {
	if uc.entries == nil {
		return fmt.Errorf("not ready")
	}
	return uc.entries.ClearAll(ctx)
}
