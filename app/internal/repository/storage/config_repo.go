package storage

import (
	"context"
	"database/sql"

	sq "github.com/Masterminds/squirrel"
)

type ConfigRepo struct {
	s *SQLite
}

func (r *ConfigRepo) Set(ctx context.Context, key, value string) error {
	encrypted, err := r.s.encrypt([]byte(value))
	if err != nil {
		return err
	}
	query, args, err := sq.Insert("config").
		Columns("key", "value").
		Values(key, encrypted).
		Suffix("ON CONFLICT(key) DO UPDATE SET value = excluded.value").
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.s.db.ExecContext(ctx, query, args...)
	return err
}

func (r *ConfigRepo) Get(ctx context.Context, key string) (string, error) {
	query, args, err := sq.Select("value").
		From("config").
		Where(sq.Eq{"key": key}).
		ToSql()
	if err != nil {
		return "", err
	}
	var encrypted []byte
	err = r.s.db.QueryRowContext(ctx, query, args...).Scan(&encrypted)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	decrypted, err := r.s.decrypt(encrypted)
	if err != nil {
		return "", err
	}
	return string(decrypted), nil
}
