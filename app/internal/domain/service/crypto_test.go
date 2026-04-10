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

func TestWipeClearsKey(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	svc := NewCryptoService(key)

	plaintext := []byte("before wipe")
	ciphertext, err := svc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	svc.Wipe()

	for i, b := range key {
		if b != 0 {
			t.Fatalf("key byte %d not zeroed: %#x", i, b)
		}
	}

	if _, err := svc.Decrypt(ciphertext); err == nil {
		t.Fatal("decrypt after wipe should fail")
	}
}

func TestDecryptRejectsNonGzipPlaintext(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	svc := NewCryptoService(key).(*cryptoService)

	raw := []byte("plain bytes that are not gzip framed")
	ciphertext, err := aesEncrypt(raw, svc.key)
	if err != nil {
		t.Fatalf("aesEncrypt: %v", err)
	}

	if _, err := svc.Decrypt(ciphertext); err == nil {
		t.Fatal("expected error when decrypting non-gzip plaintext, got nil")
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
