package application

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/solock/solock/internal/domain"
	"github.com/solock/solock/internal/domain/service"
	"github.com/solock/solock/internal/usecase"
)

type VaultRepoFactory func(keys *domain.DerivedKeys, rpcURL string) domain.VaultRepository
type StorageFactory func(dbPath string, crypto domain.CryptoService) (domain.EntryRepository, domain.GroupRepository, domain.ConfigRepository, domain.SyncStateRepository, error)

type SyncStatus struct {
	InProgress bool   `json:"in_progress"`
	Phase      string `json:"phase,omitempty"`
	Error      string `json:"error,omitempty"`
}

type App struct {
	dataDir          string
	keyDeriver       domain.KeyDeriver
	vaultRepoFactory VaultRepoFactory
	storageFactory   StorageFactory

	mu        sync.RWMutex
	keys      *domain.DerivedKeys
	crypto    domain.CryptoService
	entries   domain.EntryRepository
	groups    domain.GroupRepository
	config    domain.ConfigRepository
	syncState domain.SyncStateRepository
	vault     domain.VaultRepository
	expiresAt time.Time

	syncMu     sync.RWMutex
	syncStatus SyncStatus

	Unlock             *usecase.UnlockUseCase
	AddEntry           *usecase.AddEntryUseCase
	UpdateEntry        *usecase.UpdateEntryUseCase
	DeleteEntry        *usecase.DeleteEntryUseCase
	Sync               *usecase.SyncUseCase
	ListEntries        *usecase.ListEntriesUseCase
	SearchEntries      *usecase.SearchEntriesUseCase
	GetEntry           *usecase.GetEntryUseCase
	GetDashboard       *usecase.GetDashboardUseCase
	DeployProgram      *usecase.DeployProgramUseCase
	InitVault          *usecase.InitVaultUseCase
	ResetVault         *usecase.ResetVaultUseCase
	CheckBalance       *usecase.CheckBalanceUseCase
	CheckDeployStatus  *usecase.CheckDeployStatusUseCase
	CheckVaultStatus   *usecase.CheckVaultStatusUseCase
	ClearLocalData     *usecase.ClearLocalDataUseCase
	GeneratePassword   *usecase.GeneratePasswordUseCase
	GenerateTOTP       *usecase.GenerateTOTPUseCase
	GetConfig          *usecase.GetConfigUseCase
	SetConfig          *usecase.SetConfigUseCase
	GetPassGenConfig   *usecase.GetPassGenConfigUseCase
	SavePassGenConfig  *usecase.SavePassGenConfigUseCase
	AddGroup           *usecase.AddGroupUseCase
	UpdateGroup        *usecase.UpdateGroupUseCase
	DeleteGroup        *usecase.DeleteGroupUseCase
	PurgeGroup         *usecase.PurgeGroupUseCase
	ListGroups         *usecase.ListGroupsUseCase
	CloseProgram       *usecase.CloseProgramUseCase
	TransferAll        *usecase.TransferAllUseCase
}

func New(dataDir string, vaultFactory VaultRepoFactory, storageFactory StorageFactory) *App {
	kd := service.NewKeyDeriver()
	return &App{
		dataDir:          dataDir,
		keyDeriver:       kd,
		vaultRepoFactory: vaultFactory,
		storageFactory:   storageFactory,
		Unlock:           usecase.NewUnlockUseCase(kd),
		GeneratePassword: usecase.NewGeneratePasswordUseCase(),
		GenerateTOTP:     usecase.NewGenerateTOTPUseCase(),
	}
}

func (a *App) OnUnlock(ctx context.Context, password, rpcURL string) error {
	return a.OnUnlockWithTimeout(ctx, password, rpcURL, 0)
}

