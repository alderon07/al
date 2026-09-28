package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"github.com/pterm/pterm"
)

// cliStyled reports whether human-readable command output can use terminal styling.
// Machine-readable and redirected output never call the styling helpers.
func cliStyled() bool {
	_, noColor := os.LookupEnv("NO_COLOR")
	return !noColor && os.Getenv("TERM") != "dumb" && term.IsTerminal(os.Stdout.Fd())
}

func cliAccent(value string) string {
	if !cliStyled() {
		return value
	}
	return pterm.NewRGBStyle(pterm.NewRGB(0x72, 0xdd, 0xf7)).AddOptions(pterm.Bold).Sprint(value)
}

func cliPositive(value string) string {
	if !cliStyled() {
		return value
	}
	return pterm.NewRGBStyle(pterm.NewRGB(0xb8, 0xff, 0x6a)).AddOptions(pterm.Bold).Sprint(value)
}

func cliAttention(value string) string {
	if !cliStyled() {
		return value
	}
	return pterm.NewRGBStyle(pterm.NewRGB(0xff, 0xd1, 0x66)).AddOptions(pterm.Bold).Sprint(value)
}

func cliMuted(value string) string {
	if !cliStyled() {
		return value
	}
	return pterm.NewRGBStyle(pterm.NewRGB(0x8e, 0xa6, 0xa2)).Sprint(value)
}

func cliColumns() int {
	width, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil || width < 40 {
		return 80
	}
	return min(width, 100)
}

func cliReportHeading(section string) string {
	return cliPositive("Alias Lens") + "  " + cliAccent(section) + "\n" + cliMuted(strings.Repeat("─", min(cliColumns(), 36))) + "\n"
}

func cliHeading(value string) {
	fmt.Println(cliAccent(value))
}

func cliResult(value string) {
	if cliStyled() {
		fmt.Println(cliPositive("OK") + "  " + value)
		return
	}
	fmt.Println(value)
}

func cliKeyValue(label, value string) {
	fmt.Printf("%s: %s\n", cliAccent(label), value)
}

func cliError(err error) {
	if cliStyled() && term.IsTerminal(os.Stderr.Fd()) {
		fmt.Fprintln(os.Stderr, cliAttention("Error")+"  "+err.Error())
		return
	}
	fmt.Fprintln(os.Stderr, "Alias Lens:", err)
}

func cliUsageText(output string) string {
	if !cliStyled() {
		return output
	}
	lines := strings.SplitAfter(output, "\n")
	for index, line := range lines {
		content := strings.TrimSuffix(line, "\n")
		if index == 0 || strings.HasSuffix(content, ":") {
			lines[index] = cliAccent(content) + "\n"
		}
	}
	return strings.Join(lines, "")
}

func cliStatusText(output string) string {
	if !cliStyled() {
		return output
	}
	var result strings.Builder
	result.WriteString(cliReportHeading("status"))
	result.WriteByte('\n')
	for _, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n")[1:] {
		label, value, found := strings.Cut(line, ": ")
		if !found {
			result.WriteString(line + "\n")
			continue
		}
		prefix := "  " + cliMuted(fmt.Sprintf("%-16s", label)) + " "
		wrapped := strings.Split(ansi.Wrap(value, cliColumns()-19, ""), "\n")
		for index, part := range wrapped {
			if index == 0 {
				result.WriteString(prefix)
			} else {
				result.WriteString(strings.Repeat(" ", 19))
			}
			result.WriteString(part + "\n")
		}
	}
	return result.String()
}

func cliDoctorRow(ok bool, name, message string) {
	marker := "OK "
	if !ok {
		marker = "FIX"
	}
	if !cliStyled() {
		fmt.Printf("%s  %-18s %s\n", marker, name, message)
		return
	}
	if ok {
		marker = cliPositive(marker)
	} else {
		marker = cliAttention(marker)
	}
	prefix := "  " + marker + "  " + cliMuted(fmt.Sprintf("%-18s", name)) + " "
	for index, part := range strings.Split(ansi.Wrap(message, cliColumns()-26, ""), "\n") {
		if index == 0 {
			fmt.Println(prefix + part)
		} else {
			fmt.Println(strings.Repeat(" ", 26) + part)
		}
	}
}

func cliPlanText(output string) string {
	if !cliStyled() {
		return output
	}
	lines := strings.SplitAfter(output, "\n")
	for index, line := range lines {
		content := strings.TrimSuffix(line, "\n")
		switch {
		case content == "Planned changes":
			lines[index] = cliAccent(content) + "\n"
		case strings.HasPrefix(content, "Review needed:") || strings.HasPrefix(content, "Alias Lens cannot"):
			lines[index] = cliAttention(content) + "\n"
		case content == "No changes are needed." || content == "Nothing has been changed.":
			lines[index] = cliPositive(content) + "\n"
		}
	}
	return strings.Join(lines, "")
}

func withCLIProgress(label string, run func() (string, error)) (string, error) {
	if !cliStyled() || !term.IsTerminal(os.Stderr.Fd()) {
		return run()
	}
	spinner, err := pterm.DefaultSpinner.WithText(label).WithShowTimer(false).WithRemoveWhenDone(true).Start()
	if err != nil {
		return run()
	}
	message, runErr := run()
	_ = spinner.Stop()
	return message, runErr
}
