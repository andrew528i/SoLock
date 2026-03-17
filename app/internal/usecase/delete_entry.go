package usecase

import (
	"context"
	"fmt"

	"github.com/solock/solock/internal/domain"
)

type DeleteEntryUseCase struct {
	entries domain.EntryRepository
	vault   domain.VaultRepository
}

func NewDeleteEntryUseCase(
	entries domain.EntryRepository,
	vault domain.VaultRepository,
) *DeleteEntryUseCase {
	return &DeleteEntryUseCase{
		entries: entries,
		vault:   vault,
	}
}

type DeleteEntryResult struct {
	OnChain bool
}

func (uc *DeleteEntryUseCase) Execute(ctx context.Context, entry *domain.Entry) (*DeleteEntryResult, error) {
	if uc.vault == nil {
		return uc.deleteLocalOnly(ctx, entry)
	}

	existing, _ := uc.vault.GetEntry(ctx, entry.SlotIndex())
	if existing == nil {
		return uc.deleteLocalOnly(ctx, entry)
	}

	if err := uc.vault.DeleteEntry(ctx, entry.SlotIndex()); err != nil {
		return nil, fmt.Errorf("delete on-chain: %w", err)
	}

	if err := uc.entries.Delete(ctx, entry.ID()); err != nil {
		return nil, fmt.Errorf("delete local: %w", err)
	}

	return &DeleteEntryResult{OnChain: true}, nil
}

func (uc *DeleteEntryUseCase) deleteLocalOnly(ctx context.Context, entry *domain.Entry) (*DeleteEntryResult, error) {
	if err := uc.entries.Delete(ctx, entry.ID()); err != nil {
		return nil, err
	}
	return &DeleteEntryResult{OnChain: false}, nil
}
