package main

import (
	"image/color"
	"slices"

	"charm.land/lipgloss/v2"
)

type Status string

const (
	StatusWatching    Status = "watching"      // w
	StatusCompleted   Status = "completed"     // c
	StatusPlanToWatch Status = "plan to watch" // p
	StatusPaused      Status = "paused"        // pa
	StatusDropped     Status = "dropped"       // d
	StatusRewatching  Status = "rewatching"    // r
)

func StatusList() []Status {
	return []Status{
		StatusWatching,
		StatusCompleted,
		StatusPlanToWatch,
		StatusPaused,
		StatusDropped,
		StatusRewatching,
	}
}

func (s Status) String() string {
	if s == "" {
		return string(StatusPlanToWatch)
	}
	return string(s)
}

func (s Status) Symbol() string {
	switch s {
	case StatusCompleted:
		return "✓"
	case StatusWatching:
		return "▶"
	case StatusPlanToWatch:
		return "·"
	case StatusDropped:
		return "✗"
	case StatusPaused:
		return "!"
	case StatusRewatching:
		return "↺"
	default:
		return "?"
	}
}

// Color returns the status's foreground color, used to color-code it
// wherever it's shown (the list, and the select/filter popups).
func (s Status) Color() color.Color {
	switch s {
	case StatusCompleted:
		return lipgloss.Color("2") // green
	case StatusWatching:
		return lipgloss.Color("12") // blue
	case StatusPlanToWatch:
		return lipgloss.Color("8") // gray
	case StatusPaused:
		return lipgloss.Color("3") // yellow
	case StatusDropped:
		return lipgloss.Color("1") // red
	case StatusRewatching:
		return lipgloss.Color("5") // magenta
	default:
		return lipgloss.Color("7")
	}
}

func (s Status) Next() Status {
	list := StatusList()
	idx := slices.Index(list, s)
	if idx == -1 {
		return list[0]
	}
	return list[(idx+1)%len(list)]
}

func (s Status) Prev() Status {
	list := StatusList()
	idx := slices.Index(list, s)
	if idx == -1 {
		return list[0]
	}
	return list[(idx-1+len(list))%len(list)]
}

func ParseStatus(value string) (s Status, valid bool) {
	if slices.Contains(StatusList(), Status(value)) {
		return Status(value), true
	}
	return "", false
}
