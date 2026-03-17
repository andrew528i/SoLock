package tui

import (
	"crypto/rand"
	"math/big"

	"github.com/solock/solock/internal/domain"
)

const (
	charsLower   = "abcdefghijklmnopqrstuvwxyz"
	charsUpper   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	charsDigits  = "0123456789"
	charsSpecial = "!@#$%^&*()-_=+[]{}|;:,.<>?"
)

func generatePasswordWithConfig(cfg *domain.PasswordGenConfig) string {
	if cfg == nil {
		cfg = domain.DefaultPasswordGenConfig()
	}
	length := cfg.Length
	if length < 8 {
		length = 8
	}

	charset := charsLower
	var required []string
	required = append(required, charsLower)

	if cfg.Uppercase {
		charset += charsUpper
		required = append(required, charsUpper)
	}
	if cfg.Digits {
		charset += charsDigits
		required = append(required, charsDigits)
	}
	if cfg.Special {
		charset += charsSpecial
		required = append(required, charsSpecial)
	}

	result := make([]byte, length)
	for i := 0; i < len(required) && i < length; i++ {
		result[i] = randomChar(required[i])
	}
	for i := len(required); i < length; i++ {
		result[i] = randomChar(charset)
	}

	for i := length - 1; i > 0; i-- {
		j, _ := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		result[i], result[j.Int64()] = result[j.Int64()], result[i]
	}

	return string(result)
}

func randomChar(charset string) byte {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
	return charset[n.Int64()]
}
