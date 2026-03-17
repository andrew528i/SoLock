package adapter

import (
	"bytes"
	_ "embed"
	"fmt"

	"github.com/gagliardetto/solana-go"
)

//go:embed program/solock.so
var embeddedProgram []byte

var placeholderBytes = []byte("SOLOCK_PLACEHOLDER_PROGRAM_V1___")

func PatchedProgramBinary(programID string) ([]byte, error) {
	pk, err := solana.PublicKeyFromBase58(programID)
	if err != nil {
		return nil, fmt.Errorf("invalid program ID: %w", err)
	}

	idx := bytes.Index(embeddedProgram, placeholderBytes)
	if idx < 0 {
		return nil, fmt.Errorf("placeholder not found in embedded program binary")
	}

	patched := make([]byte, len(embeddedProgram))
	copy(patched, embeddedProgram)
	copy(patched[idx:idx+32], pk.Bytes())

	return patched, nil
}
