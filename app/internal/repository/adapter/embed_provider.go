package adapter

import (
	"bytes"
	"encoding/binary"
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

	pkBytes := pk.Bytes()

	copy(patched[idx:idx+32], pkBytes)

	patchLddwInstructions(patched, placeholderBytes, pkBytes)

	return patched, nil
}

func patchLddwInstructions(data []byte, oldID, newID []byte) {
	var oldChunks [4][8]byte
	var newChunks [4][8]byte
	for i := 0; i < 4; i++ {
		copy(oldChunks[i][:], oldID[i*8:(i+1)*8])
		copy(newChunks[i][:], newID[i*8:(i+1)*8])
	}

	for i := 0; i+16 <= len(data); i += 8 {
		if data[i] != 0x18 {
			continue
		}

		immLo := binary.LittleEndian.Uint32(data[i+4 : i+8])
		immHi := binary.LittleEndian.Uint32(data[i+12 : i+16])

		var loaded [8]byte
		binary.LittleEndian.PutUint32(loaded[:4], immLo)
		binary.LittleEndian.PutUint32(loaded[4:], immHi)

		for ci := 0; ci < 4; ci++ {
			if loaded == oldChunks[ci] {
				copy(data[i+4:i+8], newChunks[ci][:4])
				copy(data[i+12:i+16], newChunks[ci][4:])
				break
			}
		}
	}
}
