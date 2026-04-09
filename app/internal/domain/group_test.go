package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewGroup(t *testing.T) {
	g, err := NewGroup(0, "Work")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.Index() != 0 {
		t.Errorf("expected index 0, got %d", g.Index())
	}
	if g.Name() != "Work" {
		t.Errorf("expected name 'Work', got %q", g.Name())
	}
	if g.Deleted() {
		t.Error("new group should not be deleted")
	}
	if g.CreatedAt().IsZero() {
		t.Error("createdAt should not be zero")
	}
	if g.UpdatedAt().IsZero() {
		t.Error("updatedAt should not be zero")
	}
}

func TestNewGroupValidation(t *testing.T) {
	_, err := NewGroup(0, "")
	if err == nil {
		t.Error("expected error for empty name")
	}

	longName := strings.Repeat("a", MaxGroupNameLength+1)
	_, err = NewGroup(0, longName)
	if err == nil {
		t.Error("expected error for name exceeding max length")
	}

	exactName := strings.Repeat("b", MaxGroupNameLength)
	g, err := NewGroup(0, exactName)
	if err != nil {
		t.Fatalf("unexpected error for name at max length: %v", err)
	}
	if g.Name() != exactName {
		t.Error("name mismatch at max length")
	}
}

func TestGroupValidateUnicode(t *testing.T) {
	unicodeName := strings.Repeat("\U0001f512", MaxGroupNameLength)
	g, err := NewGroup(0, unicodeName)
	if err != nil {
		t.Fatalf("unexpected error for unicode name: %v", err)
	}
	if g.Name() != unicodeName {
		t.Error("unicode name mismatch")
	}

	tooLong := strings.Repeat("\U0001f512", MaxGroupNameLength+1)
	_, err = NewGroup(0, tooLong)
	if err == nil {
		t.Error("expected error for unicode name exceeding max rune count")
	}
}

func TestGroupSetName(t *testing.T) {
	g, _ := NewGroup(0, "Work")
	before := g.UpdatedAt()

	g.SetName("Personal")
	if g.Name() != "Personal" {
		t.Errorf("expected 'Personal', got %q", g.Name())
	}
	if !g.UpdatedAt().After(before) && g.UpdatedAt() != before {
		t.Error("updatedAt should be updated after SetName")
	}
}

func TestGroupSetDeleted(t *testing.T) {
	g, _ := NewGroup(0, "Work")
	g.SetDeleted(true)
	if !g.Deleted() {
		t.Error("group should be deleted")
	}
	g.SetDeleted(false)
	if g.Deleted() {
		t.Error("group should not be deleted")
	}
}

func TestGroupJSON(t *testing.T) {
	g, _ := NewGroup(5, "Finance")

	data, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var restored Group
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if restored.Name() != "Finance" {
		t.Errorf("name mismatch: %q", restored.Name())
	}
	if restored.CreatedAt().Unix() != g.CreatedAt().Unix() {
		t.Error("createdAt mismatch")
	}
	if restored.UpdatedAt().Unix() != g.UpdatedAt().Unix() {
		t.Error("updatedAt mismatch")
	}
}

func TestGroupJSONDoesNotIncludeIndex(t *testing.T) {
	g, _ := NewGroup(42, "Test")
	data, _ := json.Marshal(g)

	var raw map[string]any
	json.Unmarshal(data, &raw)
	if _, ok := raw["index"]; ok {
		t.Error("JSON should not contain index field - index is on-chain, not in encrypted blob")
	}
	if _, ok := raw["deleted"]; ok {
		t.Error("JSON should not contain deleted field - deleted is on-chain, not in encrypted blob")
	}
}

func TestGroupSetColor(t *testing.T) {
	g, _ := NewGroup(0, "Work")
	if g.Color() != "" {
		t.Error("new group should have empty color")
	}

	g.SetColor("blue")
	if g.Color() != "blue" {
		t.Errorf("expected 'blue', got %q", g.Color())
	}

	g.SetColor("")
	if g.Color() != "" {
		t.Error("color should be cleared")
	}
}

func TestGroupColorValidation(t *testing.T) {
	valid := []GroupColor{GroupColorRed, GroupColorOrange, GroupColorYellow, GroupColorGreen, GroupColorTeal, GroupColorBlue, GroupColorPurple, GroupColorPink, GroupColorGray, ""}
	for _, c := range valid {
		if err := ValidateGroupColor(c); err != nil {
			t.Errorf("color %q should be valid, got error: %v", c, err)
		}
	}

	invalid := []GroupColor{"black", "white", "RED", "Blue", "#ff0000", "random"}
	for _, c := range invalid {
		if err := ValidateGroupColor(c); err == nil {
			t.Errorf("color %q should be invalid", c)
		}
	}
}

func TestGroupColorJSON(t *testing.T) {
	g, _ := NewGroup(0, "Work")
	g.SetColor("teal")

	data, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var restored Group
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if restored.Color() != "teal" {
		t.Errorf("expected color 'teal', got %q", restored.Color())
	}
}

func TestGroupColorJSONOmitsEmpty(t *testing.T) {
	g, _ := NewGroup(0, "Work")

	data, _ := json.Marshal(g)
	var raw map[string]any
	json.Unmarshal(data, &raw)
	if _, ok := raw["color"]; ok {
		t.Error("color should be omitted from JSON when empty")
	}
}

func TestEntryGroupIndex(t *testing.T) {
	e, _ := NewEntry("id-1", EntryTypePassword, "Test", map[string]string{
		"username": "user",
		"password": "pass",
	})

	if e.GroupIndex() != nil {
		t.Error("new entry should have nil groupIndex")
	}

	idx := uint32(3)
	e.SetGroupIndex(&idx)
	if e.GroupIndex() == nil || *e.GroupIndex() != 3 {
		t.Errorf("expected groupIndex 3, got %v", e.GroupIndex())
	}

	e.SetGroupIndex(nil)
	if e.GroupIndex() != nil {
		t.Error("groupIndex should be nil after clearing")
	}
}

func TestEntryGroupIndexJSON(t *testing.T) {
	e, _ := NewEntry("id-1", EntryTypePassword, "Test", map[string]string{
		"username": "user",
		"password": "pass",
	})

	data, _ := json.Marshal(e)
	var raw map[string]any
	json.Unmarshal(data, &raw)
	if _, ok := raw["group_index"]; ok {
		t.Error("group_index should be omitted when nil")
	}

	idx := uint32(2)
	e.SetGroupIndex(&idx)
	data, _ = json.Marshal(e)

	var restored Entry
	json.Unmarshal(data, &restored)
	if restored.GroupIndex() == nil || *restored.GroupIndex() != 2 {
		t.Errorf("expected groupIndex 2 after round-trip, got %v", restored.GroupIndex())
	}
}

func TestEntryGroupIndexZeroValue(t *testing.T) {
	e, _ := NewEntry("id-1", EntryTypePassword, "Test", map[string]string{
		"username": "user",
		"password": "pass",
	})
	idx := uint32(0)
	e.SetGroupIndex(&idx)

	data, _ := json.Marshal(e)

	var restored Entry
	json.Unmarshal(data, &restored)
	if restored.GroupIndex() == nil {
		t.Fatal("groupIndex should not be nil for zero value")
	}
	if *restored.GroupIndex() != 0 {
		t.Errorf("expected groupIndex 0, got %d", *restored.GroupIndex())
	}
}
