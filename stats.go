package main

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m Model) updateStats(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.KeyPressMsg); ok {
		m.mode = modeList
	}
	return m, nil
}

// statsView summarizes the full library (ignoring the current status
// filter): a count per status, total episodes watched, average rating, and
// total rewatches.
func statsView(entries []Anime) string {
	counts := make(map[Status]int)
	totalEpisodes := 0
	totalRewatches := 0
	var ratingSum float64
	ratedCount := 0

	for _, a := range entries {
		counts[a.Status]++
		if a.Progress != nil {
			totalEpisodes += *a.Progress
		}
		totalRewatches += a.TotalRewatch()
		if a.Rating != nil {
			ratingSum += float64(*a.Rating)
			ratedCount++
		}
	}

	sb := new(strings.Builder)
	fmt.Fprintln(sb, titleStyle.Render("Stats"))
	fmt.Fprintln(sb)
	fmt.Fprintln(sb, fieldLabelStyle.Render("total:      ")+fieldValueStyle.Render(fmt.Sprintf("%d", len(entries))))
	for _, s := range StatusList() {
		if counts[s] == 0 {
			continue
		}
		label := fmt.Sprintf("%-12s", s.String()+":")
		fmt.Fprintln(sb, fieldLabelStyle.Render(label)+fieldValueStyle.Render(fmt.Sprintf("%d", counts[s])))
	}
	fmt.Fprintln(sb)
	fmt.Fprintln(sb, fieldLabelStyle.Render("episodes:   ")+fieldValueStyle.Render(fmt.Sprintf("%d", totalEpisodes)))
	fmt.Fprintln(sb, fieldLabelStyle.Render("rewatches:  ")+fieldValueStyle.Render(fmt.Sprintf("%d", totalRewatches)))
	avgRating := "–"
	if ratedCount > 0 {
		avgRating = fmt.Sprintf("%.2f (%d rated)", ratingSum/float64(ratedCount), ratedCount)
	}
	fmt.Fprintln(sb, fieldLabelStyle.Render("avg rating: ")+fieldValueStyle.Render(avgRating))

	fmt.Fprintln(sb)
	fmt.Fprint(sb, helpStyle.Render("any key to close"))
	return sb.String()
}
