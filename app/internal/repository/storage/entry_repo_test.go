package storage

import (
	"context"
	"crypto/rand"
	"path/filepath"
	"testing"

	"github.com/solock/solock/internal/domain"
	"github.com/solock/solock/internal/domain/service"
)

func testRepos(t *testing.T) (*EntryRepo, *ConfigRepo, *SyncStateRepo) {
	t.Helper()
	key := make([]byte, 32)
	rand.Read(key)
	crypto := service.NewCryptoService(key)
	s, err := Open(filepath.Join(t.TempDir(), "test.db"), crypto)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return &EntryRepo{s: s}, &ConfigRepo{s: s}, &SyncStateRepo{s: s}
}

func TestEntrySaveAndGet(t *testing.T) {
	repo, _, _ := testRepos(t)
	ctx := context.Background()

	e, _ := domain.NewEntry("e1", domain.EntryTypePassword, "GitHub", map[string]string{
		"username": "user",
		"password": "pass",
	})

	if err := repo.Save(ctx, e); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := repo.Get(ctx, "e1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil || got.Name() != "GitHub" {
		t.Fatal("entry mismatch")
	}
}

func TestEntryListAndCount(t *testing.T) {
	repo, _, _ := testRepos(t)
	ctx := context.Background()

	for i := range 3 {
		e, _ := domain.NewEntry(
			string(rune('a'+i)),
			domain.EntryTypeNote,
			"Note",
			map[string]string{"content": "text"},
		)
		e.SetSlotIndex(uint32(i))
		repo.Save(ctx, e)
	}

	entries, _ := repo.List(ctx)
	if len(entries) != 3 {
		t.Fatalf("expected 3, got %d", len(entries))
	}

	count, _ := repo.Count(ctx)
	if count != 3 {
		t.Fatalf("expected count 3, got %d", count)
	}
}

func TestEntryDelete(t *testing.T) {
	repo, _, _ := testRepos(t)
	ctx := context.Background()

	e, _ := domain.NewEntry("del1", domain.EntryTypeNote, "Note", map[string]string{"content": "x"})
	repo.Save(ctx, e)
	repo.Delete(ctx, "del1")

	got, _ := repo.Get(ctx, "del1")
	if got != nil {
		t.Fatal("should be nil after delete")
	}
}

func TestConfigSetGet(t *testing.T) {
	_, cfg, _ := testRepos(t)
	ctx := context.Background()

	cfg.Set(ctx, "network", "devnet")
	val, _ := cfg.Get(ctx, "network")
	if val != "devnet" {
		t.Fatalf("expected 'devnet', got %q", val)
	}
}

func TestSyncState(t *testing.T) {
	_, _, ss := testRepos(t)
	ctx := context.Background()

	state, _ := ss.Get(ctx)
	if state.NextIndex != 0 {
		t.Fatal("initial next_index should be 0")
	}

	state.NextIndex = 10
	state.EntryCount = 5
	ss.Set(ctx, state)

	got, _ := ss.Get(ctx)
	if got.NextIndex != 10 || got.EntryCount != 5 {
		t.Fatalf("state mismatch: %+v", got)
	}
}

func TestDataEncryptedOnDisk(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	crypto := service.NewCryptoService(key)
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, _ := Open(dbPath, crypto)
	repo := &EntryRepo{s: s}
	ctx := context.Background()

	e, _ := domain.NewEntry("enc1", domain.EntryTypePassword, "Secret Site", map[string]string{
		"username": "admin",
		"password": "supersecret",
	})
	repo.Save(ctx, e)
	s.Close()

	raw, _ := filepath.Abs(dbPath)
	data, _ := filepath.Abs(raw)
	_ = data

	wrongKey := make([]byte, 32)
	rand.Read(wrongKey)
	wrongCrypto := service.NewCryptoService(wrongKey)
	s2, _ := Open(dbPath, wrongCrypto)
	defer s2.Close()
	repo2 := &EntryRepo{s: s2}

	got, err := repo2.Get(ctx, "enc1")
	if err == nil && got != nil && got.Name() == "Secret Site" {
		t.Fatal("should not decrypt with wrong key")
	}
}
