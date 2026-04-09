package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	"github.com/solock/solock/internal/domain"
)

type SQLite struct {
	db     *sql.DB
	crypto domain.CryptoService
}

func Open(dbPath string, crypto domain.CryptoService) (*SQLite, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create dir: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}

	s := &SQLite{db: db, crypto: crypto}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

func (s *SQLite) Close() error {
	return s.db.Close()
}

func (s *SQLite) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS config (
			key TEXT PRIMARY KEY,
			value BLOB NOT NULL
		);
		CREATE TABLE IF NOT EXISTS entries (
			id TEXT PRIMARY KEY,
			slot_index INTEGER NOT NULL,
			encrypted_data BLOB NOT NULL,
			updated_at INTEGER NOT NULL,
			synced INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE IF NOT EXISTS groups (
			idx INTEGER PRIMARY KEY,
			encrypted_data BLOB NOT NULL,
			deleted INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE IF NOT EXISTS sync_state (
			key TEXT PRIMARY KEY,
			value BLOB NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_entries_slot ON entries(slot_index);
	`)
	if err != nil {
		return err
	}

	var cnt int
	err = s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('entries') WHERE name = 'accessed_at'`).Scan(&cnt)
	if err != nil {
		return err
	}
	if cnt == 0 {
		_, err = s.db.Exec(`ALTER TABLE entries ADD COLUMN accessed_at INTEGER NOT NULL DEFAULT 0`)
	}
	return err
}

func (s *SQLite) encrypt(data []byte) ([]byte, error) {
	return s.crypto.Encrypt(data)
}

func (s *SQLite) decrypt(data []byte) ([]byte, error) {
	return s.crypto.Decrypt(data)
}

func NewRepositories(dbPath string, crypto domain.CryptoService) (domain.EntryRepository, domain.GroupRepository, domain.ConfigRepository, domain.SyncStateRepository, error) {
	s, err := Open(dbPath, crypto)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return &EntryRepo{s: s}, &GroupRepo{s: s}, &ConfigRepo{s: s}, &SyncStateRepo{s: s}, nil
}
