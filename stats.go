package main

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// avgEpisodeMinutes is a rough average episode length, used only to turn
// episode counts into an approximate watch-time estimate.
const avgEpisodeMinutes = 20

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
	watchedHours := float64(totalEpisodes*avgEpisodeMinutes) / 60
	fmt.Fprintln(sb, fieldLabelStyle.Render("time watched:")+fieldValueStyle.Render(fmt.Sprintf("~%.1fh", watchedHours)))
	fmt.Fprintln(sb, fieldLabelStyle.Render("rewatches:  ")+fieldValueStyle.Render(fmt.Sprintf("%d", totalRewatches)))
	avgRating := "–"
	if ratedCount > 0 {
		avgRating = fmt.Sprintf("%.2f (%d rated)", ratingSum/float64(ratedCount), ratedCount)
	}
	fmt.Fprintln(sb, fieldLabelStyle.Render("avg rating: ")+fieldValueStyle.Render(avgRating))

	if favorites := topFavorites(entries, 5); len(favorites) > 0 {
		fmt.Fprintln(sb)
		fmt.Fprintln(sb, fieldLabelStyle.Render("top favorites:"))
		for i, a := range favorites {
			fmt.Fprintln(sb, fieldValueStyle.Render(fmt.Sprintf("  %d. %s (%.1f, %dx rewatch)", i+1, a.Title, *a.Rating, a.TotalRewatch())))
		}
	}

	fmt.Fprintln(sb)
	fmt.Fprint(sb, helpStyle.Render("any key to close"))
	return sb.String()
}

// topFavorites returns the n highest-rated entries, ranked by rating first
// and total rewatches as a tiebreaker; unrated entries never qualify.
func topFavorites(entries []Anime, n int) []Anime {
	rated := make([]Anime, 0, len(entries))
	for _, a := range entries {
		if a.Rating != nil {
			rated = append(rated, a)
		}
	}
	slices.SortFunc(rated, func(a, b Anime) int {
		if c := compareRatingDesc(a.Rating, b.Rating); c != 0 {
			return c
		}
		return b.TotalRewatch() - a.TotalRewatch()
	})
	if len(rated) > n {
		rated = rated[:n]
	}
	return rated
}
