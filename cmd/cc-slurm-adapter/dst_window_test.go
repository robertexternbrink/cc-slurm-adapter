package main

import (
	"testing"
	"time"
)

func TestSacctAutumnDSTStartMovesBackwards(t *testing.T) {
	zone, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	begin := time.Date(2026, time.October, 25, 0, 50, 0, 0, time.UTC).In(zone)
	end := time.Date(2026, time.October, 25, 1, 10, 0, 0, time.UTC).In(zone)
	got := sacctQueryStart(begin, end)
	if !got.Equal(begin.Add(-time.Hour)) || got.Format("2006-01-02T15:04:05") > end.Format("2006-01-02T15:04:05") {
		t.Fatalf("invalid local time range: %s -> %s (original start %s)", got, end, begin)
	}
	for _, interval := range [][2]time.Time{{begin.Add(-24 * time.Hour), begin.Add(-23 * time.Hour)}, {end.Add(24 * time.Hour), end.Add(25 * time.Hour)}, {time.Date(2026, 3, 29, 0, 50, 0, 0, time.UTC).In(zone), time.Date(2026, 3, 29, 1, 10, 0, 0, time.UTC).In(zone)}} {
		if got := sacctQueryStart(interval[0], interval[1]); !got.Equal(interval[0]) {
			t.Fatalf("non-autumn interval changed: %s", got)
		}
	}
}
