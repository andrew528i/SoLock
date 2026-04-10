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
	Phase            string
	Current          int
	Total            int
	CorruptedEntries []uint32
	CorruptedGroups  []uint32
}

type SyncProgressFunc func(SyncProgress)

type SyncUseCase struct {
	entries   domain.EntryRepository
	groups    domain.GroupRepository
	vault     domain.VaultRepository
	syncState domain.SyncStateRepository
	crypto    domain.CryptoService
}

func NewSyncUseCase(
	entries domain.EntryRepository,
	groups domain.GroupRepository,
	vault domain.VaultRepository,
	syncState domain.SyncStateRepository,
	crypto domain.CryptoService,
) *SyncUseCase {
	return &SyncUseCase{
		entries:   entries,
		groups:    groups,
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

	corruptedGroups, err := uc.syncGroups(ctx, meta, onProgress)
	if err != nil {
		return fmt.Errorf("sync groups: %w", err)
	}

	nextIndex := meta.NextIndex
	total := int(nextIndex)
	onProgress(SyncProgress{Phase: "Fetching entries...", Total: total})

	remoteSlots := make(map[uint32]bool)
	corruptedEntries := make([]uint32, 0)
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
				corruptedEntries = append(corruptedEntries, idx)
				continue
			}
			entry.SetSlotIndex(idx)
			entry.SetOnChainTimestamps(account.CreatedAt, account.UpdatedAt)

			if err := uc.entries.Save(ctx, entry); err != nil {
				return fmt.Errorf("save entry %d: %w", idx, err)
			}
			if err := uc.entries.MarkSynced(ctx, entry.ID()); err != nil {
				return fmt.Errorf("mark entry %d synced: %w", idx, err)
			}
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
				if err := uc.entries.Delete(ctx, entry.ID()); err != nil {
					return fmt.Errorf("delete stale entry %s: %w", entry.ID(), err)
				}
			}
		}
	}

	count, err := uc.entries.Count(ctx)
	if err != nil {
		return fmt.Errorf("count local entries: %w", err)
	}
	if err := uc.syncState.Set(ctx, &domain.SyncState{
		NextIndex:  nextIndex,
		EntryCount: uint32(count),
		LastSyncAt: time.Now().UTC(),
	}); err != nil {
		return fmt.Errorf("save sync state: %w", err)
	}

	onProgress(SyncProgress{
		Phase:            "Done",
		Current:          total,
		Total:            total,
		CorruptedEntries: corruptedEntries,
		CorruptedGroups:  corruptedGroups,
	})
	return nil
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

func (uc *SyncUseCase) syncGroups(ctx context.Context, meta *domain.VaultMeta, onProgress SyncProgressFunc) ([]uint32, error) {
	nextGroupIndex := meta.NextGroupIndex
	if nextGroupIndex == 0 {
		return nil, nil
	}

	total := int(nextGroupIndex)
	onProgress(SyncProgress{Phase: "Fetching groups...", Total: total})

	remoteIndices := make(map[uint32]bool)
	corrupted := make([]uint32, 0)
	fetched := 0

	for start := uint32(0); start < nextGroupIndex; start += syncBatchSize {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		end := min(start+syncBatchSize, nextGroupIndex)
		indices := makeRange(start, end)

		accounts, err := uc.vault.GetGroupsBatch(ctx, indices)
		if err != nil {
			return nil, fmt.Errorf("fetch group batch %d-%d: %w", start, end, err)
		}

		for idx, account := range accounts {
			remoteIndices[idx] = true

			group, err := uc.decryptGroup(account.EncryptedData)
			if err != nil {
				corrupted = append(corrupted, idx)
				continue
			}
			group.SetIndex(idx)
			group.SetDeleted(account.Deleted)

			if err := uc.groups.Save(ctx, group); err != nil {
				return nil, fmt.Errorf("save group %d: %w", idx, err)
			}
		}

		fetched += len(indices)
		onProgress(SyncProgress{Phase: "Fetching groups...", Current: fetched, Total: total})
	}

	localGroups, err := uc.groups.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list local groups: %w", err)
	}
	for _, g := range localGroups {
		if g.Index() < nextGroupIndex && !remoteIndices[g.Index()] {
			if err := uc.groups.Delete(ctx, g.Index()); err != nil {
				return nil, fmt.Errorf("delete stale group %d: %w", g.Index(), err)
			}
		}
	}

	return corrupted, nil
}

func (uc *SyncUseCase) decryptGroup(encrypted []byte) (*domain.Group, error) {
	decrypted, err := uc.crypto.Decrypt(encrypted)
	if err != nil {
		return nil, err
	}
	var group domain.Group
	if err := json.Unmarshal(decrypted, &group); err != nil {
		return nil, err
	}
	return &group, nil
}

func makeRange(start, end uint32) []uint32 {
	r := make([]uint32, 0, end-start)
	for i := start; i < end; i++ {
		r = append(r, i)
	}
	return r
}
