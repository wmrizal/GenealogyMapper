package ui

import (
	"testing"
	"time"
)

func TestCalendarDatePreservesLocalCalendarDay(t *testing.T) {
	localDate := time.Date(2026, time.September, 7, 0, 0, 0, 0, time.FixedZone("UTC+5", 5*60*60))

	got := calendarDate(localDate)
	want := time.Date(2026, time.September, 7, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("calendarDate() = %v, want %v", got, want)
	}
}
