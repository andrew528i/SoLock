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

func TestAddEntryOnChain(t *testing.T) {
	ctrl := gomock.NewController(t)

	entries := mock.NewMockEntryRepository(ctrl)
	vault := mock.NewMockVaultRepository(ctrl)
	syncState := mock.NewMockSyncStateRepository(ctrl)
	key := make([]byte, 32)
	crypto := service.NewCryptoService(key)

	uc := NewAddEntryUseCase(entries, vault, syncState, crypto)

	entry, _ := domain.NewEntry("e1", domain.EntryTypePassword, "Test", map[string]string{
		"username": "user",
		"password": "pass",
	})

	vault.EXPECT().Exists(gomock.Any()).Return(true, nil)
	vault.EXPECT().GetMeta(gomock.Any()).Return(&domain.VaultMeta{NextIndex: 0, EntryCount: 0}, nil)
	vault.EXPECT().AddEntry(gomock.Any(), uint32(0), gomock.Any()).Return(nil)
	vault.EXPECT().GetEntry(gomock.Any(), uint32(0)).Return(&domain.EntryAccount{
		Index: 0, CreatedAt: 1000, UpdatedAt: 1000,
	}, nil)
	entries.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
	entries.EXPECT().MarkSynced(gomock.Any(), "e1").Return(nil)

	result, err := uc.Execute(context.Background(), entry)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !result.OnChain {
		t.Error("should be on-chain")
	}
}

func TestAddEntryLocalOnly(t *testing.T) {
	ctrl := gomock.NewController(t)

	entries := mock.NewMockEntryRepository(ctrl)
	syncState := mock.NewMockSyncStateRepository(ctrl)
	key := make([]byte, 32)
	crypto := service.NewCryptoService(key)

	uc := NewAddEntryUseCase(entries, nil, syncState, crypto)

	entry, _ := domain.NewEntry("e1", domain.EntryTypeNote, "Note", map[string]string{
		"content": "text",
	})

	syncState.EXPECT().Get(gomock.Any()).Return(&domain.SyncState{NextIndex: 0}, nil)
	syncState.EXPECT().Set(gomock.Any(), gomock.Any()).Return(nil)
	entries.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)

	result, err := uc.Execute(context.Background(), entry)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.OnChain {
		t.Error("should be local only")
	}
}

func TestAddEntryRetryOnCollision(t *testing.T) {
	ctrl := gomock.NewController(t)

	entries := mock.NewMockEntryRepository(ctrl)
	vault := mock.NewMockVaultRepository(ctrl)
	syncState := mock.NewMockSyncStateRepository(ctrl)
	key := make([]byte, 32)
	crypto := service.NewCryptoService(key)

	uc := NewAddEntryUseCase(entries, vault, syncState, crypto)

	entry, _ := domain.NewEntry("e1", domain.EntryTypePassword, "Test", map[string]string{
		"username": "user",
		"password": "pass",
	})

	vault.EXPECT().Exists(gomock.Any()).Return(true, nil)
	vault.EXPECT().GetMeta(gomock.Any()).Return(&domain.VaultMeta{NextIndex: 5, EntryCount: 5}, nil)

	vault.EXPECT().AddEntry(gomock.Any(), uint32(5), gomock.Any()).Return(fmt.Errorf("account already in use"))
	vault.EXPECT().GetMeta(gomock.Any()).Return(&domain.VaultMeta{NextIndex: 6, EntryCount: 6}, nil)
	vault.EXPECT().AddEntry(gomock.Any(), uint32(6), gomock.Any()).Return(nil)
	vault.EXPECT().GetEntry(gomock.Any(), uint32(6)).Return(&domain.EntryAccount{
		Index: 6, CreatedAt: 2000, UpdatedAt: 2000,
	}, nil)

	entries.EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
	entries.EXPECT().MarkSynced(gomock.Any(), "e1").Return(nil)

	result, err := uc.Execute(context.Background(), entry)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !result.OnChain {
		t.Error("should be on-chain after retry")
	}
}
