package money

import "testing"

func TestCentsRoundTrip(t *testing.T) {
	if DollarsNumericLiteral(1250) != "12.50" {
		t.Fatalf("expected 12.50, got %s", DollarsNumericLiteral(1250))
	}
	c, err := ParseCents(12_50)
	if err != nil || c != 1250 {
		t.Fatalf("ParseCents: %v %d", err, c)
	}
}

func TestRejectNegativeCents(t *testing.T) {
	if err := ValidateNonNegative(-1); err == nil {
		t.Fatal("expected error")
	}
}
