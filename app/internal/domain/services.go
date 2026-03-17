package domain

type KeyDeriver interface {
	Derive(password string) (*DerivedKeys, error)
}

type CryptoService interface {
	Encrypt(data []byte) ([]byte, error)
	Decrypt(data []byte) ([]byte, error)
}
