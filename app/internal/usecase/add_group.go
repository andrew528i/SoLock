package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/solock/solock/internal/domain"
)

type AddGroupUseCase struct {
	groups domain.GroupRepository
	vault  domain.VaultRepository
	crypto domain.CryptoService
}

func NewAddGroupUseCase(
	groups domain.GroupRepository,
	vault domain.VaultRepository,
	crypto domain.CryptoService,
) *AddGroupUseCase {
	return &AddGroupUseCase{
		groups: groups,
		vault:  vault,
		crypto: crypto,
	}
}

type AddGroupResult struct {
	OnChain bool
	Group   *domain.Group
}

func (uc *AddGroupUseCase) Execute(ctx context.Context, name string, color domain.GroupColor) (*AddGroupResult, error) {
	if err := domain.ValidateGroupName(name); err != nil {
		return nil, err
	}
	if err := domain.ValidateGroupColor(color); err != nil {
		return nil, err
	}

	meta, err := uc.vault.GetMeta(ctx)
	if err != nil {
		return nil, fmt.Errorf("get vault meta: %w", err)
	}

	slot := meta.AllocateGroupSlot()
	group, err := domain.NewGroup(slot, name)
	if err != nil {
		return nil, err
	}
	if color != "" {
		group.SetColor(color)
	}

	encrypted, err := uc.encryptGroup(group)
	if err != nil {
		return nil, fmt.Errorf("encrypt: %w", err)
	}

	if err := uc.vault.AddGroup(ctx, slot, encrypted); err != nil {
		return nil, fmt.Errorf("add on-chain: %w", err)
	}

	if err := uc.groups.Save(ctx, group); err != nil {
		return nil, fmt.Errorf("save local: %w", err)
	}

	return &AddGroupResult{OnChain: true, Group: group}, nil
}

func (uc *AddGroupUseCase) encryptGroup(group *domain.Group) ([]byte, error) {
	data, err := json.Marshal(group)
	if err != nil {
		return nil, err
	}
	return uc.crypto.Encrypt(data)
}
