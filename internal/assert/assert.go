package assert

import "testing"

func Equal[T comparable](t *testing.T, actual, expected T) {
	t.Helper()
	if expected != actual {
		t.Errorf("actual: %v, expected: %v\n", actual, expected)
	}
}

func NilError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %s", err.Error())
	}
}
