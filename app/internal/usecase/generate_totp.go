package usecase

import (
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

type TOTPResult struct {
	Code      string
	Remaining int
}

type GenerateTOTPUseCase struct{}

func NewGenerateTOTPUseCase() *GenerateTOTPUseCase {
	return &GenerateTOTPUseCase{}
}

func (uc *GenerateTOTPUseCase) Execute(secret string, digits int, period int) (*TOTPResult, error) {
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
		return nil, err
	}

	remaining := period - int(time.Now().Unix()%int64(period))
	return &TOTPResult{Code: code, Remaining: remaining}, nil
}
