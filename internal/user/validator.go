package user

import (
	"net/mail"
	"strings"
	"unicode/utf8"
)

const (
	MinPasswordLength = 8
	MaxPasswordLength = 72
	MaxEmailLength    = 254
)

type Validator struct {
	FieldErrors map[string]string
}

func NewValidator() *Validator {
	return &Validator{FieldErrors: map[string]string{}}
}

func (v *Validator) Valid() bool {
	return len(v.FieldErrors) == 0
}

func (v *Validator) add(field, message string) {
	if _, exists := v.FieldErrors[field]; !exists {
		v.FieldErrors[field] = message
	}
}

func (v *Validator) Check(ok bool, field, message string) {
	if !ok {
		v.add(field, message)
	}
}

func (v *Validator) CheckEmail(field, value string) {
	v.Check(value != "", field, MsgEmailRequired)
	if value == "" {
		return
	}

	v.Check(len(value) <= MaxEmailLength, field, MsgEmailTooLong)

	address, err := mail.ParseAddress(value)
	v.Check(err == nil && address.Address == value, field, MsgEmailInvalid)

}

func (v *Validator) CheckPassword(field, value string) {
	v.Check(value != "", field, MsgPassRequired)
	if value == "" {
		return
	}

	v.Check(utf8.RuneCountInString(value) >= MinPasswordLength, field,
		MsgPassTooShort)
	v.Check(len(value) <= MaxPasswordLength, field,
		MsgPassTooLong)
}

func NormalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func NormalizeCode(value string) string {
	return strings.ToUpper(strings.Join(strings.Fields(value), ""))
}
