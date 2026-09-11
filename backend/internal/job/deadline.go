package job

import "time"

func CalendarDayUTC(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

func DeadlineCalendarDayPassed(deadline *time.Time, now time.Time) bool {
	if deadline == nil {
		return false
	}
	return CalendarDayUTC(now).After(CalendarDayUTC(*deadline))
}
