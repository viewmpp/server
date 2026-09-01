package contract

import (
	"fmt"
	"math"
	"unicode/utf8"
)

const (
	MaxBytes              = 8 << 20
	MaxTasks              = 10_000
	MaxResources          = 10_000
	MaxRelations          = 20_000
	MaxAssignments        = 20_000
	MaxCalendarExceptions = 1_000
	MaxAssignmentsPerTask = 25
	MaxRelationsPerTask   = 25
	MaxTextRunes          = 255
	MaxNotesRunes         = 32_767
	MaxTokenRunes         = 32
	MaxOutlineLevel       = 65_535
	MaxID                 = 1<<31 - 1
	MaxAssignmentUnits    = 6_000_000_000
	MaxNumber             = 9_999_999_999_999.99
)

func checkCount(field string, got, limit int) error {
	if got > limit {
		return fmt.Errorf("%s contains %d items, limit is %d", field, got, limit)
	}
	return nil
}

func checkString(field string, value *string, limit int) error {
	if value == nil {
		return nil
	}
	return checkRequiredString(field, *value, limit)
}

func checkRequiredString(field, value string, limit int) error {
	if n := utf8.RuneCountInString(value); n > limit {
		return fmt.Errorf("%s contains %d characters, limit is %d", field, n, limit)
	}
	return nil
}

func checkInteger(field string, value int64, min int64) error {
	if value < min || value > MaxID {
		return fmt.Errorf("%s is %d, range is %d..%d", field, value, min, MaxID)
	}
	return nil
}

func checkNumber(field string, value, min, max float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < min || value > max {
		return fmt.Errorf("%s is %g, range is %g..%g", field, value, min, max)
	}
	return nil
}
