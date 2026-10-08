package app

import (
	"time"

	"alias-lens/internal/exportfile"
)

type aliasExport struct {
	Kind       string    `json:"kind"`
	ExportedAt time.Time `json:"exported_at"`
	Aliases    []Alias   `json:"aliases"`
}

type statsExportRow struct {
	Name        string   `json:"name"`
	Count       int      `json:"count"`
	Command     string   `json:"command"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type statsExport struct {
	Kind       string           `json:"kind"`
	Period     string           `json:"period"`
	ExportedAt time.Time        `json:"exported_at"`
	Aliases    []statsExportRow `json:"aliases"`
}

func (svc *Services) buildExport(kind, format, period string, now time.Time) ([]byte, error) {
	if kind == "aliases" {
		aliases, err := svc.loadAliases()
		if err != nil {
			return nil, err
		}
		return svc.encodeAliasExport(aliasExport{Kind: "aliases", ExportedAt: now, Aliases: aliases}, format)
	}
	data, err := svc.loadStatsData()
	if err != nil {
		return nil, err
	}
	rows, err := rankedStatsRows(data, period, now)
	if err != nil {
		return nil, err
	}
	exported := statsExport{Kind: "stats", Period: period, ExportedAt: now, Aliases: make([]statsExportRow, 0, len(rows))}
	for _, row := range rows {
		exported.Aliases = append(exported.Aliases, statsExportRow{Name: row.Alias.Name, Count: row.Count, Command: row.Alias.Command, Description: row.Alias.Description, Tags: row.Alias.Tags})
	}
	return encodeStatsExport(exported, format)
}

func (svc *Services) encodeAliasExport(export aliasExport, format string) ([]byte, error) {
	aliases := make([]exportfile.Alias, 0, len(export.Aliases))
	for _, alias := range export.Aliases {
		aliases = append(aliases, svc.exportAlias(alias))
	}
	return exportfile.EncodeAliases(aliases, export.ExportedAt, format)
}

func encodeStatsExport(export statsExport, format string) ([]byte, error) {
	rows := make([]exportfile.StatsRow, 0, len(export.Aliases))
	for _, row := range export.Aliases {
		rows = append(rows, exportfile.StatsRow{Alias: exportfile.Alias{Name: row.Name, Command: row.Command, Description: row.Description, Tags: row.Tags}, Count: row.Count})
	}
	return exportfile.EncodeStats(rows, export.Period, export.ExportedAt, format)
}

func (svc *Services) exportAlias(alias Alias) exportfile.Alias {
	return exportfile.Alias{Name: alias.Name, Command: alias.Command, Description: alias.Description, Category: alias.Category, Type: alias.Type, Tags: alias.Tags, Platforms: alias.Platforms, Favorite: alias.Favorite}
}

func writePrivateExport(path string, contents []byte) error {
	return exportfile.WritePrivate(path, contents)
}
