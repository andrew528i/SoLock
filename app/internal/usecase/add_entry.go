package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/solock/solock/internal/domain"
)

type AddEntryUseCase struct {
	entries   domain.EntryRepository
	vault     domain.VaultRepository
	syncState domain.SyncStateRepository
	crypto    domain.CryptoService
}

func NewAddEntryUseCase(
	entries domain.EntryRepository,
	vault domain.VaultRepository,
	syncState domain.SyncStateRepository,
	crypto domain.CryptoService,
) *AddEntryUseCase {
	return &AddEntryUseCase{
		entries:   entries,
		vault:     vault,
		syncState: syncState,
		crypto:    crypto,
	}
}

type AddEntryResult struct {
	OnChain bool
}

func (uc *AddEntryUseCase) Execute(ctx context.Context, entry *domain.Entry) (*AddEntryResult, error) {
	if uc.vault == nil {
		return uc.saveLocalOnly(ctx, entry)
	}

	exists, err := uc.vault.Exists(ctx)
	if err != nil || !exists {
		return uc.saveLocalOnly(ctx, entry)
	}

	meta, err := uc.vault.GetMeta(ctx)
	if err != nil {
		return uc.saveLocalOnly(ctx, entry)
	}

	slot := meta.AllocateSlot()

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		entry.SetSlotIndex(slot)

		encrypted, err := uc.encryptEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("encrypt: %w", err)
		}

		lastErr = uc.vault.AddEntry(ctx, slot, encrypted)
		if lastErr == nil {
			if onChain, _ := uc.vault.GetEntry(ctx, slot); onChain != nil {
				entry.SetOnChainTimestamps(onChain.CreatedAt, onChain.UpdatedAt)
			}
			if err := uc.entries.Save(ctx, entry); err != nil {
				return nil, fmt.Errorf("save local: %w", err)
			}
			uc.entries.MarkSynced(ctx, entry.ID())
			return &AddEntryResult{OnChain: true}, nil
		}

		fresh, metaErr := uc.vault.GetMeta(ctx)
		if metaErr != nil {
			return nil, fmt.Errorf("add on-chain: %w (retry: %w)", lastErr, metaErr)
		}
		slot = fresh.AllocateSlot()
	}

	return nil, fmt.Errorf("add on-chain after 3 attempts: %w", lastErr)
}

func (uc *AddEntryUseCase) saveLocalOnly(ctx context.Context, entry *domain.Entry) (*AddEntryResult, error) {
	state, _ := uc.syncState.Get(ctx)
	entry.SetSlotIndex(state.NextIndex)
	state.NextIndex++
	state.EntryCount++
	uc.syncState.Set(ctx, state)

	if err := uc.entries.Save(ctx, entry); err != nil {
		return nil, err
	}
	return &AddEntryResult{OnChain: false}, nil
}

func (uc *AddEntryUseCase) encryptEntry(entry *domain.Entry) ([]byte, error) {
	data, err := json.Marshal(entry)
	if err != nil {
		return nil, err
	}
	return uc.crypto.Encrypt(data)
}
