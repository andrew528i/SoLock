package usecase

import (
	"context"
	"fmt"

	"github.com/solock/solock/internal/domain"
)

type CloseProgramUseCase struct {
	vault domain.VaultRepository
}

func NewCloseProgramUseCase(vault domain.VaultRepository) *CloseProgramUseCase {
	return &CloseProgramUseCase{vault: vault}
}

func (uc *CloseProgramUseCase) Execute(ctx context.Context) error {
	if uc.vault == nil {
		return fmt.Errorf("not connected")
	}
	return uc.vault.CloseProgram(ctx)
}
