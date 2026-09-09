package money

import (
	"fmt"
)

// Cents represents a monetary amount as integer cents (1/100 of a dollar).
type Cents int64

func ValidateNonNegative(c Cents) error {
	if c < 0 {
		return fmt.Errorf("money cents must be >= 0")
	}
	if c > 9999999999 { // $99,999,999.99
		return fmt.Errorf("money cents exceed maximum")
	}
	return nil
}

// ParseCents validates and returns a Cents value from an integer cent amount.
func ParseCents(v int64) (Cents, error) {
	c := Cents(v)
	if err := ValidateNonNegative(c); err != nil {
		return 0, err
	}
	return c, nil
}

// DollarsNumericLiteral formats cents as a PostgreSQL NUMERIC dollar literal string.
func DollarsNumericLiteral(c Cents) string {
	neg := c < 0
	if neg {
		c = -c
	}
	dollars := c / 100
	frac := c % 100
	s := fmt.Sprintf("%d.%02d", dollars, frac)
	if neg {
		return "-" + s
	}
	return s
}

func FromFloatDollarsForbidden(f float64) Cents {
	// Do not call from handlers. Exists only to document why floats are banned.
	_ = f
	return 0
}
