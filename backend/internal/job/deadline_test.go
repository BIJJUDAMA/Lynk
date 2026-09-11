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
