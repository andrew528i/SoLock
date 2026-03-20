package usecase

import (
	"context"
	"time"

	"github.com/solock/solock/internal/domain"
)

type DashboardInfo struct {
	DeployerAddress string
	ProgramID       string
	Balance         uint64
	ProgramDeployed bool
	VaultExists     bool
	EntryCount      int
	PasswordCount   int
	NoteCount       int
	CardCount       int
	TOTPCount       int
	LastSyncAt      time.Time
	Network         string
	RPCURL          string
}

type GetDashboardUseCase struct {
	vault     domain.VaultRepository
	entries   domain.EntryRepository
	config    domain.ConfigRepository
	syncState domain.SyncStateRepository
	keys      *domain.DerivedKeys
}

func NewGetDashboardUseCase(
	vault domain.VaultRepository,
	entries domain.EntryRepository,
	config domain.ConfigRepository,
	syncState domain.SyncStateRepository,
	keys *domain.DerivedKeys,
) *GetDashboardUseCase {
	return &GetDashboardUseCase{
		vault:     vault,
		entries:   entries,
		config:    config,
		syncState: syncState,
		keys:      keys,
	}
}

func (uc *GetDashboardUseCase) Execute(ctx context.Context) (*DashboardInfo, error) {
	info := &DashboardInfo{
		DeployerAddress: uc.keys.DeployerAddress,
		ProgramID:       uc.keys.ProgramID,
	}

	if network, _ := uc.config.Get(ctx, "network"); network != "" {
		info.Network = network
	}
	if rpcURL, _ := uc.config.Get(ctx, "rpc_url"); rpcURL != "" {
		info.RPCURL = rpcURL
	}

	if uc.vault != nil {
		info.Balance, _ = uc.vault.GetBalance(ctx)
		info.ProgramDeployed, _ = uc.vault.IsProgramDeployed(ctx)
		info.VaultExists, _ = uc.vault.Exists(ctx)
	}

	if uc.entries != nil {
		entries, _ := uc.entries.List(ctx)
		info.EntryCount = len(entries)
		for _, e := range entries {
			switch e.Type() {
			case domain.EntryTypePassword:
				info.PasswordCount++
			case domain.EntryTypeNote:
				info.NoteCount++
			case domain.EntryTypeCard:
				info.CardCount++
			}
			if e.HasTOTP() {
				info.TOTPCount++
			}
		}
	}

	if uc.syncState != nil {
		if state, err := uc.syncState.Get(ctx); err == nil {
			info.LastSyncAt = state.LastSyncAt
		}
	}

	return info, nil
}
