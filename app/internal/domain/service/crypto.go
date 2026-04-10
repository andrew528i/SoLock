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
	ivSize    = 16
	blockSize = aes.BlockSize
)

var (
	errInvalidCiphertext = errors.New("invalid ciphertext")
	errInvalidPadding    = errors.New("invalid PKCS7 padding")
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
	return aesEncrypt(compressed, s.key)
}

func (s *cryptoService) Decrypt(ciphertext []byte) ([]byte, error) {
	decrypted, err := aesDecrypt(ciphertext, s.key)
	if err != nil {
		return nil, err
	}
	return gzipDecompress(decrypted)
}

func aesEncrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	iv := make([]byte, ivSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	padded := pkcs7Pad(plaintext, blockSize)
	ct := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ct, padded)

	return append(iv, ct...), nil
}

func aesDecrypt(ciphertext, key []byte) ([]byte, error) {
	if len(ciphertext) < ivSize+blockSize {
		return nil, errInvalidCiphertext
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	iv := ciphertext[:ivSize]
	encrypted := ciphertext[ivSize:]
	if len(encrypted)%blockSize != 0 {
		return nil, errInvalidCiphertext
	}

	plaintext := make([]byte, len(encrypted))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plaintext, encrypted)

	return pkcs7Unpad(plaintext)
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - (len(data) % blockSize)
	return append(data, bytes.Repeat([]byte{byte(padding)}, padding)...)
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errInvalidPadding
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > blockSize || padding > len(data) {
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
