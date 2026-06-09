package model

import "slices"

type Status string

const (
	StatusWatching    Status = "watching"
	StatusCompleted   Status = "completed"
	StatusPlanToWatch Status = "plan to watch"
	StatusPaused      Status = "paused"
	StatusDropped     Status = "dropped"
	StatusRewatching  Status = "rewatching"
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

func ParseStatus(value string) (s Status, valid bool) {
	switch value {
	case string(StatusWatching):
		return StatusWatching, true
	case string(StatusCompleted):
		return StatusCompleted, true
	case string(StatusPlanToWatch):
		return StatusPlanToWatch, true
	case string(StatusPaused):
		return StatusPaused, true
	case string(StatusDropped):
		return StatusDropped, true
	case string(StatusRewatching):
		return StatusRewatching, true
	default:
		return "", false
	}
}
