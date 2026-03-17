package tui

import (
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

func generateTOTP(secret string, digits int, period int) (string, int, error) {
	if digits == 0 {
		digits = 6
	}
	if period == 0 {
		period = 30
	}

	d := otp.DigitsSix
	if digits == 8 {
		d = otp.DigitsEight
	}

	code, err := totp.GenerateCodeCustom(secret, time.Now(), totp.ValidateOpts{
		Digits: d,
		Period: uint(period),
	})
	if err != nil {
		return "", 0, err
	}

	remaining := period - int(time.Now().Unix()%int64(period))
	return code, remaining, nil
}

