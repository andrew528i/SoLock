package storage

import (
	"context"
	"database/sql"
	"encoding/json"

	sq "github.com/Masterminds/squirrel"
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
	query, args, err := sq.Insert("entries").
		Columns("id", "slot_index", "encrypted_data", "updated_at", "synced").
		Values(entry.ID(), entry.SlotIndex(), encrypted, entry.UpdatedAt().Unix(), 0).
		Suffix("ON CONFLICT(id) DO UPDATE SET slot_index = excluded.slot_index, encrypted_data = excluded.encrypted_data, updated_at = excluded.updated_at, synced = 0").
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.s.db.ExecContext(ctx, query, args...)
	return err
}

func (r *EntryRepo) Get(ctx context.Context, id string) (*domain.Entry, error) {
	query, args, err := sq.Select("encrypted_data").
		From("entries").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return nil, err
	}
	var encrypted []byte
	err = r.s.db.QueryRowContext(ctx, query, args...).Scan(&encrypted)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return r.decryptEntry(encrypted)
}

func (r *EntryRepo) Delete(ctx context.Context, id string) error {
	query, args, err := sq.Delete("entries").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.s.db.ExecContext(ctx, query, args...)
	return err
}

func (r *EntryRepo) List(ctx context.Context) ([]*domain.Entry, error) {
	query, args, err := sq.Select("encrypted_data").
		From("entries").
		OrderBy("slot_index ASC").
		ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := r.s.db.QueryContext(ctx, query, args...)
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
	query, args, err := sq.Select("COUNT(*)").
		From("entries").
		ToSql()
	if err != nil {
		return 0, err
	}
	var count int
	err = r.s.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

func (r *EntryRepo) MarkSynced(ctx context.Context, id string) error {
	query, args, err := sq.Update("entries").
		Set("synced", 1).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.s.db.ExecContext(ctx, query, args...)
	return err
}

func (r *EntryRepo) ClearAll(ctx context.Context) error {
	for _, table := range []string{"entries", "config", "sync_state"} {
		query, args, err := sq.Delete(table).ToSql()
		if err != nil {
			return err
		}
		if _, err := r.s.db.ExecContext(ctx, query, args...); err != nil {
			return err
		}
	}
	return nil
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
