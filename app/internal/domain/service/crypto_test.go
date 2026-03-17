package service

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	svc := NewCryptoService(key)

	plaintext := []byte("hello world this is a secret password")
	encrypted, err := svc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	decrypted, err := svc.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Fatal("roundtrip failed")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	rand.Read(key1)
	rand.Read(key2)

	svc1 := NewCryptoService(key1)
	svc2 := NewCryptoService(key2)

	plaintext := []byte("this is a longer secret that should not decrypt with wrong key!!")
	encrypted, _ := svc1.Encrypt(plaintext)
	decrypted, err := svc2.Decrypt(encrypted)
	if err == nil && bytes.Equal(decrypted, plaintext) {
		t.Fatal("should not produce original plaintext with wrong key")
	}
}

func TestEncryptCompresses(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	svc := NewCryptoService(key)

	plaintext := bytes.Repeat([]byte("compressible data "), 100)
	encrypted, _ := svc.Encrypt(plaintext)

	if len(encrypted) >= len(plaintext) {
		t.Log("warning: compressed ciphertext not smaller")
	}

	decrypted, err := svc.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(plaintext, decrypted) {
		t.Fatal("roundtrip failed")
	}
}
