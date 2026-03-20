package tui

import "github.com/solock/solock/internal/domain"

func (a *App) generatePassword() string {
	cfg := a.passGenConfig
	if cfg == nil {
		cfg = domain.DefaultPasswordGenConfig()
	}
	return a.app.GeneratePassword.Execute(cfg)
}
