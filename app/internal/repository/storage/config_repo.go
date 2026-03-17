package storage

import (
	"context"
	"database/sql"
)

type ConfigRepo struct {
	s *SQLite
}

func (r *ConfigRepo) Set(ctx context.Context, key, value string) error {
	encrypted, err := r.s.encrypt([]byte(value))
	if err != nil {
		return err
	}
	_, err = r.s.db.ExecContext(ctx,
		"INSERT INTO config (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, encrypted,
	)
	return err
}

func (r *ConfigRepo) Get(ctx context.Context, key string) (string, error) {
	var encrypted []byte
	err := r.s.db.QueryRowContext(ctx, "SELECT value FROM config WHERE key = ?", key).Scan(&encrypted)
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
