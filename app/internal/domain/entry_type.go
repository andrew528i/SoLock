package domain

type EntryType string

const (
	EntryTypePassword EntryType = "password"
	EntryTypeTOTP     EntryType = "totp"
	EntryTypeNote     EntryType = "note"
	EntryTypeCard     EntryType = "card"
)

var CreatableEntryTypes = []EntryType{
	EntryTypePassword,
	EntryTypeNote,
	EntryTypeCard,
}

func (t EntryType) Label() string {
	switch t {
	case EntryTypePassword:
		return "Password"
	case EntryTypeTOTP:
		return "TOTP"
	case EntryTypeNote:
		return "Note"
	case EntryTypeCard:
		return "Card"
	default:
		return string(t)
	}
}

func (t EntryType) ShortLabel() string {
	switch t {
	case EntryTypePassword:
		return "PASS"
	case EntryTypeTOTP:
		return "TOTP"
	case EntryTypeNote:
		return "NOTE"
	case EntryTypeCard:
		return "CARD"
	default:
		return string(t)
	}
}

func (t EntryType) IsValid() bool {
	switch t {
	case EntryTypePassword, EntryTypeTOTP, EntryTypeNote, EntryTypeCard:
		return true
	default:
		return false
	}
}
