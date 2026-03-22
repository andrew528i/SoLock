package domain

import (
	"encoding/json"
	"errors"
	"time"
	"unicode/utf8"
)

const MaxGroupNameLength = 64

type Group struct {
	index     uint32
	name      string
	deleted   bool
	createdAt time.Time
	updatedAt time.Time
}

func NewGroup(index uint32, name string) (*Group, error) {
	if err := ValidateGroupName(name); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	return &Group{
		index:     index,
		name:      name,
		createdAt: now,
		updatedAt: now,
	}, nil
}

func (g *Group) Index() uint32       { return g.index }
func (g *Group) Name() string        { return g.name }
func (g *Group) Deleted() bool       { return g.deleted }
func (g *Group) CreatedAt() time.Time { return g.createdAt }
func (g *Group) UpdatedAt() time.Time { return g.updatedAt }

func (g *Group) SetName(name string) {
	g.name = name
	g.updatedAt = time.Now().UTC()
}

func (g *Group) SetDeleted(deleted bool) {
	g.deleted = deleted
}

func (g *Group) SetIndex(index uint32) {
	g.index = index
}

func ValidateGroupName(name string) error {
	if name == "" {
		return errors.New("group name is required")
	}
	if utf8.RuneCountInString(name) > MaxGroupNameLength {
		return errors.New("group name exceeds 64 characters")
	}
	return nil
}

type groupJSON struct {
	Name      string `json:"name"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

func (g *Group) MarshalJSON() ([]byte, error) {
	return json.Marshal(groupJSON{
		Name:      g.name,
		CreatedAt: g.createdAt.Unix(),
		UpdatedAt: g.updatedAt.Unix(),
	})
}

func (g *Group) UnmarshalJSON(data []byte) error {
	var j groupJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	g.name = j.Name
	if j.CreatedAt > 0 {
		g.createdAt = time.Unix(j.CreatedAt, 0).UTC()
	}
	if j.UpdatedAt > 0 {
		g.updatedAt = time.Unix(j.UpdatedAt, 0).UTC()
	}
	return nil
}
