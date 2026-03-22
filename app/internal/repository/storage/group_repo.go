package storage

import (
	"context"
	"database/sql"
	"encoding/json"

	sq "github.com/Masterminds/squirrel"
	"github.com/solock/solock/internal/domain"
)

type GroupRepo struct {
	s *SQLite
}

func (r *GroupRepo) Save(ctx context.Context, group *domain.Group) error {
	data, err := json.Marshal(group)
	if err != nil {
		return err
	}
	encrypted, err := r.s.encrypt(data)
	if err != nil {
		return err
	}
	deleted := 0
	if group.Deleted() {
		deleted = 1
	}
	query, args, err := sq.Insert("groups").
		Columns("idx", "encrypted_data", "deleted").
		Values(group.Index(), encrypted, deleted).
		Suffix("ON CONFLICT(idx) DO UPDATE SET encrypted_data = excluded.encrypted_data, deleted = excluded.deleted").
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.s.db.ExecContext(ctx, query, args...)
	return err
}

func (r *GroupRepo) Get(ctx context.Context, index uint32) (*domain.Group, error) {
	query, args, err := sq.Select("encrypted_data", "deleted").
		From("groups").
		Where(sq.Eq{"idx": index}).
		ToSql()
	if err != nil {
		return nil, err
	}
	var encrypted []byte
	var deleted int
	err = r.s.db.QueryRowContext(ctx, query, args...).Scan(&encrypted, &deleted)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	group, err := r.decryptGroup(encrypted)
	if err != nil {
		return nil, err
	}
	group.SetIndex(index)
	group.SetDeleted(deleted != 0)
	return group, nil
}

func (r *GroupRepo) Delete(ctx context.Context, index uint32) error {
	query, args, err := sq.Delete("groups").
		Where(sq.Eq{"idx": index}).
		ToSql()
	if err != nil {
		return err
	}
	_, err = r.s.db.ExecContext(ctx, query, args...)
	return err
}

func (r *GroupRepo) List(ctx context.Context) ([]*domain.Group, error) {
	query, args, err := sq.Select("idx", "encrypted_data", "deleted").
		From("groups").
		OrderBy("idx ASC").
		ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := r.s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []*domain.Group
	for rows.Next() {
		var idx uint32
		var encrypted []byte
		var deleted int
		if err := rows.Scan(&idx, &encrypted, &deleted); err != nil {
			return nil, err
		}
		group, err := r.decryptGroup(encrypted)
		if err != nil {
			continue
		}
		group.SetIndex(idx)
		group.SetDeleted(deleted != 0)
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func (r *GroupRepo) ClearAll(ctx context.Context) error {
	query, args, err := sq.Delete("groups").ToSql()
	if err != nil {
		return err
	}
	_, err = r.s.db.ExecContext(ctx, query, args...)
	return err
}

func (r *GroupRepo) decryptGroup(encrypted []byte) (*domain.Group, error) {
	decrypted, err := r.s.decrypt(encrypted)
	if err != nil {
		return nil, err
	}
	var group domain.Group
	if err := json.Unmarshal(decrypted, &group); err != nil {
		return nil, err
	}
	return &group, nil
}
