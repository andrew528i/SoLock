package service

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"

	"github.com/solock/solock/internal/domain"
)

const (
	// v1 (legacy, read-only): AES-256-CBC with PKCS7 padding, 16-byte random IV
	// prepended to ciphertext. Total length is always a multiple of 16 and >= 32.
	v1IVSize    = 16
	v1BlockSize = aes.BlockSize

	// v2 (current): AES-256-GCM. Layout: [0x02][12-byte nonce][ciphertext][16-byte tag].
	// GCM provides authenticated encryption - tampered blobs are rejected on decrypt.
	v2Version   byte = 0x02
	v2NonceSize      = 12
	v2TagSize        = 16
	v2MinSize        = 1 + v2NonceSize + v2TagSize
)

var (
	errInvalidCiphertext = errors.New("invalid ciphertext")
	errInvalidPadding    = errors.New("invalid PKCS7 padding")
	errUnknownVersion    = errors.New("unknown ciphertext version")
)

type cryptoService struct {
	key []byte
}

func NewCryptoService(key []byte) domain.CryptoService {
	return &cryptoService{key: key}
}

func (s *cryptoService) Wipe() {
	for i := range s.key {
		s.key[i] = 0
	}
	s.key = nil
}

func (s *cryptoService) Encrypt(plaintext []byte) ([]byte, error) {
	compressed, err := gzipCompress(plaintext)
	if err != nil {
		return nil, err
	}
	return aesGCMEncrypt(compressed, s.key)
}

// Decrypt recognises both the current v2 (AES-GCM) format and the legacy v1
// (AES-CBC) format. v2 blobs start with a 0x02 version byte; v1 blobs have no
// version prefix and are always a multiple of 16 bytes long. A v1 blob whose
// first byte happens to be 0x02 will fail the v2 GCM tag check and fall back
// to v1 decoding automatically.
func (s *cryptoService) Decrypt(ciphertext []byte) ([]byte, error) {
	if looksLikeV2(ciphertext) {
		plaintext, err := aesGCMDecrypt(ciphertext, s.key)
		if err == nil {
			return gzipDecompress(plaintext)
		}
		if !looksLikeV1(ciphertext) {
			return nil, err
		}
	}

	if !looksLikeV1(ciphertext) {
		return nil, errInvalidCiphertext
	}
	plaintext, err := aesCBCDecrypt(ciphertext, s.key)
	if err != nil {
		return nil, err
	}
	return gzipDecompress(plaintext)
}

func looksLikeV2(blob []byte) bool {
	return len(blob) >= v2MinSize && blob[0] == v2Version
}

func looksLikeV1(blob []byte) bool {
	return len(blob) >= v1IVSize+v1BlockSize && len(blob)%v1BlockSize == 0
}

func aesGCMEncrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, v2NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	out := make([]byte, 0, 1+v2NonceSize+len(plaintext)+v2TagSize)
	out = append(out, v2Version)
	out = append(out, nonce...)
	out = gcm.Seal(out, nonce, plaintext, nil)
	return out, nil
}

func aesGCMDecrypt(blob, key []byte) ([]byte, error) {
	if len(blob) < v2MinSize {
		return nil, errInvalidCiphertext
	}
	if blob[0] != v2Version {
		return nil, errUnknownVersion
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := blob[1 : 1+v2NonceSize]
	ciphertext := blob[1+v2NonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func aesCBCDecrypt(ciphertext, key []byte) ([]byte, error) {
	if len(ciphertext) < v1IVSize+v1BlockSize {
		return nil, errInvalidCiphertext
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	iv := ciphertext[:v1IVSize]
	encrypted := ciphertext[v1IVSize:]
	if len(encrypted)%v1BlockSize != 0 {
		return nil, errInvalidCiphertext
	}

	plaintext := make([]byte, len(encrypted))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plaintext, encrypted)

	return pkcs7Unpad(plaintext)
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errInvalidPadding
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > v1BlockSize || padding > len(data) {
		return nil, errInvalidPadding
	}
	for i := 0; i < padding; i++ {
		if data[len(data)-1-i] != byte(padding) {
			return nil, errInvalidPadding
		}
	}
	return data[:len(data)-padding], nil
}

func gzipCompress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gzipDecompress(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
