package main

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	tea "charm.land/bubbletea/v2"
)

// updateInfo handles the read-only detail popup ("i"). Any key closes it —
// it exists purely to look, not to edit.
func (m Model) updateInfo(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.KeyPressMsg); ok {
		m.mode = modeList
	}
	return m, nil
}

func infoView(a Anime) string {
	sb := new(strings.Builder)
	fmt.Fprintln(sb, titleStyle.Render(a.Title))
	fmt.Fprintln(sb)
	fmt.Fprintln(sb, fieldLabelStyle.Render("status:   ")+lipgloss.NewStyle().Foreground(a.Status.Color()).Render(a.Status.Symbol()+" "+a.Status.String()))
	fmt.Fprintln(sb, fieldLabelStyle.Render("progress: ")+fieldValueStyle.Render(fmt.Sprintf("ep %d", a.Progress)))
	fmt.Fprintln(sb, fieldLabelStyle.Render("rating:   ")+fieldValueStyle.Render(ratingLabel(a.Rating)))
	fmt.Fprintln(sb, fieldLabelStyle.Render("started:  ")+fieldValueStyle.Render(dateLabel(a.StartedAt())))
	fmt.Fprintln(sb, fieldLabelStyle.Render("last:     ")+fieldValueStyle.Render(dateLabel(a.LastWatch())))
	fmt.Fprintln(sb, fieldLabelStyle.Render("finished: ")+fieldValueStyle.Render(dateLabel(a.FinishedAt())))
	fmt.Fprintln(sb, fieldLabelStyle.Render("rewatches:")+fieldValueStyle.Render(fmt.Sprintf("%d", a.TotalRewatch())))
	fmt.Fprintln(sb)

	fmt.Fprintln(sb, fieldLabelStyle.Render("watch history:"))
	if len(a.WatchSessions) == 0 {
		fmt.Fprintln(sb, dimStyle.Render("  (none)"))
	}
	for i, session := range a.WatchSessions {
		label := fmt.Sprintf("  watch %d:", i+1)
		if i > 0 {
			label = fmt.Sprintf("  rewatch %d:", i)
		}
		dates := make([]string, len(session))
		for j, d := range session {
			dates[j] = d.Format(DateDisplayFormat)
		}
		fmt.Fprintln(sb, fieldValueStyle.Render(label+" "+strings.Join(dates, ", ")))
	}

	if a.Notes != "" {
		fmt.Fprintln(sb)
		fmt.Fprintln(sb, fieldLabelStyle.Render("notes:"))
		fmt.Fprintln(sb, dimStyle.Render("  "+a.Notes))
	}

	fmt.Fprintln(sb)
	fmt.Fprint(sb, helpStyle.Render("any key to close"))
	return sb.String()
}
