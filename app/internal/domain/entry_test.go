package domain

import (
	"encoding/json"
	"testing"
)

func TestNewEntry(t *testing.T) {
	e, err := NewEntry("id-1", EntryTypePassword, "GitHub", map[string]string{
		"username": "user@example.com",
		"password": "secret",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.ID() != "id-1" {
		t.Errorf("expected id 'id-1', got %q", e.ID())
	}
	if e.Name() != "GitHub" {
		t.Errorf("expected name 'GitHub', got %q", e.Name())
	}
	if e.Field("username") != "user@example.com" {
		t.Errorf("username mismatch")
	}
}

func TestNewEntryValidation(t *testing.T) {
	_, err := NewEntry("", EntryTypePassword, "Test", nil)
	if err == nil {
		t.Error("expected error for empty id")
	}

	_, err = NewEntry("id", EntryTypePassword, "", nil)
	if err == nil {
		t.Error("expected error for empty name")
	}

	_, err = NewEntry("id", EntryType("unknown"), "Test", nil)
	if err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestEntryRequiredFields(t *testing.T) {
	_, err := NewEntry("id", EntryTypeNote, "Note", map[string]string{})
	if err == nil {
		t.Error("expected error: note requires content")
	}

	_, err = NewEntry("id", EntryTypeNote, "Note", map[string]string{"content": "text"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEntryTOTP(t *testing.T) {
	e, _ := NewEntry("id", EntryTypePassword, "AWS", map[string]string{
		"username":    "admin",
		"password":    "pass",
		"totp_secret": "JBSWY3DPEHPK3PXP",
	})
	if !e.HasTOTP() {
		t.Error("should have TOTP")
	}
	if e.TOTPSecret() != "JBSWY3DPEHPK3PXP" {
		t.Errorf("TOTP secret mismatch")
	}

	e2, _ := NewEntry("id2", EntryTypePassword, "Simple", map[string]string{
		"username": "user",
		"password": "pass",
	})
	if e2.HasTOTP() {
		t.Error("should not have TOTP")
	}
}

func TestEntryJSON(t *testing.T) {
	e, _ := NewEntry("id-1", EntryTypePassword, "Test", map[string]string{
		"username": "user",
		"password": "pass",
	})
	e.SetSlotIndex(5)

	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var restored Entry
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if restored.ID() != "id-1" {
		t.Errorf("ID mismatch: %q", restored.ID())
	}
	if restored.SlotIndex() != 5 {
		t.Errorf("SlotIndex mismatch: %d", restored.SlotIndex())
	}
	if restored.Field("username") != "user" {
		t.Errorf("field mismatch")
	}
}

func TestEntryFieldsCopy(t *testing.T) {
	e, _ := NewEntry("id", EntryTypePassword, "Test", map[string]string{
		"username": "user",
	})

	fields := e.Fields()
	fields["username"] = "hacked"

	if e.Field("username") != "user" {
		t.Error("Fields() should return a copy, not reference")
	}
}

func TestEntryCopyableSecret(t *testing.T) {
	e, _ := NewEntry("id", EntryTypePassword, "Test", map[string]string{
		"username": "u",
		"password": "secret123",
	})
	if e.CopyableSecret() != "secret123" {
		t.Errorf("expected 'secret123', got %q", e.CopyableSecret())
	}
}

func TestEntrySchemaFor(t *testing.T) {
	s := SchemaFor(EntryTypePassword)
	if s == nil {
		t.Fatal("schema should not be nil")
	}
	if s.TOTPKey() != "totp_secret" {
		t.Errorf("expected TOTP key 'totp_secret', got %q", s.TOTPKey())
	}

	s2 := SchemaFor(EntryTypeNote)
	if s2.TOTPKey() != "" {
		t.Error("note should not have TOTP key")
	}
}
