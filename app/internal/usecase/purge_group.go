package usecase

import (
	"context"
	"fmt"

	"github.com/solock/solock/internal/domain"
)

type PurgeGroupUseCase struct {
	groups domain.GroupRepository
	vault  domain.VaultRepository
}

func NewPurgeGroupUseCase(
	groups domain.GroupRepository,
	vault domain.VaultRepository,
) *PurgeGroupUseCase {
	return &PurgeGroupUseCase{
		groups: groups,
		vault:  vault,
	}
}

type PurgeGroupResult struct {
	OnChain bool
}

func (uc *PurgeGroupUseCase) Execute(ctx context.Context, index uint32) (*PurgeGroupResult, error) {
	group, err := uc.groups.Get(ctx, index)
	if err != nil {
		return nil, fmt.Errorf("get group: %w", err)
	}
	if group == nil {
		return nil, fmt.Errorf("group not found")
	}
	if !group.Deleted() {
		return nil, fmt.Errorf("group must be deleted before purging")
	}

	if err := uc.vault.PurgeGroup(ctx, index); err != nil {
		return nil, fmt.Errorf("purge on-chain: %w", err)
	}

	if err := uc.groups.Delete(ctx, index); err != nil {
		return nil, fmt.Errorf("delete local: %w", err)
	}

	return &PurgeGroupResult{OnChain: true}, nil
}
