package storage

import (
	"context"
	"database/sql"
	"encoding/json"

	sq "github.com/Masterminds/squirrel"
	"github.com/solock/solock/internal/domain"
)

type SyncStateRepo struct {
	s *SQLite
}

func (r *SyncStateRepo) Get(ctx context.Context) (*domain.SyncState, error) {
	query, args, err := sq.Select("value").
		From("sync_state").
		Where(sq.Eq{"key": "state"}).
		ToSql()
	if err != nil {
		return nil, err
	}
	var encrypted []byte
	err = r.s.db.QueryRowContext(ctx, query, args...).Scan(&encrypted)
	if err == sql.ErrNoRows {
		return &domain.SyncState{}, nil
	}
	if err != nil {
		return nil, err
	}
	decrypted, err := r.s.decrypt(encrypted)
	if err != nil {
		return nil, err
	}
	var state domain.SyncState
	if err := json.Unmarshal(decrypted, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func (r *SyncStateRepo) Set(ctx context.Context, state *domain.SyncState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	encrypted, err := r.s.encrypt(data)
	if err != nil {
		return err
	}
	query, args, err := sq.Insert("sync_state").
		Columns("key", "value").
		Values("state", encrypted).
		Suffix("ON CONFLICT(key) DO UPDATE SET value = excluded.value").
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.s.db.ExecContext(ctx, query, args...)
	return err
}
