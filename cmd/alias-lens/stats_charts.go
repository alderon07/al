package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type statsChartStyles struct {
	accent lipgloss.Style
	text   lipgloss.Style
	muted  lipgloss.Style
	theme  Theme
}

func renderStatsOverview(m statsModel, rows []statsRow, inner int, styles statsChartStyles) string {
	var body strings.Builder
	bodyWidth := inner
	showCoverage := len(m.data.Aliases) > 0
	wideCoverage := showCoverage && inner >= 76
	if wideCoverage {
		bodyWidth -= 30
	}
	if len(rows) == 0 {
		period := statsPeriods[m.periodIndex]
		if period != "all" && hasUntimestampedUsage(m.data.Events) {
			body.WriteString(styles.accent.Render("Your alias history has no dates yet."))
			body.WriteString("\n")
			body.WriteString(styles.muted.Render("Start a new shell. Alias Lens will date future commands without changing how history looks."))
		} else {
			body.WriteString(styles.accent.Render("No uses in this window yet."))
			body.WriteString("\n")
			body.WriteString(styles.muted.Render("No matching aliases were found in terminal history."))
		}
	} else {
		visible := len(rows)
		if limit := m.height - 16; limit > 0 && visible > limit {
			visible = limit
		}
		if m.selected >= visible {
			m.selected = visible - 1
		}
		maxCount := rows[0].Count
		nameWidth := 16
		barWidth := bodyWidth - nameWidth - 17
		if barWidth < 8 {
			barWidth = 8
		}
		for index, row := range rows[:visible] {
			filled := row.Count * barWidth / maxCount
			if filled < 1 {
				filled = 1
			}
			marker := "  "
			nameStyle := styles.text
			barColor := styles.theme.Secondary
			if index == m.selected {
				marker = styles.accent.Render("› ")
				nameStyle = styles.accent
				barColor = styles.theme.Accent
			}
			bar := lipgloss.NewStyle().Foreground(lipgloss.Color(barColor)).Render(strings.Repeat("━", filled)) + styles.muted.Render(strings.Repeat("─", barWidth-filled))
			body.WriteString(fmt.Sprintf("%s%2d  %s %s %4d", marker, index+1, nameStyle.Render(fmt.Sprintf("%-*s", nameWidth, truncate(row.Alias.Name, nameWidth))), bar, row.Count))
			body.WriteString("\n")
		}
		selected := rows[m.selected].Alias
		body.WriteString("\n")
		body.WriteString(styles.muted.Render("expands to  "))
		body.WriteString(styles.text.Render(truncate(selected.Command, bodyWidth-12)))
		if len(selected.Tags) > 0 {
			body.WriteString("\n")
			body.WriteString(styles.muted.Render("tags        "))
			body.WriteString(styles.accent.Render(strings.Join(selected.Tags, "  ")))
		}
	}
	bodyContent := strings.TrimRight(body.String(), "\n")
	if showCoverage {
		sidebar := renderCoverageMap(len(rows), len(m.data.Aliases), styles.theme)
		if len(rows) > 0 {
			sidebar += "\n\n" + renderConcentration(rows, styles)
		}
		if wideCoverage {
			return lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().Width(bodyWidth).Render(bodyContent), strings.Repeat(" ", 3), sidebar)
		}
		return sidebar + "\n\n" + bodyContent
	}
	return bodyContent
}

func renderCoverageMap(used, total int, theme Theme) string {
	const columns, maxDots, width = 10, 100, 20
	coverage := 0
	if total > 0 {
		coverage = (used*100 + total/2) / total
	}
	dots, usedDots := total, used
	scaleNote := "1 dot = 1 alias"
	if dots > maxDots {
		dots = maxDots
		usedDots = (used*maxDots + total/2) / total
		scaleNote = "scaled to 100 dots"
	}
	rows := (dots + columns - 1) / columns
	usedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Accent))
	unusedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Muted))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Text)).Bold(true).Width(width).Align(lipgloss.Center)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Muted))
	lines := []string{labelStyle.Render(fmt.Sprintf("Alias coverage  %d%%", coverage))}
	for row := 0; row < rows; row++ {
		var line strings.Builder
		for column := 0; column < columns; column++ {
			index := row*columns + column
			switch {
			case index >= dots:
				line.WriteString("  ")
			case index < usedDots:
				line.WriteString(usedStyle.Render("● "))
			default:
				line.WriteString(unusedStyle.Render("○ "))
			}
		}
		lines = append(lines, strings.TrimRight(line.String(), " "))
	}
	lines = append(lines,
		usedStyle.Render(fmt.Sprintf("● %d used", used))+muted.Render("  ")+unusedStyle.Render(fmt.Sprintf("○ %d unused", total-used)),
		muted.Width(width).Align(lipgloss.Center).Render(scaleNote),
	)
	return strings.Join(lines, "\n")
}

