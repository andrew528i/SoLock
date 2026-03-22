package domain

type GroupAccount struct {
	Index         uint32
	EncryptedData []byte
	Deleted       bool
}
