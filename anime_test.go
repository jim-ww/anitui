package main

import (
	"testing"
	"time"
)

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse(DateDisplayFormat, s)
	if err != nil {
		t.Fatalf("parsing date %q: %v", s, err)
	}
	return d
}

func dates(t *testing.T, ss ...string) []time.Time {
	t.Helper()
	out := make([]time.Time, len(ss))
	for i, s := range ss {
		out[i] = mustDate(t, s)
	}
	return out
}

func TestNextExpectedReleaseUsesMostRecentGap(t *testing.T) {
	tests := []struct {
		name  string
		dates []string
		want  string
	}{
		{
			// Regression: an 8-day gap ends the session, but an earlier
			// smaller-gap anchor (weekly-projection) predicted 08-28,
			// which was still too early in practice.
			name:  "irregular cadence ending in a wide gap",
			dates: []string{"2026-07-12", "2026-07-18", "2026-07-24", "2026-07-30", "2026-08-07", "2026-08-14", "2026-08-22"},
			want:  "2026-08-30",
		},
		{
			// Regression: anchoring on the single 7-day gap predicted
			// 08-13, which was still too early in practice.
			name:  "short session ending in a wide gap",
			dates: []string{"2026-07-18", "2026-07-23", "2026-07-30", "2026-08-08"},
			want:  "2026-08-17",
		},
		{
			name:  "clean weekly cadence",
			dates: []string{"2026-08-01", "2026-08-08", "2026-08-15"},
			want:  "2026-08-22",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextExpectedRelease(dates(t, tt.dates...))
			if got == nil {
				t.Fatalf("nextExpectedRelease(%v) = nil, want %s", tt.dates, tt.want)
			}
			if want := mustDate(t, tt.want); !got.Equal(want) {
				t.Errorf("nextExpectedRelease(%v) = %s, want %s", tt.dates, got.Format(DateDisplayFormat), tt.want)
			}
		})
	}
}

func TestNextExpectedReleaseNoPattern(t *testing.T) {
	tests := []struct {
		name  string
		dates []string
	}{
		{"empty", nil},
		{"single date", []string{"2026-08-01"}},
		{"binge catch-up, no gap long enough", []string{"2026-08-01", "2026-08-02", "2026-08-03"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ds []time.Time
			if tt.dates != nil {
				ds = dates(t, tt.dates...)
			}
			if got := nextExpectedRelease(ds); got != nil {
				t.Errorf("nextExpectedRelease(%v) = %s, want nil", tt.dates, got.Format(DateDisplayFormat))
			}
		})
	}
}
