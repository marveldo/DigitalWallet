package shared

import (
	"encoding/json"
	"testing"
)

func TestMoneyRoundTrip(t *testing.T) {
	m := NewMoney(12.34)
	if m.Minor() != 1234 {
		t.Fatalf("Minor() = %d, want 1234", m.Minor())
	}
	if m.String() != "12.34" {
		t.Fatalf("String() = %q, want 12.34", m.String())
	}

	v, _ := m.Value()
	var back Money
	if err := back.Scan(v); err != nil {
		t.Fatal(err)
	}
	if back != m {
		t.Fatalf("scan back = %v, want %v", back, m)
	}
}

func TestMoneyExactArithmetic(t *testing.T) {
	// The float trap this type exists to avoid: 0.1 + 0.2 != 0.3
	got := NewMoney(0.1).Add(NewMoney(0.2))
	if got != NewMoney(0.3) {
		t.Fatalf("0.1 + 0.2 = %v, want 0.30", got)
	}
}

func TestMoneyJSON(t *testing.T) {
	out, err := json.Marshal(struct{ Amount Money }{NewMoney(99.05)})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"Amount":99.05}` {
		t.Fatalf("marshal = %s", out)
	}

	var in struct{ Amount Money }
	if err := json.Unmarshal([]byte(`{"Amount":"7.5"}`), &in); err != nil {
		t.Fatal(err)
	}
	if in.Amount.Minor() != 750 {
		t.Fatalf("unmarshal = %d, want 750", in.Amount.Minor())
	}
}

func TestMoneyNegative(t *testing.T) {
	if s := NewMoney(-3.05).String(); s != "-3.05" {
		t.Fatalf("String() = %q, want -3.05", s)
	}
}
