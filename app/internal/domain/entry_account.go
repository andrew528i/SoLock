package domain

type EntryAccount struct {
	Index         uint32
	EncryptedData []byte
	CreatedAt     int64
	UpdatedAt     int64
}
