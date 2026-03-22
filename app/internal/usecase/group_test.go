package usecase

import (
	"context"
	"fmt"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/solock/solock/internal/domain"
	"github.com/solock/solock/internal/domain/service"
	"github.com/solock/solock/internal/mock"
)

func testCrypto() domain.CryptoService {
	return service.NewCryptoService(make([]byte, 32))
}

func TestAddGroupSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	groups := mock.NewMockGroupRepository(ctrl)
	vault := mock.NewMockVaultRepository(ctrl)
	crypto := testCrypto()

	uc := NewAddGroupUseCase(groups, vault, crypto)

	vault.EXPECT().GetMeta(gomock.Any()).Return(&domain.VaultMeta{NextGroupIndex: 0}, nil)
	vault.EXPECT().AddGroup(gomock.Any(), uint32(0), gomock.Any()).Return(nil)
	groups.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)

	result, err := uc.Execute(context.Background(), "Work")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !result.OnChain {
		t.Error("should be on-chain")
	}
	if result.Group.Name() != "Work" {
		t.Errorf("expected name 'Work', got %q", result.Group.Name())
	}
	if result.Group.Index() != 0 {
		t.Errorf("expected index 0, got %d", result.Group.Index())
	}
}

func TestAddGroupAllocatesCorrectSlot(t *testing.T) {
	ctrl := gomock.NewController(t)
	groups := mock.NewMockGroupRepository(ctrl)
	vault := mock.NewMockVaultRepository(ctrl)
	crypto := testCrypto()

	uc := NewAddGroupUseCase(groups, vault, crypto)

	vault.EXPECT().GetMeta(gomock.Any()).Return(&domain.VaultMeta{NextGroupIndex: 5, GroupCount: 3}, nil)
	vault.EXPECT().AddGroup(gomock.Any(), uint32(5), gomock.Any()).Return(nil)
	groups.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)

	result, err := uc.Execute(context.Background(), "Finance")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Group.Index() != 5 {
		t.Errorf("expected index 5 (next_group_index), got %d", result.Group.Index())
	}
}

func TestAddGroupValidation(t *testing.T) {
	ctrl := gomock.NewController(t)
	groups := mock.NewMockGroupRepository(ctrl)
	vault := mock.NewMockVaultRepository(ctrl)
	crypto := testCrypto()

	uc := NewAddGroupUseCase(groups, vault, crypto)

	_, err := uc.Execute(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestAddGroupVaultError(t *testing.T) {
	ctrl := gomock.NewController(t)
	groups := mock.NewMockGroupRepository(ctrl)
	vault := mock.NewMockVaultRepository(ctrl)
	crypto := testCrypto()

	uc := NewAddGroupUseCase(groups, vault, crypto)

	vault.EXPECT().GetMeta(gomock.Any()).Return(nil, fmt.Errorf("connection error"))

	_, err := uc.Execute(context.Background(), "Test")
	if err == nil {
		t.Error("expected error when vault fails")
	}
}

func TestUpdateGroupSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	groups := mock.NewMockGroupRepository(ctrl)
	vault := mock.NewMockVaultRepository(ctrl)
	crypto := testCrypto()

	uc := NewUpdateGroupUseCase(groups, vault, crypto)

	g, _ := domain.NewGroup(0, "Work")
	g.SetName("Personal")

	vault.EXPECT().UpdateGroup(gomock.Any(), uint32(0), gomock.Any()).Return(nil)
	groups.EXPECT().Save(gomock.Any(), g).Return(nil)

	result, err := uc.Execute(context.Background(), g)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !result.OnChain {
		t.Error("should be on-chain")
	}
}

func TestUpdateGroupValidation(t *testing.T) {
	ctrl := gomock.NewController(t)
	groups := mock.NewMockGroupRepository(ctrl)
	vault := mock.NewMockVaultRepository(ctrl)
	crypto := testCrypto()

	uc := NewUpdateGroupUseCase(groups, vault, crypto)

	g, _ := domain.NewGroup(0, "Work")
	g.SetName("")

	_, err := uc.Execute(context.Background(), g)
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestDeleteGroupSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	groups := mock.NewMockGroupRepository(ctrl)
	entries := mock.NewMockEntryRepository(ctrl)
	vault := mock.NewMockVaultRepository(ctrl)

	uc := NewDeleteGroupUseCase(groups, entries, vault)

	g, _ := domain.NewGroup(0, "Work")
	groups.EXPECT().Get(gomock.Any(), uint32(0)).Return(g, nil)
	vault.EXPECT().DeleteGroup(gomock.Any(), uint32(0)).Return(nil)
	groups.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)

	result, err := uc.Execute(context.Background(), 0, false)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !result.OnChain {
		t.Error("should be on-chain")
	}
}

func TestDeleteGroupNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	groups := mock.NewMockGroupRepository(ctrl)
	entries := mock.NewMockEntryRepository(ctrl)
	vault := mock.NewMockVaultRepository(ctrl)

	uc := NewDeleteGroupUseCase(groups, entries, vault)

	groups.EXPECT().Get(gomock.Any(), uint32(99)).Return(nil, nil)

	_, err := uc.Execute(context.Background(), 99, false)
	if err == nil {
		t.Error("expected error for non-existent group")
	}
}

func TestDeleteGroupWithEntries(t *testing.T) {
	ctrl := gomock.NewController(t)
	groups := mock.NewMockGroupRepository(ctrl)
	entries := mock.NewMockEntryRepository(ctrl)
	vault := mock.NewMockVaultRepository(ctrl)

	uc := NewDeleteGroupUseCase(groups, entries, vault)

	g, _ := domain.NewGroup(2, "Work")
	groups.EXPECT().Get(gomock.Any(), uint32(2)).Return(g, nil)

	idx := uint32(2)
	e1, _ := domain.NewEntry("e1", domain.EntryTypePassword, "Test", map[string]string{
		"username": "user", "password": "pass",
	})
	e1.SetGroupIndex(&idx)
	e1.SetSlotIndex(0)

	e2, _ := domain.NewEntry("e2", domain.EntryTypeNote, "Note", map[string]string{
		"content": "text",
	})

	entries.EXPECT().List(gomock.Any()).Return([]*domain.Entry{e1, e2}, nil)
	vault.EXPECT().GetEntry(gomock.Any(), uint32(0)).Return(&domain.EntryAccount{Index: 0}, nil)
	vault.EXPECT().DeleteEntry(gomock.Any(), uint32(0)).Return(nil)
	entries.EXPECT().Delete(gomock.Any(), "e1").Return(nil)

	vault.EXPECT().DeleteGroup(gomock.Any(), uint32(2)).Return(nil)
	groups.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)

	result, err := uc.Execute(context.Background(), 2, true)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !result.OnChain {
		t.Error("should be on-chain")
	}
}

func TestPurgeGroupSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	groups := mock.NewMockGroupRepository(ctrl)
	vault := mock.NewMockVaultRepository(ctrl)

	uc := NewPurgeGroupUseCase(groups, vault)

	g, _ := domain.NewGroup(0, "Old")
	g.SetDeleted(true)
	groups.EXPECT().Get(gomock.Any(), uint32(0)).Return(g, nil)
	vault.EXPECT().PurgeGroup(gomock.Any(), uint32(0)).Return(nil)
	groups.EXPECT().Delete(gomock.Any(), uint32(0)).Return(nil)

	result, err := uc.Execute(context.Background(), 0)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !result.OnChain {
		t.Error("should be on-chain")
	}
}

func TestPurgeGroupNotDeleted(t *testing.T) {
	ctrl := gomock.NewController(t)
	groups := mock.NewMockGroupRepository(ctrl)
	vault := mock.NewMockVaultRepository(ctrl)

	uc := NewPurgeGroupUseCase(groups, vault)

	g, _ := domain.NewGroup(0, "Active")
	groups.EXPECT().Get(gomock.Any(), uint32(0)).Return(g, nil)

	_, err := uc.Execute(context.Background(), 0)
	if err == nil {
		t.Error("expected error when purging non-deleted group")
	}
}

func TestListGroupsSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	groups := mock.NewMockGroupRepository(ctrl)

	uc := NewListGroupsUseCase(groups)

	g1, _ := domain.NewGroup(0, "Work")
	g2, _ := domain.NewGroup(1, "Personal")
	groups.EXPECT().List(gomock.Any()).Return([]*domain.Group{g1, g2}, nil)

	result, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(result))
	}
	if result[0].Name() != "Work" {
		t.Errorf("expected 'Work', got %q", result[0].Name())
	}
}
