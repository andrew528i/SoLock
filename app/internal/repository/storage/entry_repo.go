package storage

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/solock/solock/internal/domain"
)

type EntryRepo struct {
	s *SQLite
}

func (r *EntryRepo) Save(ctx context.Context, entry *domain.Entry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	encrypted, err := r.s.encrypt(data)
	if err != nil {
		return err
	}
	_, err = r.s.db.ExecContext(ctx,
		`INSERT INTO entries (id, slot_index, encrypted_data, updated_at, synced)
		 VALUES (?, ?, ?, ?, 0)
		 ON CONFLICT(id) DO UPDATE SET
			slot_index = excluded.slot_index,
			encrypted_data = excluded.encrypted_data,
			updated_at = excluded.updated_at,
			synced = 0`,
		entry.ID(), entry.SlotIndex(), encrypted, entry.UpdatedAt().Unix(),
	)
	return err
}

func (r *EntryRepo) Get(ctx context.Context, id string) (*domain.Entry, error) {
	var encrypted []byte
	err := r.s.db.QueryRowContext(ctx, "SELECT encrypted_data FROM entries WHERE id = ?", id).Scan(&encrypted)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.decryptEntry(encrypted)
}

func (r *EntryRepo) Delete(ctx context.Context, id string) error {
	_, err := r.s.db.ExecContext(ctx, "DELETE FROM entries WHERE id = ?", id)
	return err
}

func (r *EntryRepo) List(ctx context.Context) ([]*domain.Entry, error) {
	rows, err := r.s.db.QueryContext(ctx, "SELECT encrypted_data FROM entries ORDER BY slot_index ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*domain.Entry
	for rows.Next() {
		var encrypted []byte
		if err := rows.Scan(&encrypted); err != nil {
			return nil, err
		}
		entry, err := r.decryptEntry(encrypted)
		if err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (r *EntryRepo) Count(ctx context.Context) (int, error) {
	var count int
	err := r.s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM entries").Scan(&count)
	return count, err
}

func (r *EntryRepo) MarkSynced(ctx context.Context, id string) error {
	_, err := r.s.db.ExecContext(ctx, "UPDATE entries SET synced = 1 WHERE id = ?", id)
	return err
}

func (r *EntryRepo) ClearAll(ctx context.Context) error {
	_, err := r.s.db.ExecContext(ctx, "DELETE FROM entries; DELETE FROM config; DELETE FROM sync_state;")
	return err
}

func (r *EntryRepo) decryptEntry(encrypted []byte) (*domain.Entry, error) {
	decrypted, err := r.s.decrypt(encrypted)
	if err != nil {
		return nil, err
	}
	var entry domain.Entry
	if err := json.Unmarshal(decrypted, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}