func renderConcentration(rows []statsRow, styles statsChartStyles) string {
	total, leading := 0, 0
	limit := min(5, len(rows))
	for index, row := range rows {
		total += row.Count
		if index < limit {
			leading += row.Count
		}
	}
	percent := 0
	if total > 0 {
		percent = (leading*100 + total/2) / total
	}
	const width = 20
	filled := percent * width / 100
	bar := styles.accent.Render(strings.Repeat("━", filled)) + styles.muted.Render(strings.Repeat("─", width-filled))
	return styles.text.Bold(true).Render("Top-five share") + styles.accent.Render(fmt.Sprintf("  %d%%", percent)) + "\n" + bar + "\n" + styles.muted.Render(fmt.Sprintf("%d of %d runs", leading, total))
}

type activityDay struct {
	date  time.Time
	count int
}

func sevenDayActivity(events []usageEvent, now time.Time) []activityDay {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	days := make([]activityDay, 7)
	byDate := make(map[string]int, 7)
	for index := range days {
		days[index].date = today.AddDate(0, 0, index-6)
		byDate[days[index].date.Format("2006-01-02")] = index
	}
	for _, event := range events {
		if event.Time.IsZero() {
			continue
		}
		if index, ok := byDate[event.Time.In(now.Location()).Format("2006-01-02")]; ok {
			days[index].count++
		}
	}
	return days
}

func renderActivityHistogram(events []usageEvent, period string, now time.Time, width int, styles statsChartStyles) string {
	title, buckets := activityBuckets(events, period, now)
	total, activeBuckets, maximum := 0, 0, 0
	for _, bucket := range buckets {
		total += bucket.count
		if bucket.count > 0 {
			activeBuckets++
		}
		maximum = max(maximum, bucket.count)
	}
	var body strings.Builder
	bucketLabel := "active buckets"
	if activeBuckets == 1 {
		bucketLabel = "active bucket"
	}
	body.WriteString(styles.text.Bold(true).Render(title))
	summary := fmt.Sprintf("  %d runs · %d %s", total, activeBuckets, bucketLabel)
	if width < 70 {
		summary = fmt.Sprintf("  %d uses · %d active", total, activeBuckets)
	}
	body.WriteString(styles.muted.Render(summary))
	body.WriteString("\n\n")
	if maximum == 0 {
		body.WriteString(styles.accent.Render("No dated alias activity in this timeframe."))
		body.WriteString("\n")
		body.WriteString(styles.muted.Render("New shell history entries will appear here as they accumulate."))
		return body.String()
	}
	cellWidth := 4
	if len(buckets)*cellWidth > width-4 {
		cellWidth = 3
	}
	barCell := strings.Repeat("█", cellWidth-1) + " "
	emptyCell := strings.Repeat(" ", cellWidth)
	const chartHeight = 8
	for level := chartHeight; level > 0; level-- {
		for _, bucket := range buckets {
			filled := (bucket.count*chartHeight + maximum - 1) / maximum
			if filled >= level {
				body.WriteString(styles.accent.Render(barCell))
			} else {
				body.WriteString(emptyCell)
			}
		}
		body.WriteString("\n")
	}
	for _, bucket := range buckets {
		label := truncate(bucket.label, cellWidth-1)
		body.WriteString(styles.muted.Render(fmt.Sprintf("%-*s", cellWidth, label)))
	}
	body.WriteString("\n")
	for _, bucket := range buckets {
		count := fmt.Sprintf("%d", bucket.count)
		if len(count) > cellWidth {
			count = fmt.Sprintf("%dk", bucket.count/1000)
		}
		body.WriteString(styles.text.Render(fmt.Sprintf("%-*s", cellWidth, truncate(count, cellWidth))))
	}
	return strings.TrimRight(body.String(), "\n")
}

type activityBucket struct {
	label string
	count int
}

