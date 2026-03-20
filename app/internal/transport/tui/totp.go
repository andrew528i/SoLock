package tui

func (a *App) generateTOTP(secret string, digits int, period int) (string, int, error) {
	result, err := a.app.GenerateTOTP.Execute(secret, digits, period)
	if err != nil {
		return "", 0, err
	}
	return result.Code, result.Remaining, nil
}
