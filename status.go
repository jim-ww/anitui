package main

import "slices"

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
