package service

import (
	"testing"
)

func TestDeriveKeysDeterministic(t *testing.T) {
	d := NewKeyDeriver()

	k1, err := d.Derive("testpassword")
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	k2, err := d.Derive("testpassword")
	if err != nil {
		t.Fatalf("derive: %v", err)
	}

	if k1.DeployerAddress != k2.DeployerAddress {
		t.Error("deployer address should be deterministic")
	}
	if k1.ProgramID != k2.ProgramID {
		t.Error("program ID should be deterministic")
	}
}

func TestDeriveKeysDifferentPasswords(t *testing.T) {
	d := NewKeyDeriver()

	k1, _ := d.Derive("password1")
	k2, _ := d.Derive("password2")

	if k1.DeployerAddress == k2.DeployerAddress {
		t.Error("different passwords should give different deployer")
	}
	if k1.ProgramID == k2.ProgramID {
		t.Error("different passwords should give different program")
	}
}

func TestDeriveKeysEncryptionKey(t *testing.T) {
	d := NewKeyDeriver()

	k, _ := d.Derive("test")
	if len(k.EncryptionKey) != 32 {
		t.Errorf("expected 32 byte key, got %d", len(k.EncryptionKey))
	}
}

func TestDeriveKeysZero(t *testing.T) {
	d := NewKeyDeriver()
	k, _ := d.Derive("test")

	k.Zero()

	for _, b := range k.EncryptionKey {
		if b != 0 {
			t.Fatal("encryption key should be zeroed")
		}
	}
	for _, b := range k.DeployerKeypair {
		if b != 0 {
			t.Fatal("deployer keypair should be zeroed")
		}
	}
}
