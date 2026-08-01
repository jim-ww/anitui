package main

import "testing"

func TestStatusNextPrev(t *testing.T) {
	tests := []struct {
		name string
		s    Status
		next Status
		prev Status
	}{
		{"watching", StatusWatching, StatusCompleted, StatusRewatching},
		{"rewatching wraps", StatusRewatching, StatusWatching, StatusDropped},
		{"unknown falls back to first", Status("bogus"), StatusList()[0], StatusList()[0]},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.Next(); got != tt.next {
				t.Errorf("Next() = %q, want %q", got, tt.next)
			}
			if got := tt.s.Prev(); got != tt.prev {
				t.Errorf("Prev() = %q, want %q", got, tt.prev)
			}
		})
	}
}

func TestParseStatus(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  Status
		valid bool
	}{
		{"valid", "completed", StatusCompleted, true},
		{"invalid", "finished", "", false},
		{"empty", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, valid := ParseStatus(tt.value)
			if got != tt.want || valid != tt.valid {
				t.Errorf("ParseStatus(%q) = (%q, %v), want (%q, %v)", tt.value, got, valid, tt.want, tt.valid)
			}
		})
	}
}
