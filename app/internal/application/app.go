package application

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/solock/solock/internal/domain"
	"github.com/solock/solock/internal/domain/service"
	"github.com/solock/solock/internal/usecase"
)

type VaultRepoFactory func(keys *domain.DerivedKeys, rpcURL string) domain.VaultRepository
type StorageFactory func(dbPath string, crypto domain.CryptoService) (domain.EntryRepository, domain.ConfigRepository, domain.SyncStateRepository, error)

type App struct {
	dataDir          string
	keyDeriver       domain.KeyDeriver
	vaultRepoFactory VaultRepoFactory
	storageFactory   StorageFactory

	keys      *domain.DerivedKeys
	crypto    domain.CryptoService
	entries   domain.EntryRepository
	config    domain.ConfigRepository
	syncState domain.SyncStateRepository
	vault     domain.VaultRepository

	Unlock      *usecase.UnlockUseCase
	AddEntry    *usecase.AddEntryUseCase
	UpdateEntry *usecase.UpdateEntryUseCase
	DeleteEntry *usecase.DeleteEntryUseCase
	Sync        *usecase.SyncUseCase
}

func New(dataDir string, vaultFactory VaultRepoFactory, storageFactory StorageFactory) *App {
	kd := service.NewKeyDeriver()
	return &App{
		dataDir:          dataDir,
		keyDeriver:       kd,
		vaultRepoFactory: vaultFactory,
		storageFactory:   storageFactory,
		Unlock:           usecase.NewUnlockUseCase(kd),
	}
}

func (a *App) OnUnlock(ctx context.Context, password, rpcURL string) error {
	result, err := a.Unlock.Execute(ctx, password)
	if err != nil {
		return fmt.Errorf("derive keys: %w", err)
	}
	a.keys = result.Keys
	a.crypto = result.Crypto

	dbPath := filepath.Join(a.dataDir, "vault.db")
	entries, config, syncState, err := a.storageFactory(dbPath, a.crypto)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	a.entries = entries
	a.config = config
	a.syncState = syncState

	if savedRPC, _ := config.Get(ctx, "rpc_url"); savedRPC != "" {
		rpcURL = savedRPC
	}

	a.vault = a.vaultRepoFactory(a.keys, rpcURL)

	a.AddEntry = usecase.NewAddEntryUseCase(a.entries, a.vault, a.syncState, a.crypto)
	a.UpdateEntry = usecase.NewUpdateEntryUseCase(a.entries, a.vault, a.crypto)
	a.DeleteEntry = usecase.NewDeleteEntryUseCase(a.entries, a.vault)
	a.Sync = usecase.NewSyncUseCase(a.entries, a.vault, a.syncState, a.crypto)

	return nil
}

func (a *App) Keys() *domain.DerivedKeys            { return a.keys }
func (a *App) Entries() domain.EntryRepository       { return a.entries }
func (a *App) Config() domain.ConfigRepository       { return a.config }
func (a *App) SyncState() domain.SyncStateRepository { return a.syncState }
func (a *App) Vault() domain.VaultRepository         { return a.vault }
func (a *App) Crypto() domain.CryptoService          { return a.crypto }

func (a *App) Shutdown() {
	if a.keys != nil {
		a.keys.Zero()
		a.keys = nil
	}
}
