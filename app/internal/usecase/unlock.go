package usecase

import (
	"context"

	"github.com/solock/solock/internal/domain"
	"github.com/solock/solock/internal/domain/service"
)

type UnlockResult struct {
	Keys    *domain.DerivedKeys
	Crypto  domain.CryptoService
}

type UnlockUseCase struct {
	keyDeriver domain.KeyDeriver
}

func NewUnlockUseCase(keyDeriver domain.KeyDeriver) *UnlockUseCase {
	return &UnlockUseCase{keyDeriver: keyDeriver}
}

func (uc *UnlockUseCase) Execute(_ context.Context, password string) (*UnlockResult, error) {
	keys, err := uc.keyDeriver.Derive(password)
	if err != nil {
		return nil, err
	}

	crypto := service.NewCryptoService(keys.EncryptionKey)

	return &UnlockResult{
		Keys:   keys,
		Crypto: crypto,
	}, nil
}