func (a *App) OnUnlockWithTimeout(ctx context.Context, password, rpcURL string, timeoutMinutes int) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	result, err := a.Unlock.Execute(ctx, password)
	if err != nil {
		return fmt.Errorf("derive keys: %w", err)
	}
	a.keys = result.Keys
	a.crypto = result.Crypto

	if timeoutMinutes > 0 {
		a.expiresAt = time.Now().Add(time.Duration(timeoutMinutes) * time.Minute)
	}

	dbPath := filepath.Join(a.dataDir, "vault.db")
	entries, groups, config, syncState, err := a.storageFactory(dbPath, a.crypto)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	a.entries = entries
	a.groups = groups
	a.config = config
	a.syncState = syncState

	if savedRPC, _ := config.Get(ctx, "rpc_url"); savedRPC != "" {
		rpcURL = savedRPC
	}

	a.vault = a.vaultRepoFactory(a.keys, rpcURL)

	a.AddEntry = usecase.NewAddEntryUseCase(a.entries, a.vault, a.syncState, a.crypto)
	a.UpdateEntry = usecase.NewUpdateEntryUseCase(a.entries, a.vault, a.crypto)
	a.DeleteEntry = usecase.NewDeleteEntryUseCase(a.entries, a.vault)
	a.Sync = usecase.NewSyncUseCase(a.entries, a.groups, a.vault, a.syncState, a.crypto)
	a.ListEntries = usecase.NewListEntriesUseCase(a.entries)
	a.SearchEntries = usecase.NewSearchEntriesUseCase(a.entries)
	a.GetEntry = usecase.NewGetEntryUseCase(a.entries)
	a.GetDashboard = usecase.NewGetDashboardUseCase(a.vault, a.entries, a.groups, a.config, a.syncState, a.keys)
	a.DeployProgram = usecase.NewDeployProgramUseCase(a.vault, a.keys)
	a.InitVault = usecase.NewInitVaultUseCase(a.vault)
	a.ResetVault = usecase.NewResetVaultUseCase(a.vault)
	a.CheckBalance = usecase.NewCheckBalanceUseCase(a.vault)
	a.CheckDeployStatus = usecase.NewCheckDeployStatusUseCase(a.vault)
	a.CheckVaultStatus = usecase.NewCheckVaultStatusUseCase(a.vault)
	a.ClearLocalData = usecase.NewClearLocalDataUseCase(a.entries)
	a.GetConfig = usecase.NewGetConfigUseCase(a.config)
	a.SetConfig = usecase.NewSetConfigUseCase(a.config)
	a.GetPassGenConfig = usecase.NewGetPassGenConfigUseCase(a.config)
	a.SavePassGenConfig = usecase.NewSavePassGenConfigUseCase(a.config)
	a.AddGroup = usecase.NewAddGroupUseCase(a.groups, a.vault, a.crypto)
	a.UpdateGroup = usecase.NewUpdateGroupUseCase(a.groups, a.vault, a.crypto)
	a.DeleteGroup = usecase.NewDeleteGroupUseCase(a.groups, a.entries, a.vault)
	a.PurgeGroup = usecase.NewPurgeGroupUseCase(a.groups, a.vault)
	a.ListGroups = usecase.NewListGroupsUseCase(a.groups)
	a.CloseProgram = usecase.NewCloseProgramUseCase(a.vault)
	a.TransferAll = usecase.NewTransferAllUseCase(a.vault)

	return nil
}

func (a *App) Lock() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.keys != nil {
		a.keys.Zero()
		a.keys = nil
	}
	a.expiresAt = time.Time{}
}

func (a *App) IsLocked() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.keys == nil
}

func (a *App) IsExpired() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.expiresAt.IsZero() {
		return false
	}
	return time.Now().After(a.expiresAt)
}

func (a *App) ExpiresAt() time.Time {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.expiresAt
}

func (a *App) Keys() *domain.DerivedKeys            { return a.keys }
func (a *App) Entries() domain.EntryRepository       { return a.entries }
func (a *App) Groups() domain.GroupRepository        { return a.groups }
func (a *App) Config() domain.ConfigRepository       { return a.config }
func (a *App) SyncState() domain.SyncStateRepository { return a.syncState }
func (a *App) Vault() domain.VaultRepository         { return a.vault }
func (a *App) Crypto() domain.CryptoService          { return a.crypto }

func (a *App) GetSyncStatus() SyncStatus {
	a.syncMu.RLock()
	defer a.syncMu.RUnlock()
	return a.syncStatus
}

func (a *App) StartBackgroundSync() {
	a.syncMu.Lock()
	if a.syncStatus.InProgress {
		a.syncMu.Unlock()
		return
	}
	a.mu.RLock()
	syncUC := a.Sync
	a.mu.RUnlock()
	if syncUC == nil {
		a.syncMu.Unlock()
		return
	}
	a.syncStatus = SyncStatus{InProgress: true, Phase: "Starting..."}
	a.syncMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		err := syncUC.Execute(ctx, func(p usecase.SyncProgress) {
			a.syncMu.Lock()
			a.syncStatus.Phase = p.Phase
			a.syncMu.Unlock()
		})

		a.syncMu.Lock()
		a.syncStatus.InProgress = false
		a.syncStatus.Phase = ""
		if err != nil {
			a.syncStatus.Error = err.Error()
		} else {
			a.syncStatus.Error = ""
		}
		a.syncMu.Unlock()
	}()
}

func (a *App) Shutdown() {
	a.Lock()
}
