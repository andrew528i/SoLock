package usecase

import (
	"testing"

	"github.com/solock/solock/internal/domain"
)

func makeEntry(t *testing.T, name string, fields map[string]string) *domain.Entry {
	t.Helper()
	e, err := domain.NewEntry("id-"+name, domain.EntryTypePassword, name, fields)
	if err != nil {
		t.Fatalf("new entry %q: %v", name, err)
	}
	return e
}

func TestFuzzyFilterMatchesUsernameField(t *testing.T) {
	entries := []*domain.Entry{
		makeEntry(t, "GitHub", map[string]string{"username": "octocat", "password": "p"}),
		makeEntry(t, "GitLab", map[string]string{"username": "tanuki", "password": "p"}),
	}

	got := FuzzyFilter(entries, "octocat")
	if len(got) != 1 || got[0].Name() != "GitHub" {
		t.Fatalf("want [GitHub], got %v", got)
	}
}

func TestFuzzyFilterMatchesUrlField(t *testing.T) {
	entries := []*domain.Entry{
		makeEntry(t, "Home", map[string]string{"site": "example.com", "username": "u", "password": "p"}),
		makeEntry(t, "Work", map[string]string{"site": "corp.io", "username": "u", "password": "p"}),
	}

	got := FuzzyFilter(entries, "corp")
	if len(got) != 1 || got[0].Name() != "Work" {
		t.Fatalf("want [Work], got %v", got)
	}
}

func TestFuzzyFilterMatchesNotesField(t *testing.T) {
	entries := []*domain.Entry{
		makeEntry(t, "Bank", map[string]string{"username": "u", "password": "p", "notes": "security questions: maiden name"}),
		makeEntry(t, "Email", map[string]string{"username": "u", "password": "p", "notes": "recovery code 12345"}),
	}

	got := FuzzyFilter(entries, "recovery")
	if len(got) != 1 || got[0].Name() != "Email" {
		t.Fatalf("want [Email], got %v", got)
	}
}

func TestFuzzyFilterMatchesGroupName(t *testing.T) {
	finance, _ := domain.NewGroup(0, "Finance")
	work, _ := domain.NewGroup(1, "Work")
	groups := []*domain.Group{finance, work}

	financeIdx := uint32(0)
	workIdx := uint32(1)

	bank := makeEntry(t, "Bank", map[string]string{"username": "u", "password": "p"})
	bank.SetGroupIndex(&financeIdx)
	jira := makeEntry(t, "Jira", map[string]string{"username": "u", "password": "p"})
	jira.SetGroupIndex(&workIdx)

	got := FuzzyFilterWithGroups([]*domain.Entry{bank, jira}, groups, "finance")
	if len(got) != 1 || got[0].Name() != "Bank" {
		t.Fatalf("want [Bank], got %v", got)
	}
}

func TestFuzzyFilterEmptyQueryReturnsAll(t *testing.T) {
	entries := []*domain.Entry{
		makeEntry(t, "A", map[string]string{"username": "u", "password": "p"}),
		makeEntry(t, "B", map[string]string{"username": "u", "password": "p"}),
	}

	got := FuzzyFilter(entries, "")
	if len(got) != 2 {
		t.Fatalf("want 2 entries, got %d", len(got))
	}
}

func TestFuzzyFilterMatchesEntryType(t *testing.T) {
	password := makeEntry(t, "Login", map[string]string{"username": "u", "password": "p"})
	note, err := domain.NewEntry("n1", domain.EntryTypeNote, "Secret", map[string]string{"content": "text"})
	if err != nil {
		t.Fatalf("note: %v", err)
	}

	got := FuzzyFilter([]*domain.Entry{password, note}, "note")
	if len(got) != 1 || got[0].Name() != "Secret" {
		t.Fatalf("want [Secret], got %v", got)
	}
}
