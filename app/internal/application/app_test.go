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

func TestTouchExtendsExpiresAt(t *testing.T) {
	a := &App{
		keys:        &domain.DerivedKeys{EncryptionKey: make([]byte, 32)},
		crypto:      &stubCrypto{},
		idleTimeout: 90 * time.Minute,
		expiresAt:   time.Now().Add(10 * time.Minute),
	}

	before := a.ExpiresAt()
	time.Sleep(2 * time.Millisecond)
	a.Touch()
	after := a.ExpiresAt()

	if !after.After(before) {
		t.Fatalf("Touch should extend expiresAt: before=%v after=%v", before, after)
	}
	if d := time.Until(after); d < 89*time.Minute || d > 91*time.Minute {
		t.Fatalf("expected ~90 min remaining, got %v", d)
	}
}

func TestTouchIgnoredWhenLocked(t *testing.T) {
	a := &App{}
	a.Touch()

	if !a.ExpiresAt().IsZero() {
		t.Fatal("Touch on locked app must not set expiresAt")
	}
}

func TestTouchIgnoredWhenIdleTimeoutZero(t *testing.T) {
	a := &App{
		keys:      &domain.DerivedKeys{EncryptionKey: make([]byte, 32)},
		expiresAt: time.Now().Add(5 * time.Minute),
	}
	before := a.ExpiresAt()
	a.Touch()
	after := a.ExpiresAt()
	if !after.Equal(before) {
		t.Fatalf("Touch should not change expiresAt without idleTimeout")
	}
}

func TestLockWipesCrypto(t *testing.T) {
	sc := &stubCrypto{key: []byte{1, 2, 3}}
	a := &App{
		keys:        &domain.DerivedKeys{EncryptionKey: []byte{1, 2, 3}},
		crypto:      sc,
		idleTimeout: time.Hour,
		expiresAt:   time.Now().Add(time.Hour),
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

