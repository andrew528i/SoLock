package domain

type DerivedKeys struct {
	DeployerKeypair  []byte
	DeployerAddress  string
	DeployerMnemonic string
	ProgramKeypair   []byte
	ProgramID        string
	ProgramMnemonic  string
	EncryptionKey    []byte
}

func (k *DerivedKeys) Zero() {
	for i := range k.EncryptionKey {
		k.EncryptionKey[i] = 0
	}
	for i := range k.DeployerKeypair {
		k.DeployerKeypair[i] = 0
	}
	for i := range k.ProgramKeypair {
		k.ProgramKeypair[i] = 0
	}
}
