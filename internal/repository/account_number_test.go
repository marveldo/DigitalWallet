package repository

import (
	"strconv"
	"testing"
)

func TestNewAccountNumberShape(t *testing.T) {
	for i := 0; i < 1000; i++ {
		got, err := NewAccountNumber()
		if err != nil {
			t.Fatalf("NewAccountNumber: %v", err)
		}
		if len(got) != AccountNumberLength {
			t.Fatalf("got %q, want %d digits", got, AccountNumberLength)
		}
		n, err := strconv.ParseInt(got, 10, 64)
		if err != nil {
			t.Fatalf("got %q, want digits only: %v", got, err)
		}
		if n < accountNumberMin || n >= accountNumberMin+accountNumberSpan {
			t.Fatalf("got %d, want it inside [%d, %d)", n, accountNumberMin, accountNumberMin+accountNumberSpan)
		}
	}
}

// A repeat inside a few thousand draws would mean the range or the source is
// far smaller than intended. It is not a uniqueness guarantee — that lives in
// the unique index — only a check that the generator is not obviously broken.
func TestNewAccountNumberDoesNotRepeatQuickly(t *testing.T) {
	seen := make(map[string]struct{}, 5000)
	for i := 0; i < 5000; i++ {
		got, err := NewAccountNumber()
		if err != nil {
			t.Fatalf("NewAccountNumber: %v", err)
		}
		if _, dup := seen[got]; dup {
			t.Fatalf("drew %q twice within %d draws", got, i+1)
		}
		seen[got] = struct{}{}
	}
}
