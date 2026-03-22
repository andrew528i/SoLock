package usecase

import (
	"context"
	"fmt"

	"github.com/solock/solock/internal/domain"
)

type TransferAllUseCase struct {
	vault domain.VaultRepository
}

func NewTransferAllUseCase(vault domain.VaultRepository) *TransferAllUseCase {
	return &TransferAllUseCase{vault: vault}
}

func (uc *TransferAllUseCase) Execute(ctx context.Context, to string) error {
	if uc.vault == nil {
		return fmt.Errorf("not connected")
	}
	if to == "" {
		return fmt.Errorf("recipient address is required")
	}
	return uc.vault.TransferAll(ctx, to)
}
