package domain

import "fmt"

type EntrySchema struct {
	Type   EntryType
	Fields []FieldDef
}

func (s *EntrySchema) RequiredKeys() []string {
	var keys []string
	for _, f := range s.Fields {
		if f.Required {
			keys = append(keys, f.Key)
		}
	}
	return keys
}

func (s *EntrySchema) SensitiveKeys() []string {
	var keys []string
	for _, f := range s.Fields {
		if f.Sensitive {
			keys = append(keys, f.Key)
		}
	}
	return keys
}

func (s *EntrySchema) TOTPKey() string {
	for _, f := range s.Fields {
		if f.IsTOTP {
			return f.Key
		}
	}
	return ""
}

func (s *EntrySchema) HasKey(key string) bool {
	for _, f := range s.Fields {
		if f.Key == key {
			return true
		}
	}
	return false
}

func (s *EntrySchema) ValidateFields(fields map[string]string) error {
	for _, f := range s.Fields {
		if f.Required && fields[f.Key] == "" {
			return fmt.Errorf("%s is required", f.Label)
		}
	}
	return nil
}

var schemas = map[EntryType]*EntrySchema{
	EntryTypePassword: {
		Type: EntryTypePassword,
		Fields: []FieldDef{
			{Key: "site", Label: "Site/URL"},
			{Key: "username", Label: "Username"},
			{Key: "password", Label: "Password", Sensitive: true, Generable: true},
			{Key: "totp_secret", Label: "TOTP Secret", Sensitive: true, IsTOTP: true},
			{Key: "notes", Label: "Notes"},
		},
	},
	EntryTypeTOTP: {
		Type: EntryTypeTOTP,
		Fields: []FieldDef{
			{Key: "site", Label: "Site"},
			{Key: "secret", Label: "Secret", Required: true, Sensitive: true, IsTOTP: true},
			{Key: "digits", Label: "Digits"},
			{Key: "period", Label: "Period (sec)"},
		},
	},
	EntryTypeNote: {
		Type: EntryTypeNote,
		Fields: []FieldDef{
			{Key: "content", Label: "Content", Required: true},
		},
	},
	EntryTypeCard: {
		Type: EntryTypeCard,
		Fields: []FieldDef{
			{Key: "cardholder", Label: "Cardholder"},
			{Key: "number", Label: "Card Number", Required: true, Sensitive: true},
			{Key: "expiry", Label: "Expiry"},
			{Key: "cvv", Label: "CVV", Sensitive: true},
			{Key: "notes", Label: "Notes"},
		},
	},
}

func SchemaFor(t EntryType) *EntrySchema {
	if s, ok := schemas[t]; ok {
		return s
	}
	return nil
}
