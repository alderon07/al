package app

import (
	neutralcatalog "github.com/alderon07/al/internal/catalog"
	"github.com/alderon07/al/internal/presentation"
)

func terminalSafeText(value string) string     { return presentation.TerminalSafeText(value) }
func escapePlainText(value string) string      { return presentation.EscapePlainText(value) }
func shortFingerprint(value string) string     { return presentation.ShortFingerprint(value) }
func friendlyCatalogField(value string) string { return presentation.FriendlyCatalogField(value) }
func quotedCatalogValue(value any) string      { return presentation.QuotedCatalogValue(value) }
func (svc *Services) catalogFieldValue(value neutralcatalog.Entry, path string) any {
	return presentation.CatalogFieldValue(value, path)
}
func friendlyShellName(value string) string       { return presentation.FriendlyShellName(value) }
func defaultString(value, fallback string) string { return presentation.DefaultString(value, fallback) }
func findingCount(count int, noun string) string  { return presentation.FindingCount(count, noun) }
