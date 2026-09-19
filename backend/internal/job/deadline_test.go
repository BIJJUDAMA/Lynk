package job

import (
	"testing"
	"time"
)

func TestDeadlineCalendarDayPassed(t *testing.T) {
	deadline := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	noon := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	next := time.Date(2026, 9, 10, 0, 0, 1, 0, time.UTC)
	if DeadlineCalendarDayPassed(&deadline, noon) {
		t.Fatal("deadline day noon UTC must still be open")
	}
	if !DeadlineCalendarDayPassed(&deadline, next) {
		t.Fatal("next UTC calendar day must be closed")
	}
	if DeadlineCalendarDayPassed(nil, noon) {
		t.Fatal("nil deadline never passes")
	}
}

func TestCalendarDayUTC(t *testing.T) {
	// Midday time in non-UTC timezone
	loc := time.FixedZone("EDT", -4*3600)
	dt := time.Date(2026, 9, 11, 22, 30, 0, 0, loc) // 2026-09-12 02:30:00 UTC
	cal := CalendarDayUTC(dt)
	expected := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	if !cal.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, cal)
	}
}
