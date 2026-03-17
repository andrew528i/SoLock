package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/solock/solock/internal/domain"
)

const syncBatchSize = 100

type SyncProgress struct {
	Phase   string
	Current int
	Total   int
}

type SyncProgressFunc func(SyncProgress)

type SyncUseCase struct {
	entries   domain.EntryRepository
	vault     domain.VaultRepository
	syncState domain.SyncStateRepository
	crypto    domain.CryptoService
}

func NewSyncUseCase(
	entries domain.EntryRepository,
	vault domain.VaultRepository,
	syncState domain.SyncStateRepository,
	crypto domain.CryptoService,
) *SyncUseCase {
	return &SyncUseCase{
		entries:   entries,
		vault:     vault,
		syncState: syncState,
		crypto:    crypto,
	}
}

func (uc *SyncUseCase) Execute(ctx context.Context, onProgress SyncProgressFunc) error {
	if onProgress == nil {
		onProgress = func(SyncProgress) {}
	}

	onProgress(SyncProgress{Phase: "Reading vault metadata..."})

	meta, err := uc.vault.GetMeta(ctx)
	if err != nil {
		return fmt.Errorf("get vault meta: %w", err)
	}

	nextIndex := meta.NextIndex
	if nextIndex > 10000 || meta.EntryCount > 10000 {
		return uc.resetAndRepush(ctx, onProgress)
	}

	total := int(nextIndex)
	onProgress(SyncProgress{Phase: "Fetching entries...", Total: total})

	remoteSlots := make(map[uint32]bool)
	skipped := 0
	fetched := 0

	for start := uint32(0); start < nextIndex; start += syncBatchSize {
		if err := ctx.Err(); err != nil {
			return err
		}

		end := min(start+syncBatchSize, nextIndex)
		indices := makeRange(start, end)

		accounts, err := uc.vault.GetEntriesBatch(ctx, indices)
		if err != nil {
			return fmt.Errorf("fetch batch %d-%d: %w", start, end, err)
		}

		for idx, account := range accounts {
			remoteSlots[idx] = true

			entry, err := uc.decryptEntry(account.EncryptedData)
			if err != nil {
				skipped++
				continue
			}
			entry.SetSlotIndex(idx)
			entry.SetOnChainTimestamps(account.CreatedAt, account.UpdatedAt)

			if err := uc.entries.Save(ctx, entry); err != nil {
				return fmt.Errorf("save entry %d: %w", idx, err)
			}
			uc.entries.MarkSynced(ctx, entry.ID())
		}

		fetched += len(indices)
		onProgress(SyncProgress{Phase: "Fetching entries...", Current: fetched, Total: total})
	}

	if nextIndex > 0 && len(remoteSlots) > 0 {
		localEntries, err := uc.entries.List(ctx)
		if err != nil {
			return fmt.Errorf("list local: %w", err)
		}
		for _, entry := range localEntries {
			if entry.SlotIndex() < nextIndex && !remoteSlots[entry.SlotIndex()] {
				uc.entries.Delete(ctx, entry.ID())
			}
		}
	}

	if skipped > 0 {
		onProgress(SyncProgress{
			Phase: fmt.Sprintf("Warning: %d entries could not be decrypted", skipped),
			Current: total, Total: total,
		})
	}

	count, _ := uc.entries.Count(ctx)
	if err := uc.syncState.Set(ctx, &domain.SyncState{
		NextIndex:  nextIndex,
		EntryCount: uint32(count),
		LastSyncAt: time.Now().UTC(),
	}); err != nil {
		return fmt.Errorf("save sync state: %w", err)
	}

	onProgress(SyncProgress{Phase: "Done", Current: total, Total: total})
	return nil
}

func (uc *SyncUseCase) resetAndRepush(ctx context.Context, onProgress SyncProgressFunc) error {
	onProgress(SyncProgress{Phase: "Resetting vault..."})

	if err := uc.vault.Reset(ctx); err != nil {
		return fmt.Errorf("reset vault: %w", err)
	}

	localEntries, err := uc.entries.List(ctx)
	if err != nil {
		return fmt.Errorf("list local: %w", err)
	}

	total := len(localEntries)
	onProgress(SyncProgress{Phase: "Pushing local entries...", Total: total})

	pushed := 0
	for i, entry := range localEntries {
		if err := ctx.Err(); err != nil {
			uc.saveSyncState(ctx, uint32(pushed))
			return fmt.Errorf("cancelled after %d/%d entries", pushed, total)
		}

		entry.SetSlotIndex(uint32(i))
		encrypted, err := uc.encryptEntry(entry)
		if err != nil {
			uc.saveSyncState(ctx, uint32(pushed))
			return fmt.Errorf("encrypt entry %d: %w", i, err)
		}

		if err := uc.vault.AddEntry(ctx, uint32(i), encrypted); err != nil {
			uc.saveSyncState(ctx, uint32(pushed))
			return fmt.Errorf("push entry %d: %w", i, err)
		}

		uc.entries.Save(ctx, entry)
		uc.entries.MarkSynced(ctx, entry.ID())
		pushed++
		onProgress(SyncProgress{Phase: "Pushing local entries...", Current: pushed, Total: total})
	}

	uc.saveSyncState(ctx, uint32(pushed))
	onProgress(SyncProgress{Phase: "Done", Current: total, Total: total})
	return nil
}

func (uc *SyncUseCase) saveSyncState(ctx context.Context, nextIndex uint32) {
	count, _ := uc.entries.Count(ctx)
	uc.syncState.Set(ctx, &domain.SyncState{
		NextIndex:  nextIndex,
		EntryCount: uint32(count),
		LastSyncAt: time.Now().UTC(),
	})
}

func (uc *SyncUseCase) encryptEntry(entry *domain.Entry) ([]byte, error) {
	data, err := json.Marshal(entry)
	if err != nil {
		return nil, err
	}
	return uc.crypto.Encrypt(data)
}

func (uc *SyncUseCase) decryptEntry(encrypted []byte) (*domain.Entry, error) {
	decrypted, err := uc.crypto.Decrypt(encrypted)
	if err != nil {
		return nil, err
	}
	var entry domain.Entry
	if err := json.Unmarshal(decrypted, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

func makeRange(start, end uint32) []uint32 {
	r := make([]uint32, 0, end-start)
	for i := start; i < end; i++ {
		r = append(r, i)
	}
	return r
}