func activityBuckets(events []usageEvent, period string, now time.Time) (string, []activityBucket) {
	location := now.Location()
	switch period {
	case "today":
		buckets := make([]activityBucket, 8)
		for index := range buckets {
			buckets[index].label = fmt.Sprintf("%02d", index*3)
		}
		for _, event := range events {
			local := event.Time.In(location)
			if event.Time.IsZero() || local.Year() != now.Year() || local.YearDay() != now.YearDay() {
				continue
			}
			buckets[local.Hour()/3].count++
		}
		return "Today's activity", buckets
	case "year":
		buckets := make([]activityBucket, 12)
		byMonth := make(map[string]int, 12)
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location).AddDate(0, -11, 0)
		for index := range buckets {
			month := start.AddDate(0, index, 0)
			buckets[index].label = month.Format("Jan")
			byMonth[month.Format("2006-01")] = index
		}
		for _, event := range events {
			if event.Time.IsZero() {
				continue
			}
			if index, ok := byMonth[event.Time.In(location).Format("2006-01")]; ok {
				buckets[index].count++
			}
		}
		return "Twelve-month activity", buckets
	case "all":
		firstYear := now.Year()
		for _, event := range events {
			if !event.Time.IsZero() && event.Time.In(location).Year() < firstYear {
				firstYear = event.Time.In(location).Year()
			}
		}
		firstYear = max(firstYear, now.Year()-9)
		buckets := make([]activityBucket, now.Year()-firstYear+1)
		for index := range buckets {
			year := firstYear + index
			buckets[index].label = fmt.Sprintf("%02d", year%100)
		}
		for _, event := range events {
			if event.Time.IsZero() {
				continue
			}
			year := event.Time.In(location).Year()
			if year >= firstYear && year <= now.Year() {
				buckets[year-firstYear].count++
			}
		}
		return "All-time activity by year", buckets
	default:
		days := sevenDayActivity(events, now)
		buckets := make([]activityBucket, len(days))
		for index, day := range days {
			buckets[index] = activityBucket{label: day.date.Format("Mon"), count: day.count}
		}
		return "Seven-day activity", buckets
	}
}

type staleAlias struct {
	name    string
	ageDays int
	never   bool
}

func renderStaleAliases(data statsData, period string, now time.Time, width, height int, styles statsChartStyles) string {
	lastSeen := make(map[string]time.Time)
	unknownAge := make(map[string]bool)
	seen := make(map[string]bool)
	for _, event := range data.Events {
		seen[event.Name] = true
		if event.Time.IsZero() {
			unknownAge[event.Name] = true
			continue
		}
		if event.Time.Before(lastSeen[event.Name]) {
			continue
		}
		lastSeen[event.Name] = event.Time
	}
	threshold := 30
	title := "Unused for a month"
	if period == "quarter" {
		threshold, title = 90, "Unused for a quarter"
	} else if period == "year" {
		threshold, title = 365, "Unused for a year"
	} else if period == "never" {
		title = "Never-used aliases"
	}
	var aliases []staleAlias
	unknownCount := 0
	for _, alias := range data.Aliases {
		last, ok := lastSeen[alias.Name]
		if !ok {
			if unknownAge[alias.Name] {
				unknownCount++
			} else if period == "never" && !seen[alias.Name] {
				aliases = append(aliases, staleAlias{name: alias.Name, never: true})
			}
			continue
		}
		ageDays := max(0, int(now.Sub(last).Hours()/24))
		if period != "never" && ageDays >= threshold {
			aliases = append(aliases, staleAlias{name: alias.Name, ageDays: ageDays})
		}
	}
	sort.Slice(aliases, func(i, j int) bool {
		if aliases[i].ageDays == aliases[j].ageDays {
			return aliases[i].name < aliases[j].name
		}
		return aliases[i].ageDays > aliases[j].ageDays
	})
	var body strings.Builder
	aliasLabel := "aliases"
	if len(aliases) == 1 {
		aliasLabel = "alias"
	}
	body.WriteString(styles.text.Bold(true).Render(title))
	body.WriteString(styles.muted.Render(fmt.Sprintf("  %d %s", len(aliases), aliasLabel)))
	body.WriteString("\n\n")
	if len(aliases) == 0 {
		body.WriteString(styles.accent.Render("Nothing matches this cleanup window."))
		body.WriteString("\n")
		body.WriteString(styles.muted.Render("Try a shorter timeframe or keep using your current alias set."))
		return body.String()
	}
	visible := min(len(aliases), max(3, height-16))
	barWidth := max(8, min(50, width-32))
	maximum := max(1, aliases[0].ageDays)
	for _, alias := range aliases[:visible] {
		filled := barWidth
		age := "never"
		if !alias.never {
			filled = max(1, alias.ageDays*barWidth/maximum)
			age = formatAliasAge(alias.ageDays)
		}
		body.WriteString(styles.text.Render(fmt.Sprintf("%-16s", truncate(alias.name, 16))))
		body.WriteString(styles.accent.Render(strings.Repeat("━", filled)))
		body.WriteString(styles.muted.Render(strings.Repeat("─", barWidth-filled)))
		body.WriteString(styles.text.Render(fmt.Sprintf("  %s", age)))
		body.WriteString("\n")
	}
	body.WriteString("\n")
	if len(aliases) > visible {
		body.WriteString(styles.muted.Render(fmt.Sprintf("%d more aliases are hidden. ", len(aliases)-visible)))
	}
	if unknownCount > 0 {
		unknownLabel := "aliases have"
		if unknownCount == 1 {
			unknownLabel = "alias has"
		}
		body.WriteString(styles.muted.Render(fmt.Sprintf("%d %s usage with unknown dates.", unknownCount, unknownLabel)))
	}
	return strings.TrimRight(body.String(), "\n")
}

