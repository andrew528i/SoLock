package domain

type PasswordGenConfig struct {
	Length     int  `json:"length"`
	Uppercase  bool `json:"uppercase"`
	Digits     bool `json:"digits"`
	Special    bool `json:"special"`
}

func DefaultPasswordGenConfig() *PasswordGenConfig {
	return &PasswordGenConfig{
		Length:    20,
		Uppercase: true,
		Digits:    true,
		Special:   true,
	}
}
