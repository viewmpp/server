package user

import (
	"net/mail"
	"server/internal/validator"
	"strings"
	"unicode/utf8"
)

const (
	MinPasswordLength = 8
	MaxPasswordLength = 72
	MaxEmailLength    = 254
)

func CheckEmail(v *validator.Validator, field, value string) {
	v.Check(value != "", field, MsgEmailRequired)
	if value == "" {
		return
	}

	v.Check(len(value) <= MaxEmailLength, field, MsgEmailTooLong)

	address, err := mail.ParseAddress(value)
	v.Check(err == nil && address.Address == value, field, MsgEmailInvalid)

}

func CheckPassword(v *validator.Validator, field, value string) {
	v.Check(value != "", field, MsgPassRequired)
	if value == "" {
		return
	}

	v.Check(utf8.RuneCountInString(value) >= MinPasswordLength, field,
		MsgPassTooShort())
	v.Check(len(value) <= MaxPasswordLength, field,
		MsgPassTooLong)
}

func NormalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func NormalizeCode(value string) string {
	return strings.ToUpper(strings.Join(strings.Fields(value), ""))
}
