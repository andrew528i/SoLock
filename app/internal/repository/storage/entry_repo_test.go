package storage

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

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

// legacyV1EncryptEntry reproduces the exact pre-v2 write path (gzip +
// AES-256-CBC + PKCS7) so we can plant a "legacy" blob straight into SQLite
// and verify that the new read path (which uses v2 GCM with v1 fallback)
// recovers it without touching production write code.
func legacyV1EncryptEntry(t *testing.T, entry *domain.Entry, key []byte) []byte {
	t.Helper()
	plaintext, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(plaintext); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	compressed := buf.Bytes()

	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("aes: %v", err)
	}
	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		t.Fatalf("rand: %v", err)
	}
	padding := aes.BlockSize - (len(compressed) % aes.BlockSize)
	padded := append(compressed, bytes.Repeat([]byte{byte(padding)}, padding)...)
	ct := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ct, padded)

	return append(iv, ct...)
}

func TestEntryRepoReadsLegacyV1Blob(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	crypto := service.NewCryptoService(key)
	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	s, err := Open(dbPath, crypto)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	repo := &EntryRepo{s: s}

	// Build an entry exactly like the old code would have serialised it
	// and insert the resulting v1 blob directly via raw SQL - bypassing
	// repo.Save so the bytes on disk are guaranteed to be the legacy format.
	entry, err := domain.NewEntry("legacy-1", domain.EntryTypePassword, "Legacy GitHub", map[string]string{
		"username": "olduser",
		"password": "oldpass-very-secret-123",
		"site":     "github.com",
	})
	if err != nil {
		t.Fatalf("new entry: %v", err)
	}
	entry.SetSlotIndex(7)

	legacyBlob := legacyV1EncryptEntry(t, entry, key)

	// Sanity: the blob really is the v1 shape (multiple of 16, >= 32 bytes).
	if len(legacyBlob) < 32 || len(legacyBlob)%16 != 0 {
		t.Fatalf("legacy blob does not look v1: len=%d", len(legacyBlob))
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO entries (id, slot_index, encrypted_data, updated_at, synced, accessed_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, entry.ID(), entry.SlotIndex(), legacyBlob, time.Now().Unix(), 1, 0)
	if err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}

	// Now read via the production path - this exercises crypto.Decrypt,
	// which must detect v1 and fall through aesCBCDecrypt + gzipDecompress.
	got, err := repo.Get(ctx, "legacy-1")
	if err != nil {
		t.Fatalf("repo.Get of v1 blob failed: %v", err)
	}
	if got == nil {
		t.Fatal("repo.Get returned nil for existing legacy entry")
	}
	if got.Name() != "Legacy GitHub" {
		t.Fatalf("name mismatch: got %q", got.Name())
	}
	if got.Field("username") != "olduser" {
		t.Fatalf("username mismatch: got %q", got.Field("username"))
	}
	if got.Field("password") != "oldpass-very-secret-123" {
		t.Fatalf("password mismatch")
	}
	if got.Field("site") != "github.com" {
		t.Fatalf("site mismatch: got %q", got.Field("site"))
	}

	// List must also surface it.
	all, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 1 || all[0].ID() != "legacy-1" {
		t.Fatalf("list mismatch: %+v", all)
	}
}

func TestEntryRepoMixedV1AndV2Blobs(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	crypto := service.NewCryptoService(key)
	dbPath := filepath.Join(t.TempDir(), "mixed.db")
	s, err := Open(dbPath, crypto)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	repo := &EntryRepo{s: s}

	// Plant a v1 legacy entry via raw SQL.
	v1Entry, _ := domain.NewEntry("v1-id", domain.EntryTypePassword, "V1 Item", map[string]string{
		"username": "u1", "password": "p1",
	})
	v1Entry.SetSlotIndex(1)
	v1Blob := legacyV1EncryptEntry(t, v1Entry, key)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO entries (id, slot_index, encrypted_data, updated_at, synced, accessed_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, v1Entry.ID(), v1Entry.SlotIndex(), v1Blob, time.Now().Unix(), 1, 0)
	if err != nil {
		t.Fatalf("insert v1: %v", err)
	}

	// Write a v2 entry the normal way.
	v2Entry, _ := domain.NewEntry("v2-id", domain.EntryTypePassword, "V2 Item", map[string]string{
		"username": "u2", "password": "p2",
	})
	v2Entry.SetSlotIndex(2)
	if err := repo.Save(ctx, v2Entry); err != nil {
		t.Fatalf("save v2: %v", err)
	}

	got1, err := repo.Get(ctx, "v1-id")
	if err != nil || got1 == nil || got1.Field("password") != "p1" {
		t.Fatalf("v1 read failed: err=%v got=%+v", err, got1)
	}
	got2, err := repo.Get(ctx, "v2-id")
	if err != nil || got2 == nil || got2.Field("password") != "p2" {
		t.Fatalf("v2 read failed: err=%v got=%+v", err, got2)
	}

	all, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(all))
	}

	// Simulate "update" of the v1 entry: repo.Save must re-encrypt it in v2.
	// After this, the on-disk blob should be readable both by the current code
	// and look like a v2 blob (first byte 0x02).
	v1Entry.SetField("password", "p1-updated")
	if err := repo.Save(ctx, v1Entry); err != nil {
		t.Fatalf("re-save v1 entry: %v", err)
	}

	var rawBlob []byte
	err = s.db.QueryRowContext(ctx, `SELECT encrypted_data FROM entries WHERE id = ?`, "v1-id").Scan(&rawBlob)
	if err != nil {
		t.Fatalf("read raw blob: %v", err)
	}
	if len(rawBlob) == 0 || rawBlob[0] != 0x02 {
		t.Fatalf("expected v2 blob after re-save, got first byte %#x", rawBlob[0])
	}

	got1After, err := repo.Get(ctx, "v1-id")
	if err != nil || got1After == nil || got1After.Field("password") != "p1-updated" {
		t.Fatalf("post-migration read failed: err=%v got=%+v", err, got1After)
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
