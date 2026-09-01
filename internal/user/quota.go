package user

import (
	"errors"
	"fmt"
)

const earlyAccessCampaign = "early_access"

var (
	ErrSaveLimit       = errors.New("save quota exceeded")
	ErrShareLimit      = errors.New("share quota exceeded")
	ErrShareUnverified = errors.New("sharing requires verification")
	ErrSeatLimit       = errors.New("seat quota exceeded")
)

type QuotaError struct {
	Kind  error
	Used  int
	Limit int
}

func (e *QuotaError) Error() string {
	return fmt.Sprintf("%s: %d of %d used", e.Kind, e.Used, e.Limit)
}

func (e *QuotaError) Unwrap() error {
	return e.Kind
}
