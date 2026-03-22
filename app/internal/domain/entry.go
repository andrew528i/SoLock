package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type Entry struct {
	id         string
	slotIndex  uint32
	entryType  EntryType
	name       string
	fields     map[string]string
	groupIndex *uint32
	createdAt  time.Time
	updatedAt  time.Time
}

func NewEntry(id string, entryType EntryType, name string, fields map[string]string) (*Entry, error) {
	if id == "" {
		return nil, errors.New("entry id is required")
	}
	if name == "" {
		return nil, errors.New("entry name is required")
	}
	if !entryType.IsValid() {
		return nil, fmt.Errorf("invalid entry type: %s", entryType)
	}

	now := time.Now().UTC()
	e := &Entry{
		id:        id,
		entryType: entryType,
		name:      name,
		fields:    make(map[string]string),
		createdAt: now,
		updatedAt: now,
	}

	for k, v := range fields {
		e.fields[k] = v
	}

	if err := e.Validate(); err != nil {
		return nil, err
	}

	return e, nil
}

func (e *Entry) ID() string            { return e.id }
func (e *Entry) SlotIndex() uint32     { return e.slotIndex }
func (e *Entry) Type() EntryType       { return e.entryType }
func (e *Entry) Name() string          { return e.name }
func (e *Entry) GroupIndex() *uint32   { return e.groupIndex }
func (e *Entry) CreatedAt() time.Time  { return e.createdAt }
func (e *Entry) UpdatedAt() time.Time  { return e.updatedAt }
func (e *Entry) Schema() *EntrySchema  { return SchemaFor(e.entryType) }

func (e *Entry) Field(key string) string {
	return e.fields[key]
}

func (e *Entry) Fields() map[string]string {
	cp := make(map[string]string, len(e.fields))
	for k, v := range e.fields {
		cp[k] = v
	}
	return cp
}

func (e *Entry) SetSlotIndex(idx uint32) {
	e.slotIndex = idx
}

func (e *Entry) SetGroupIndex(idx *uint32) {
	e.groupIndex = idx
	e.updatedAt = time.Now().UTC()
}

func (e *Entry) SetOnChainTimestamps(createdAt, updatedAt int64) {
	if createdAt > 0 {
		e.createdAt = time.Unix(createdAt, 0).UTC()
	}
	if updatedAt > 0 {
		e.updatedAt = time.Unix(updatedAt, 0).UTC()
	}
}

func (e *Entry) SetName(name string) {
	e.name = name
	e.updatedAt = time.Now().UTC()
}

func (e *Entry) SetField(key, value string) {
	e.fields[key] = value
	e.updatedAt = time.Now().UTC()
}

func (e *Entry) SetFields(fields map[string]string) {
	e.fields = make(map[string]string, len(fields))
	for k, v := range fields {
		e.fields[k] = v
	}
	e.updatedAt = time.Now().UTC()
}

func (e *Entry) Touch() {
	e.updatedAt = time.Now().UTC()
}

func (e *Entry) TOTPSecret() string {
	schema := e.Schema()
	if schema == nil {
		return ""
	}
	key := schema.TOTPKey()
	if key == "" {
		return ""
	}
	return e.fields[key]
}

func (e *Entry) HasTOTP() bool {
	return e.TOTPSecret() != ""
}

func (e *Entry) CopyableSecret() string {
	switch e.entryType {
	case EntryTypePassword:
		return e.fields["password"]
	case EntryTypeCard:
		return e.fields["number"]
	case EntryTypeNote:
		return e.fields["content"]
	case EntryTypeTOTP:
		return e.fields["secret"]
	default:
		return ""
	}
}

func (e *Entry) Validate() error {
	if e.name == "" {
		return errors.New("entry name is required")
	}
	if !e.entryType.IsValid() {
		return fmt.Errorf("invalid entry type: %s", e.entryType)
	}
	schema := e.Schema()
	if schema == nil {
		return fmt.Errorf("no schema for type: %s", e.entryType)
	}
	return schema.ValidateFields(e.fields)
}

type entryJSON struct {
	ID         string            `json:"id"`
	SlotIndex  uint32            `json:"slot_index"`
	Type       EntryType         `json:"type"`
	Name       string            `json:"name"`
	Fields     map[string]string `json:"fields"`
	GroupIndex *uint32           `json:"group_index,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

func (e *Entry) MarshalJSON() ([]byte, error) {
	return json.Marshal(entryJSON{
		ID:         e.id,
		SlotIndex:  e.slotIndex,
		Type:       e.entryType,
		Name:       e.name,
		Fields:     e.fields,
		GroupIndex: e.groupIndex,
		CreatedAt:  e.createdAt,
		UpdatedAt:  e.updatedAt,
	})
}

func (e *Entry) UnmarshalJSON(data []byte) error {
	var j entryJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}
	e.id = j.ID
	e.slotIndex = j.SlotIndex
	e.entryType = j.Type
	e.name = j.Name
	e.fields = j.Fields
	e.groupIndex = j.GroupIndex
	e.createdAt = j.CreatedAt
	e.updatedAt = j.UpdatedAt
	if e.fields == nil {
		e.fields = make(map[string]string)
	}
	return nil
}
