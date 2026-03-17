package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/solock/solock/internal/api/tui"
	"github.com/solock/solock/internal/application"
	"github.com/solock/solock/internal/domain"
	"github.com/solock/solock/internal/repository/adapter"
	"github.com/solock/solock/internal/repository/storage"
)

func main() {
	devNull, err := os.Open(os.DevNull)
	if err == nil {
		os.Stderr = devNull
	}

	dataDir, err := resolveDataDir()
	if err != nil {
		fmt.Fprintf(os.Stdout, "error: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		fmt.Fprintf(os.Stdout, "error: %v\n", err)
		os.Exit(1)
	}

	vaultFactory := func(keys *domain.DerivedKeys, rpcURL string) domain.VaultRepository {
		return adapter.NewSolanaVaultRepo(keys, rpcURL)
	}
	storageFactory := func(dbPath string, crypto domain.CryptoService) (domain.EntryRepository, domain.ConfigRepository, domain.SyncStateRepository, error) {
		return storage.NewRepositories(dbPath, crypto)
	}

	app := application.New(dataDir, vaultFactory, storageFactory)
	tuiApp := tui.NewTUI(app)

	p := tea.NewProgram(tuiApp, tea.WithAltScreen())
	tuiApp.SetProgram(p)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stdout, "error: %v\n", err)
		os.Exit(1)
	}
}

func resolveDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".solock"), nil
}
