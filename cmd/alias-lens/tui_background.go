package main

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func terminalBackgroundColor(value string) lipgloss.Color {
	if lipgloss.ColorProfile() != termenv.ANSI256 || len(value) != 7 || value[0] != '#' {
		return lipgloss.Color(value)
	}
	channels := [3]int{}
	for index := range channels {
		channel, err := strconv.ParseUint(value[1+index*2:3+index*2], 16, 8)
		if err != nil {
			return lipgloss.Color(value)
		}
		channels[index] = int(channel)
	}
	bestIndex := 16
	bestDistance := int(^uint(0) >> 1)
	levels := [6]int{0, 95, 135, 175, 215, 255}
	for red := range levels {
		for green := range levels {
			for blue := range levels {
				index := 16 + 36*red + 6*green + blue
				distance := colorDistance(channels, [3]int{levels[red], levels[green], levels[blue]})
				if distance < bestDistance {
					bestIndex, bestDistance = index, distance
				}
			}
		}
	}
	for index := 232; index <= 255; index++ {
		gray := 8 + 10*(index-232)
		distance := colorDistance(channels, [3]int{gray, gray, gray})
		if distance < bestDistance {
			bestIndex, bestDistance = index, distance
		}
	}
	return lipgloss.Color(strconv.Itoa(bestIndex))
}

func colorDistance(left, right [3]int) int {
	distance := 0
	for index := range left {
		delta := left[index] - right[index]
		distance += delta * delta
	}
	return distance
}

func fillUnstyledBackground(view string, color lipgloss.Color) string {
	if !themeCanvasAvailable() {
		return view
	}
	marker := lipgloss.NewStyle().Background(color).Render("x")
	background := marker[:strings.IndexByte(marker, 'x')]
	if background == "" {
		return view
	}
	var output strings.Builder
	output.Grow(len(view) + len(background)*strings.Count(view, "\x1b[0m"))
	hasBackground := false
	for index := 0; index < len(view); {
		if view[index] == '\x1b' && index+1 < len(view) && view[index+1] == '[' {
			end := index + 2
			for end < len(view) && (view[end] < 0x40 || view[end] > 0x7e) {
				end++
			}
			if end < len(view) {
				if view[end] == 'm' {
					hasBackground = sgrBackgroundState(hasBackground, view[index+2:end])
				}
				output.WriteString(view[index : end+1])
				index = end + 1
				continue
			}
		}
		if view[index] == '\n' || view[index] == '\r' {
			output.WriteByte(view[index])
			index++
			continue
		}
		if !hasBackground {
			output.WriteString(background)
			hasBackground = true
		}
		output.WriteByte(view[index])
		index++
	}
	return output.String()
}

func sgrBackgroundState(current bool, parameters string) bool {
	if parameters == "" {
		return false
	}
	parts := strings.Split(parameters, ";")
	for index := 0; index < len(parts); index++ {
		parameter, err := strconv.Atoi(parts[index])
		if err != nil {
			continue
		}
		switch {
		case parameter == 0 || parameter == 49:
			current = false
		case parameter >= 40 && parameter <= 47 || parameter >= 100 && parameter <= 107:
			current = true
		case parameter == 38 || parameter == 48 || parameter == 58:
			if parameter == 48 {
				current = true
			}
			if index+1 < len(parts) {
				switch parts[index+1] {
				case "2":
					index += min(4, len(parts)-index-1)
				case "5":
					index += min(2, len(parts)-index-1)
				}
			}
		}
	}
	return current
}
