package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/solock/solock/internal/domain"
)

type GetConfigUseCase struct {
	config domain.ConfigRepository
}

func NewGetConfigUseCase(config domain.ConfigRepository) *GetConfigUseCase {
	return &GetConfigUseCase{config: config}
}

func (uc *GetConfigUseCase) Execute(ctx context.Context, key string) (string, error) {
	if uc.config == nil {
		return "", fmt.Errorf("not ready")
	}
	return uc.config.Get(ctx, key)
}

type SetConfigUseCase struct {
	config domain.ConfigRepository
}

func NewSetConfigUseCase(config domain.ConfigRepository) *SetConfigUseCase {
	return &SetConfigUseCase{config: config}
}

func (uc *SetConfigUseCase) Execute(ctx context.Context, key, value string) error {
	if uc.config == nil {
		return fmt.Errorf("not ready")
	}
	return uc.config.Set(ctx, key, value)
}

type GetPassGenConfigUseCase struct {
	config domain.ConfigRepository
}

func NewGetPassGenConfigUseCase(config domain.ConfigRepository) *GetPassGenConfigUseCase {
	return &GetPassGenConfigUseCase{config: config}
}

func (uc *GetPassGenConfigUseCase) Execute(ctx context.Context) *domain.PasswordGenConfig {
	cfg := domain.DefaultPasswordGenConfig()
	if uc.config == nil {
		return cfg
	}
	raw, _ := uc.config.Get(ctx, "passgen_config")
	if raw == "" {
		return cfg
	}
	var c domain.PasswordGenConfig
	if err := json.Unmarshal([]byte(raw), &c); err == nil {
		return &c
	}
	return cfg
}

type SavePassGenConfigUseCase struct {
	config domain.ConfigRepository
}

func NewSavePassGenConfigUseCase(config domain.ConfigRepository) *SavePassGenConfigUseCase {
	return &SavePassGenConfigUseCase{config: config}
}

func (uc *SavePassGenConfigUseCase) Execute(ctx context.Context, cfg *domain.PasswordGenConfig) error {
	if uc.config == nil {
		return fmt.Errorf("not ready")
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return uc.config.Set(ctx, "passgen_config", string(data))
}
