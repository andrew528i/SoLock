package application

import (
	"testing"
	"time"

	"github.com/solock/solock/internal/domain"
)

type stubCrypto struct{ key []byte }

func (s *stubCrypto) Encrypt(data []byte) ([]byte, error) { return data, nil }
func (s *stubCrypto) Decrypt(data []byte) ([]byte, error) { return data, nil }
func (s *stubCrypto) Wipe()                                { s.key = nil }

func TestLockWipesCrypto(t *testing.T) {
	sc := &stubCrypto{key: []byte{1, 2, 3}}
	a := &App{
		keys:      &domain.DerivedKeys{EncryptionKey: []byte{1, 2, 3}},
		crypto:    sc,
		expiresAt: time.Now().Add(time.Hour),
	}

	a.Lock()

	if !a.IsLocked() {
		t.Fatal("app should be locked")
	}
	if sc.key != nil {
		t.Fatal("crypto.Wipe should have been called")
	}
	if !a.ExpiresAt().IsZero() {
		t.Fatal("expiresAt should be zero after Lock")
	}
}
