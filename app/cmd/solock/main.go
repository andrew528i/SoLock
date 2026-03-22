package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/solock/solock/internal/application"
	"github.com/solock/solock/internal/domain"
	"github.com/solock/solock/internal/repository/adapter"
	"github.com/solock/solock/internal/repository/storage"
	"github.com/solock/solock/internal/transport/jsonrpc"
	"github.com/solock/solock/internal/transport/tui"
)

func main() {
	dataDir, err := resolveDataDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) > 1 && os.Args[1] == "serve" {
		runServe(dataDir)
		return
	}

	runTUI(dataDir)
}

func runTUI(dataDir string) {
	devNull, err := os.Open(os.DevNull)
	if err == nil {
		os.Stderr = devNull
	}

	app := newApp(dataDir)
	tuiApp := tui.NewTUI(app)

	p := tea.NewProgram(tuiApp, tea.WithAltScreen())
	tuiApp.SetProgram(p)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stdout, "error: %v\n", err)
		os.Exit(1)
	}
}

func runServe(dataDir string) {
	app := newApp(dataDir)

	srv, err := jsonrpc.NewServer(app, dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stdout, "%s\n%s\n", srv.SocketPath(), srv.Token())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		srv.Stop()
	}()

	if err := srv.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func newApp(dataDir string) *application.App {
	vaultFactory := func(keys *domain.DerivedKeys, rpcURL string) domain.VaultRepository {
		return adapter.NewSolanaVaultRepo(keys, rpcURL)
	}
	storageFactory := func(dbPath string, crypto domain.CryptoService) (domain.EntryRepository, domain.GroupRepository, domain.ConfigRepository, domain.SyncStateRepository, error) {
		return storage.NewRepositories(dbPath, crypto)
	}
	return application.New(dataDir, vaultFactory, storageFactory)
}

func resolveDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".solock"), nil
}
