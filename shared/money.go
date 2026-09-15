package shared

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

// Money is a monetary amount stored as minor units (kobo, cents).
//
// It is an int64 so arithmetic is exact - never use float64 for money, since
// 0.1 + 0.2 != 0.3 in binary floating point and cents silently disappear.
//
// The conversion between minor units (database, payment providers) and major
// units (JSON, humans) happens automatically:
//
//	DB   -> bigint minor units, via Scan/Value
//	JSON -> decimal major units, via MarshalJSON/UnmarshalJSON
//
// So a wallet field is simply:
//
//	Balance shared.Money `gorm:"type:bigint;not null;default:0"`
//
// and no BeforeCreate/AfterCreate hooks are needed anywhere.
type Money int64

// minorUnits is the number of minor units in one major unit. All currencies we
// support are two-decimal; if a zero-decimal currency (JPY) is ever settled on
// chain, give Money a Currency and switch on it here.
const minorUnits = 100

// NewMoney builds a Money from major units, e.g. NewMoney(12.34) -> 1234.
// The value is rounded half-away-from-zero to the nearest minor unit.
func NewMoney(major float64) Money {
	return Money(math.Round(major * minorUnits))
}

// NewMoneyFromMinor builds a Money from a raw minor-unit amount, e.g. from a
// payment provider webhook that already speaks kobo.
func NewMoneyFromMinor(minor int64) Money { return Money(minor) }

// ParseMoney parses a decimal major-unit string such as "12.34".
func ParseMoney(s string) (Money, error) {
	major, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q: %w", s, err)
	}
	return NewMoney(major), nil
}

func (m Money) Minor() int64 { return int64(m) }

func (m Money) Major() float64 { return float64(m) / minorUnits }

func (m Money) Add(other Money) Money { return m + other }

func (m Money) Sub(other Money) Money { return m - other }

func (m Money) IsZero() bool     { return m == 0 }
func (m Money) IsPositive() bool { return m > 0 }
func (m Money) IsNegative() bool { return m < 0 }

// String renders the amount with two decimals, e.g. "12.34".
func (m Money) String() string {
	sign := ""
	v := int64(m)
	if v < 0 {
		sign, v = "-", -v
	}
	return fmt.Sprintf("%s%d.%02d", sign, v/minorUnits, v%minorUnits)
}

// Value implements driver.Valuer: Money is persisted as bigint minor units.
func (m Money) Value() (driver.Value, error) { return int64(m), nil }

// Scan implements sql.Scanner, reading the bigint minor units back.
func (m *Money) Scan(value any) error {
	switch v := value.(type) {
	case nil:
		*m = 0
	case int64:
		*m = Money(v)
	case int32:
		*m = Money(v)
	case float64:
		// Some drivers hand back numerics as float64.
		*m = Money(math.Round(v))
	case []byte:
		parsed, err := strconv.ParseInt(string(v), 10, 64)
		if err != nil {
			return fmt.Errorf("scan money: %w", err)
		}
		*m = Money(parsed)
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("scan money: %w", err)
		}
		*m = Money(parsed)
	default:
		return fmt.Errorf("scan money: unsupported type %T", value)
	}
	return nil
}

// GormDataType tells GORM the column type when it builds the schema, so the
// `gorm:"type:bigint"` tag on every model field is belt and braces.
func (Money) GormDataType() string { return "bigint" }

// MarshalJSON emits major units so API clients see 12.34, not 1234.
func (m Money) MarshalJSON() ([]byte, error) {
	return []byte(m.String()), nil
}

// UnmarshalJSON accepts either a number (12.34) or a string ("12.34").
func (m *Money) UnmarshalJSON(data []byte) error {
	var major float64
	if err := json.Unmarshal(data, &major); err == nil {
		*m = NewMoney(major)
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("unmarshal money: %w", err)
	}
	parsed, err := ParseMoney(s)
	if err != nil {
		return err
	}
	*m = parsed
	return nil
}
