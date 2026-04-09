package usecase

import (
	"context"
	"fmt"

	"github.com/solock/solock/internal/domain"
)

type GetEntryUseCase struct {
	entries domain.EntryRepository
}

func NewGetEntryUseCase(entries domain.EntryRepository) *GetEntryUseCase {
	return &GetEntryUseCase{entries: entries}
}

func (uc *GetEntryUseCase) Execute(ctx context.Context, id string) (*domain.Entry, error) {
	entry, err := uc.entries.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, fmt.Errorf("entry not found: %s", id)
	}
	_ = uc.entries.TouchAccessed(ctx, id)
	return entry, nil
}
