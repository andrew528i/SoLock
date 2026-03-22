package storage

import (
	"context"
	"testing"

	"github.com/solock/solock/internal/domain"
)

func testGroupRepo(t *testing.T) *GroupRepo {
	t.Helper()
	entryRepo, _, _ := testRepos(t)
	return &GroupRepo{s: entryRepo.s}
}

func TestGroupSaveAndGet(t *testing.T) {
	repo := testGroupRepo(t)
	ctx := context.Background()

	g, _ := domain.NewGroup(0, "Work")

	if err := repo.Save(ctx, g); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := repo.Get(ctx, 0)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil {
		t.Fatal("group should not be nil")
	}
	if got.Name() != "Work" {
		t.Errorf("expected 'Work', got %q", got.Name())
	}
	if got.Index() != 0 {
		t.Errorf("expected index 0, got %d", got.Index())
	}
	if got.Deleted() {
		t.Error("should not be deleted")
	}
}

func TestGroupSaveDeleted(t *testing.T) {
	repo := testGroupRepo(t)
	ctx := context.Background()

	g, _ := domain.NewGroup(1, "Old Group")
	g.SetDeleted(true)

	repo.Save(ctx, g)

	got, _ := repo.Get(ctx, 1)
	if got == nil {
		t.Fatal("group should not be nil")
	}
	if !got.Deleted() {
		t.Error("group should be deleted")
	}
	if got.Name() != "Old Group" {
		t.Errorf("name mismatch: %q", got.Name())
	}
}

func TestGroupUpdate(t *testing.T) {
	repo := testGroupRepo(t)
	ctx := context.Background()

	g, _ := domain.NewGroup(0, "Work")
	repo.Save(ctx, g)

	g.SetName("Personal")
	repo.Save(ctx, g)

	got, _ := repo.Get(ctx, 0)
	if got.Name() != "Personal" {
		t.Errorf("expected 'Personal' after update, got %q", got.Name())
	}
}

func TestGroupList(t *testing.T) {
	repo := testGroupRepo(t)
	ctx := context.Background()

	names := []string{"Work", "Personal", "Finance"}
	for i, name := range names {
		g, _ := domain.NewGroup(uint32(i), name)
		repo.Save(ctx, g)
	}

	groups, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}

	for i, name := range names {
		if groups[i].Name() != name {
			t.Errorf("group %d: expected %q, got %q", i, name, groups[i].Name())
		}
		if groups[i].Index() != uint32(i) {
			t.Errorf("group %d: expected index %d, got %d", i, i, groups[i].Index())
		}
	}
}

func TestGroupListOrder(t *testing.T) {
	repo := testGroupRepo(t)
	ctx := context.Background()

	g2, _ := domain.NewGroup(5, "Second")
	g1, _ := domain.NewGroup(2, "First")
	repo.Save(ctx, g2)
	repo.Save(ctx, g1)

	groups, _ := repo.List(ctx)
	if len(groups) != 2 {
		t.Fatalf("expected 2, got %d", len(groups))
	}
	if groups[0].Index() != 2 {
		t.Error("should be ordered by index ASC")
	}
	if groups[1].Index() != 5 {
		t.Error("should be ordered by index ASC")
	}
}

func TestGroupDelete(t *testing.T) {
	repo := testGroupRepo(t)
	ctx := context.Background()

	g, _ := domain.NewGroup(0, "ToDelete")
	repo.Save(ctx, g)

	if err := repo.Delete(ctx, 0); err != nil {
		t.Fatalf("delete: %v", err)
	}

	got, _ := repo.Get(ctx, 0)
	if got != nil {
		t.Error("group should be nil after delete")
	}
}

func TestGroupClearAll(t *testing.T) {
	repo := testGroupRepo(t)
	ctx := context.Background()

	for i := range 5 {
		g, _ := domain.NewGroup(uint32(i), "Group")
		repo.Save(ctx, g)
	}

	if err := repo.ClearAll(ctx); err != nil {
		t.Fatalf("clear: %v", err)
	}

	groups, _ := repo.List(ctx)
	if len(groups) != 0 {
		t.Errorf("expected 0 groups after clear, got %d", len(groups))
	}
}

func TestGroupGetNotFound(t *testing.T) {
	repo := testGroupRepo(t)
	ctx := context.Background()

	got, err := repo.Get(ctx, 999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("should return nil for non-existent group")
	}
}

func TestGroupDeletedPreserved(t *testing.T) {
	repo := testGroupRepo(t)
	ctx := context.Background()

	g, _ := domain.NewGroup(3, "WillDelete")
	repo.Save(ctx, g)

	g.SetDeleted(true)
	repo.Save(ctx, g)

	got, _ := repo.Get(ctx, 3)
	if !got.Deleted() {
		t.Error("deleted flag should be preserved")
	}

	g.SetDeleted(false)
	repo.Save(ctx, g)

	got, _ = repo.Get(ctx, 3)
	if got.Deleted() {
		t.Error("deleted flag should be cleared")
	}
}
