package require

import (
	"reflect"
	"testing"
)

type ErrorAssertionFunc func(t *testing.T, err error)

func NoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func Error(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error, got nil")
	}
}

func Equal(t *testing.T, want, got any) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("unexpected value: want:\n%v, got:\n%v", want, got)
	}
}
