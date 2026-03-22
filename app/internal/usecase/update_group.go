package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/solock/solock/internal/domain"
)

type UpdateGroupUseCase struct {
	groups domain.GroupRepository
	vault  domain.VaultRepository
	crypto domain.CryptoService
}

func NewUpdateGroupUseCase(
	groups domain.GroupRepository,
	vault domain.VaultRepository,
	crypto domain.CryptoService,
) *UpdateGroupUseCase {
	return &UpdateGroupUseCase{
		groups: groups,
		vault:  vault,
		crypto: crypto,
	}
}

type UpdateGroupResult struct {
	OnChain bool
}

func (uc *UpdateGroupUseCase) Execute(ctx context.Context, group *domain.Group) (*UpdateGroupResult, error) {
	if err := domain.ValidateGroupName(group.Name()); err != nil {
		return nil, err
	}

	encrypted, err := uc.encryptGroup(group)
	if err != nil {
		return nil, fmt.Errorf("encrypt: %w", err)
	}

	if err := uc.vault.UpdateGroup(ctx, group.Index(), encrypted); err != nil {
		return nil, fmt.Errorf("update on-chain: %w", err)
	}

	if err := uc.groups.Save(ctx, group); err != nil {
		return nil, fmt.Errorf("save local: %w", err)
	}

	return &UpdateGroupResult{OnChain: true}, nil
}

func (uc *UpdateGroupUseCase) encryptGroup(group *domain.Group) ([]byte, error) {
	data, err := json.Marshal(group)
	if err != nil {
		return nil, err
	}
	return uc.crypto.Encrypt(data)
}
