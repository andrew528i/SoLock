package usecase

import (
	"context"
	"fmt"

	"github.com/solock/solock/internal/domain"
)

type DeleteGroupUseCase struct {
	groups  domain.GroupRepository
	entries domain.EntryRepository
	vault   domain.VaultRepository
}

func NewDeleteGroupUseCase(
	groups domain.GroupRepository,
	entries domain.EntryRepository,
	vault domain.VaultRepository,
) *DeleteGroupUseCase {
	return &DeleteGroupUseCase{
		groups:  groups,
		entries: entries,
		vault:   vault,
	}
}

type DeleteGroupResult struct {
	OnChain      bool
	EntriesMoved int
}

func (uc *DeleteGroupUseCase) Execute(ctx context.Context, index uint32, deleteEntries bool) (*DeleteGroupResult, error) {
	group, err := uc.groups.Get(ctx, index)
	if err != nil {
		return nil, fmt.Errorf("get group: %w", err)
	}
	if group == nil {
		return nil, fmt.Errorf("group not found")
	}

	if deleteEntries {
		if err := uc.deleteGroupEntries(ctx, index); err != nil {
			return nil, fmt.Errorf("delete group entries: %w", err)
		}
	}

	if err := uc.vault.DeleteGroup(ctx, index); err != nil {
		return nil, fmt.Errorf("delete on-chain: %w", err)
	}

	group.SetDeleted(true)
	if err := uc.groups.Save(ctx, group); err != nil {
		return nil, fmt.Errorf("save local: %w", err)
	}

	return &DeleteGroupResult{OnChain: true}, nil
}

func (uc *DeleteGroupUseCase) deleteGroupEntries(ctx context.Context, groupIndex uint32) error {
	allEntries, err := uc.entries.List(ctx)
	if err != nil {
		return err
	}
	for _, entry := range allEntries {
		if entry.GroupIndex() != nil && *entry.GroupIndex() == groupIndex {
			existing, _ := uc.vault.GetEntry(ctx, entry.SlotIndex())
			if existing != nil {
				if err := uc.vault.DeleteEntry(ctx, entry.SlotIndex()); err != nil {
					return fmt.Errorf("delete entry %s on-chain: %w", entry.ID(), err)
				}
			}
			if err := uc.entries.Delete(ctx, entry.ID()); err != nil {
				return fmt.Errorf("delete entry %s local: %w", entry.ID(), err)
			}
		}
	}
	return nil
}
