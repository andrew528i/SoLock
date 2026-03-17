package service

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"fmt"

	"github.com/solock/solock/internal/domain"
	"github.com/tyler-smith/go-bip39"
)

type keyDeriver struct{}

func NewKeyDeriver() domain.KeyDeriver {
	return &keyDeriver{}
}

func (d *keyDeriver) Derive(password string) (*domain.DerivedKeys, error) {
	deployerEntropy := deriveEntropy(password, 0)
	deployerMnemonic, err := bip39.NewMnemonic(deployerEntropy)
	if err != nil {
		return nil, fmt.Errorf("deployer mnemonic: %w", err)
	}
	deployerSeed := bip39.NewSeed(deployerMnemonic, "")
	deployerPrivKey := ed25519.NewKeyFromSeed(deriveSLIP0010(deployerSeed))

	programEntropy := deriveEntropy(password, 1)
	programMnemonic, err := bip39.NewMnemonic(programEntropy)
	if err != nil {
		return nil, fmt.Errorf("program mnemonic: %w", err)
	}
	programSeed := bip39.NewSeed(programMnemonic, "")
	programPrivKey := ed25519.NewKeyFromSeed(deriveSLIP0010(programSeed))

	encryptionKey := deriveEntropy(password, 0)

	return &domain.DerivedKeys{
		DeployerKeypair:  deployerPrivKey,
		DeployerAddress:  pubkeyToBase58(deployerPrivKey.Public().(ed25519.PublicKey)),
		DeployerMnemonic: deployerMnemonic,
		ProgramKeypair:   programPrivKey,
		ProgramID:        pubkeyToBase58(programPrivKey.Public().(ed25519.PublicKey)),
		ProgramMnemonic:  programMnemonic,
		EncryptionKey:    encryptionKey,
	}, nil
}

func deriveEntropy(password string, index int) []byte {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", password, index)))
	return h[:]
}

func deriveSLIP0010(seed []byte) []byte {
	mac := hmac.New(sha512.New, []byte("ed25519 seed"))
	mac.Write(seed)
	I := mac.Sum(nil)
	key := I[:32]
	chainCode := I[32:]

	path := []uint32{
		44 + 0x80000000,
		501 + 0x80000000,
		0 + 0x80000000,
		0 + 0x80000000,
	}

	for _, index := range path {
		data := make([]byte, 37)
		data[0] = 0x00
		copy(data[1:33], key)
		binary.BigEndian.PutUint32(data[33:], index)

		mac = hmac.New(sha512.New, chainCode)
		mac.Write(data)
		I = mac.Sum(nil)
		key = I[:32]
		chainCode = I[32:]
	}

	return key
}

func pubkeyToBase58(pub ed25519.PublicKey) string {
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	num := make([]byte, len(pub))
	copy(num, pub)

	var result []byte
	for {
		var rem int
		var next []byte
		for _, b := range num {
			acc := rem*256 + int(b)
			rem = acc % 58
			q := acc / 58
			if len(next) > 0 || q > 0 {
				next = append(next, byte(q))
			}
		}
		result = append(result, alphabet[rem])
		num = next
		if len(num) == 0 {
			break
		}
	}

	for _, b := range pub {
		if b != 0 {
			break
		}
		result = append(result, alphabet[0])
	}

	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return string(result)
}
