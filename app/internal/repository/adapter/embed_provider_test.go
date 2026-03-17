package adapter

import (
	"bytes"
	"testing"
)

func TestPatchedProgramBinary_PlaceholderNotFound(t *testing.T) {
	if !bytes.Contains(embeddedProgram, placeholderBytes) {
		t.Skip("embedded program is a placeholder file without marker bytes")
	}

	programID := "SoLKbzFRBKBMBFoYT17q7RMDVCKrEjU4r9QAaQ5bNqE"
	patched, err := PatchedProgramBinary(programID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bytes.Contains(patched, placeholderBytes) {
		t.Fatal("patched binary still contains placeholder bytes")
	}

	if bytes.Equal(patched, embeddedProgram) {
		t.Fatal("patched binary is identical to original")
	}

	if len(patched) != len(embeddedProgram) {
		t.Fatalf("patched binary length %d != original %d", len(patched), len(embeddedProgram))
	}
}

func TestPatchedProgramBinary_InvalidProgramID(t *testing.T) {
	_, err := PatchedProgramBinary("not-a-valid-base58")
	if err == nil {
		t.Fatal("expected error for invalid program ID")
	}
}

func TestPatchedProgramBinary_OriginalUnchanged(t *testing.T) {
	if !bytes.Contains(embeddedProgram, placeholderBytes) {
		t.Skip("embedded program is a placeholder file without marker bytes")
	}

	original := make([]byte, len(embeddedProgram))
	copy(original, embeddedProgram)

	programID := "SoLKbzFRBKBMBFoYT17q7RMDVCKrEjU4r9QAaQ5bNqE"
	_, err := PatchedProgramBinary(programID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.Equal(embeddedProgram, original) {
		t.Fatal("PatchedProgramBinary modified the original embedded binary")
	}
}
