package usecase

import (
	"context"
	"fmt"

	"github.com/solock/solock/internal/domain"
	"github.com/solock/solock/internal/repository/adapter"
)

type DeployProgramUseCase struct {
	vault domain.VaultRepository
	keys  *domain.DerivedKeys
}

func NewDeployProgramUseCase(vault domain.VaultRepository, keys *domain.DerivedKeys) *DeployProgramUseCase {
	return &DeployProgramUseCase{vault: vault, keys: keys}
}

func (uc *DeployProgramUseCase) Execute(ctx context.Context) error {
	if uc.vault == nil || uc.keys == nil {
		return fmt.Errorf("not connected")
	}
	binary, err := adapter.PatchedProgramBinary(uc.keys.ProgramID)
	if err != nil {
		return fmt.Errorf("patch program: %w", err)
	}
	return uc.vault.DeployProgram(ctx, binary)
}
