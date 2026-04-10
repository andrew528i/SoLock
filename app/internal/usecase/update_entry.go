package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/solock/solock/internal/domain"
)

type UpdateEntryUseCase struct {
	entries domain.EntryRepository
	vault   domain.VaultRepository
	crypto  domain.CryptoService
}

func NewUpdateEntryUseCase(
	entries domain.EntryRepository,
	vault domain.VaultRepository,
	crypto domain.CryptoService,
) *UpdateEntryUseCase {
	return &UpdateEntryUseCase{
		entries: entries,
		vault:   vault,
		crypto:  crypto,
	}
}

type UpdateEntryResult struct {
	OnChain bool
}

func (uc *UpdateEntryUseCase) Execute(ctx context.Context, entry *domain.Entry) (*UpdateEntryResult, error) {
	if uc.vault == nil {
		return uc.saveLocalOnly(ctx, entry)
	}

	encrypted, err := uc.encryptEntry(entry)
	if err != nil {
		return nil, fmt.Errorf("encrypt: %w", err)
	}

	for attempt := 0; attempt < 2; attempt++ {
		existing, _ := uc.vault.GetEntry(ctx, entry.SlotIndex())
		if existing == nil {
			return uc.saveLocalOnly(ctx, entry)
		}

		err := uc.vault.UpdateEntry(ctx, entry.SlotIndex(), encrypted, existing.UpdatedAt)
		if err == nil {
			if onChain, _ := uc.vault.GetEntry(ctx, entry.SlotIndex()); onChain != nil {
				entry.SetOnChainTimestamps(onChain.CreatedAt, onChain.UpdatedAt)
			}
			if err := uc.entries.Save(ctx, entry); err != nil {
				return nil, fmt.Errorf("save local: %w", err)
			}
			if err := uc.entries.MarkSynced(ctx, entry.ID()); err != nil {
				return nil, fmt.Errorf("mark synced: %w", err)
			}
			return &UpdateEntryResult{OnChain: true}, nil
		}

		if !isConflictError(err) {
			return nil, fmt.Errorf("update on-chain: %w", err)
		}
	}

	return nil, fmt.Errorf("conflict: entry modified by another client, sync first")
}

func (uc *UpdateEntryUseCase) saveLocalOnly(ctx context.Context, entry *domain.Entry) (*UpdateEntryResult, error) {
	if err := uc.entries.Save(ctx, entry); err != nil {
		return nil, err
	}
	return &UpdateEntryResult{OnChain: false}, nil
}

func (uc *UpdateEntryUseCase) encryptEntry(entry *domain.Entry) ([]byte, error) {
	data, err := json.Marshal(entry)
	if err != nil {
		return nil, err
	}
	return uc.crypto.Encrypt(data)
}

func isConflictError(err error) bool {
	s := err.Error()
	return strings.Contains(s, "ConflictDetected") || strings.Contains(s, "0x1774")
}
