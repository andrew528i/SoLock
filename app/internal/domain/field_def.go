package domain

type FieldDef struct {
	Key       string
	Label     string
	Required  bool
	Sensitive bool
	Generable bool
	IsTOTP    bool
}
