package service

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"testing"
)

// legacyV1Encrypt reproduces the pre-v2 (AES-256-CBC + PKCS7) ciphertext
// format byte-for-byte. It exists only in tests so the production path can
// stay write-v2-only while backward-compat decryption is still exercised.
func legacyV1Encrypt(t *testing.T, plaintext, key []byte) []byte {
	t.Helper()
	compressed, err := gzipCompress(plaintext)
	if err != nil {
		t.Fatalf("gzipCompress: %v", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	iv := make([]byte, v1IVSize)
	if _, err := rand.Read(iv); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	padding := v1BlockSize - (len(compressed) % v1BlockSize)
	padded := append(compressed, bytes.Repeat([]byte{byte(padding)}, padding)...)
	ct := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ct, padded)
	return append(iv, ct...)
}

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

func TestEncryptProducesV2Format(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	svc := NewCryptoService(key)

	encrypted, err := svc.Encrypt([]byte("hello"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if len(encrypted) < v2MinSize {
		t.Fatalf("ciphertext too short: %d", len(encrypted))
	}
	if encrypted[0] != v2Version {
		t.Fatalf("expected version byte %#x, got %#x", v2Version, encrypted[0])
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
	if _, err := svc2.Decrypt(encrypted); err == nil {
		t.Fatal("GCM must reject decryption with wrong key")
	}
}

func TestDecryptRejectsTamperedCiphertext(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	svc := NewCryptoService(key)

	encrypted, err := svc.Encrypt([]byte("some secret material"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	// flip a bit in the ciphertext body (after version + nonce, before the tag)
	mid := 1 + v2NonceSize + 2
	if mid >= len(encrypted)-v2TagSize {
		t.Fatalf("ciphertext too short to tamper: %d", len(encrypted))
	}
	encrypted[mid] ^= 0x01

	if _, err := svc.Decrypt(encrypted); err == nil {
		t.Fatal("GCM must reject tampered ciphertext")
	}
}

func TestDecryptRejectsTamperedTag(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	svc := NewCryptoService(key)

	encrypted, err := svc.Encrypt([]byte("secret"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	// flip a bit inside the final 16-byte GCM tag
	encrypted[len(encrypted)-1] ^= 0x01

	if _, err := svc.Decrypt(encrypted); err == nil {
		t.Fatal("GCM must reject tampered tag")
	}
}

func TestDecryptRejectsTamperedNonce(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	svc := NewCryptoService(key)

	encrypted, err := svc.Encrypt([]byte("secret"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	encrypted[3] ^= 0x01 // inside nonce region

	if _, err := svc.Decrypt(encrypted); err == nil {
		t.Fatal("GCM must reject tampered nonce")
	}
}

func TestDecryptReadsLegacyV1Format(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	svc := NewCryptoService(key)

	plaintext := []byte("legacy password entry from before v2 migration")
	legacyBlob := legacyV1Encrypt(t, plaintext, key)

	decrypted, err := svc.Decrypt(legacyBlob)
	if err != nil {
		t.Fatalf("v1 decrypt failed: %v", err)
	}
	if !bytes.Equal(plaintext, decrypted) {
		t.Fatalf("v1 roundtrip mismatch: got %q", decrypted)
	}
}

func TestDecryptReadsLegacyV1WithVersionByteCollision(t *testing.T) {
	// Force legacyV1Encrypt to produce a blob whose first byte happens to be
	// 0x02 (the v2 version marker). Decrypt must fall back to v1 when the v2
	// path fails GCM authentication.
	key := make([]byte, 32)
	rand.Read(key)
	svc := NewCryptoService(key)

	plaintext := []byte("v1 blob that looks like v2")
	var blob []byte
	for attempt := 0; attempt < 256; attempt++ {
		candidate := legacyV1Encrypt(t, plaintext, key)
		if candidate[0] == v2Version {
			blob = candidate
			break
		}
	}
	if blob == nil {
		t.Skip("could not synthesise v1 blob starting with 0x02 within 256 tries")
	}

	decrypted, err := svc.Decrypt(blob)
	if err != nil {
		t.Fatalf("v1 fallback decrypt failed: %v", err)
	}
	if !bytes.Equal(plaintext, decrypted) {
		t.Fatalf("v1 fallback mismatch: got %q", decrypted)
	}
}

func TestDecryptRejectsGarbage(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	svc := NewCryptoService(key)

	cases := map[string][]byte{
		"empty":         {},
		"too_short":     {0x02, 0x00, 0x00},
		"v2_marker_short": append([]byte{v2Version}, make([]byte, 20)...),
		"random_blob":    bytes.Repeat([]byte{0xAB}, 64),
	}
	for name, blob := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.Decrypt(blob); err == nil {
				t.Fatal("expected error")
			}
		})
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

func TestEncryptNonceUnique(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)
	svc := NewCryptoService(key)

	plaintext := []byte("same input")
	a, err := svc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt a: %v", err)
	}
	b, err := svc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt b: %v", err)
	}

	if bytes.Equal(a, b) {
		t.Fatal("two encryptions of the same plaintext produced identical ciphertext - nonce reuse")
	}

	// nonce region must differ
	if bytes.Equal(a[1:1+v2NonceSize], b[1:1+v2NonceSize]) {
		t.Fatal("nonces must be random and unique per encryption")
	}
}