func formatAliasAge(days int) string {
	if days < 365 {
		return fmt.Sprintf("%dmo", max(1, days/30))
	}
	years := days / 365
	months := (days % 365) / 30
	if months == 0 {
		return fmt.Sprintf("%dy", years)
	}
	return fmt.Sprintf("%dy %dmo", years, months)
}

type groupUsage struct {
	name  string
	count int
}

func renderGroupShare(data statsData, period string, now time.Time, width int, styles statsChartStyles) string {
	since, _ := statsPeriodSince(period, now)
	groupsByAlias := make(map[string]string, len(data.Aliases))
	for _, alias := range data.Aliases {
		group := alias.Category
		if group == "" && len(alias.Tags) > 0 {
			group = alias.Tags[0]
		}
		if group == "" {
			group = "untagged"
		}
		groupsByAlias[alias.Name] = group
	}
	counts := make(map[string]int)
	total := 0
	for _, event := range data.Events {
		if !since.IsZero() && (event.Time.IsZero() || event.Time.Before(since)) {
			continue
		}
		if group, ok := groupsByAlias[event.Name]; ok {
			counts[group]++
			total++
		}
	}
	groups := make([]groupUsage, 0, len(counts))
	for name, count := range counts {
		groups = append(groups, groupUsage{name: name, count: count})
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].count == groups[j].count {
			return groups[i].name < groups[j].name
		}
		return groups[i].count > groups[j].count
	})
	var body strings.Builder
	body.WriteString(styles.text.Bold(true).Render("Usage by group"))
	body.WriteString(styles.muted.Render(fmt.Sprintf("  %s · %d runs", period, total)))
	body.WriteString("\n\n")
	if len(groups) == 0 {
		body.WriteString(styles.accent.Render("No grouped activity in this period."))
		body.WriteString("\n")
		body.WriteString(styles.muted.Render("Add categories or tags to see where your aliases do the most work."))
		return body.String()
	}
	groups = collapseGroupSlices(groups)
	palette := []string{styles.theme.Accent, styles.theme.Secondary, styles.theme.Git, styles.theme.Docker, styles.theme.Files}
	pie := renderGroupPie(groups, total, palette)
	var legend strings.Builder
	for index, group := range groups {
		percent := (group.count*100 + total/2) / total
		marker := lipgloss.NewStyle().Foreground(lipgloss.Color(palette[index])).Render("▪")
		legend.WriteString(marker)
		legend.WriteString(styles.text.Render(fmt.Sprintf(" %-13s %4d  %3d%%", truncate(group.name, 13), group.count, percent)))
		legend.WriteString("\n")
	}
	chart := ""
	if width >= 64 {
		chart = lipgloss.JoinHorizontal(lipgloss.Top, pie, "    ", strings.TrimRight(legend.String(), "\n"))
	} else {
		chart = pie + "\n\n" + strings.TrimRight(legend.String(), "\n")
	}
	body.WriteString(chart)
	body.WriteString("\n")
	body.WriteString("\n")
	body.WriteString(styles.muted.Render("Uses category first, then the first tag, then untagged."))
	return strings.TrimRight(body.String(), "\n")
}

func collapseGroupSlices(groups []groupUsage) []groupUsage {
	const maxSlices = 5
	if len(groups) <= maxSlices {
		return groups
	}
	collapsed := append([]groupUsage(nil), groups[:maxSlices-1]...)
	other := groupUsage{name: "other"}
	for _, group := range groups[maxSlices-1:] {
		other.count += group.count
	}
	return append(collapsed, other)
}

func renderGroupPie(groups []groupUsage, total int, palette []string) string {
	const columns, rows = 20, 10
	styles := make([]lipgloss.Style, len(groups))
	for index := range groups {
		styles[index] = lipgloss.NewStyle().Foreground(lipgloss.Color(palette[index%len(palette)]))
	}
	var pie strings.Builder
	for row := 0; row < rows; row++ {
		for column := 0; column < columns; column++ {
			x := (float64(column) + 0.5 - float64(columns)/2) / (float64(columns) / 2)
			y := (float64(row) + 0.5 - float64(rows)/2) / (float64(rows) / 2)
			if x*x+y*y > 1 {
				pie.WriteString(" ")
				continue
			}
			angle := math.Atan2(x, -y)
			if angle < 0 {
				angle += 2 * math.Pi
			}
			position := angle / (2 * math.Pi)
			cumulative := 0
			segment := len(groups) - 1
			for index, group := range groups {
				cumulative += group.count
				if position < float64(cumulative)/float64(total) {
					segment = index
					break
				}
			}
			pie.WriteString(styles[segment].Render("●"))
		}
		if row < rows-1 {
			pie.WriteString("\n")
		}
	}
	return pie.String()
}
