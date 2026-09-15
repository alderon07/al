package exportfile

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Alias struct {
	Name        string   `json:"name"`
	Command     string   `json:"command"`
	Description string   `json:"description,omitempty"`
	Category    string   `json:"category,omitempty"`
	Type        string   `json:"type,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Platforms   []string `json:"platforms,omitempty"`
	Favorite    bool     `json:"favorite,omitempty"`
}

type StatsRow struct {
	Alias
	Count int `json:"count"`
}

func EncodeAliases(aliases []Alias, exportedAt time.Time, format string) ([]byte, error) {
	switch format {
	case "json":
		return indentedJSON(struct {
			Kind       string    `json:"kind"`
			ExportedAt time.Time `json:"exported_at"`
			Aliases    []Alias   `json:"aliases"`
		}{Kind: "aliases", ExportedAt: exportedAt, Aliases: aliases})
	case "yaml":
		return encodeAliasesYAML(aliases, exportedAt), nil
	case "csv":
		return encodeAliasesCSV(aliases)
	default:
		return nil, fmt.Errorf("unsupported format %q", format)
	}
}

func EncodeStats(rows []StatsRow, period string, exportedAt time.Time, format string) ([]byte, error) {
	switch format {
	case "json":
		return indentedJSON(struct {
			Kind       string     `json:"kind"`
			Period     string     `json:"period"`
			ExportedAt time.Time  `json:"exported_at"`
			Aliases    []StatsRow `json:"aliases"`
		}{Kind: "stats", Period: period, ExportedAt: exportedAt, Aliases: rows})
	case "yaml":
		return encodeStatsYAML(rows, period, exportedAt), nil
	case "csv":
		return encodeStatsCSV(rows, period)
	default:
		return nil, fmt.Errorf("unsupported format %q", format)
	}
}

func encodeAliasesYAML(aliases []Alias, exportedAt time.Time) []byte {
	var output strings.Builder
	fmt.Fprintf(&output, "kind: aliases\nexported_at: %s\n", yamlString(exportedAt.Format(time.RFC3339)))
	if len(aliases) == 0 {
		output.WriteString("aliases: []\n")
		return []byte(output.String())
	}
	output.WriteString("aliases:\n")
	for _, alias := range aliases {
		output.WriteString(yamlAlias(alias, false, 0))
	}
	return []byte(output.String())
}

func encodeStatsYAML(rows []StatsRow, period string, exportedAt time.Time) []byte {
	var output strings.Builder
	fmt.Fprintf(&output, "kind: stats\nperiod: %s\nexported_at: %s\n", yamlString(period), yamlString(exportedAt.Format(time.RFC3339)))
	if len(rows) == 0 {
		output.WriteString("aliases: []\n")
		return []byte(output.String())
	}
	output.WriteString("aliases:\n")
	for _, row := range rows {
		output.WriteString(yamlAlias(row.Alias, true, row.Count))
	}
	return []byte(output.String())
}

func yamlAlias(alias Alias, includeCount bool, count int) string {
	var output strings.Builder
	fmt.Fprintf(&output, "  - name: %s\n", yamlString(alias.Name))
	if includeCount {
		fmt.Fprintf(&output, "    count: %d\n", count)
	}
	fmt.Fprintf(&output, "    command: %s\n", yamlString(alias.Command))
	fmt.Fprintf(&output, "    description: %s\n", yamlString(alias.Description))
	if alias.Category != "" {
		fmt.Fprintf(&output, "    category: %s\n", yamlString(alias.Category))
	}
	if alias.Type != "" {
		fmt.Fprintf(&output, "    type: %s\n", yamlString(alias.Type))
	}
	if len(alias.Tags) > 0 {
		tags, _ := json.Marshal(alias.Tags)
		fmt.Fprintf(&output, "    tags: %s\n", tags)
	}
	if len(alias.Platforms) > 0 {
		platforms, _ := json.Marshal(alias.Platforms)
		fmt.Fprintf(&output, "    platforms: %s\n", platforms)
	}
	if alias.Favorite {
		output.WriteString("    favorite: true\n")
	}
	return output.String()
}

func encodeAliasesCSV(aliases []Alias) ([]byte, error) {
	var output bytes.Buffer
	writer := csv.NewWriter(&output)
	_ = writer.Write([]string{"name", "command", "description", "category", "type", "tags", "platforms", "favorite"})
	for _, alias := range aliases {
		_ = writer.Write(safeCSVFields([]string{alias.Name, alias.Command, alias.Description, alias.Category, alias.Type, strings.Join(alias.Tags, ","), strings.Join(alias.Platforms, ","), fmt.Sprint(alias.Favorite)}))
	}
	writer.Flush()
	return output.Bytes(), writer.Error()
}

func encodeStatsCSV(rows []StatsRow, period string) ([]byte, error) {
	var output bytes.Buffer
	writer := csv.NewWriter(&output)
	_ = writer.Write([]string{"period", "name", "count", "command", "description", "tags"})
	for _, row := range rows {
		_ = writer.Write(safeCSVFields([]string{period, row.Name, fmt.Sprint(row.Count), row.Command, row.Description, strings.Join(row.Tags, ",")}))
	}
	writer.Flush()
	return output.Bytes(), writer.Error()
}

func safeCSVFields(fields []string) []string {
	for index, field := range fields {
		trimmed := strings.TrimLeft(field, " \t\r\n")
		if trimmed != "" && strings.ContainsRune("=+-@", []rune(trimmed)[0]) {
			fields[index] = "'" + field
		}
	}
	return fields
}

func indentedJSON(value any) ([]byte, error) {
	contents, err := json.MarshalIndent(value, "", "  ")
	return append(contents, '\n'), err
}

func yamlString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func WritePrivate(path string, contents []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".alias-lens-export-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}
