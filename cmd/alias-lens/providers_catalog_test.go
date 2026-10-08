package main

import (
	"github.com/alderon07/al/internal/catalog"

	"net/http"
	"testing"
)

type catalogProviderRoundTrip func(*http.Request) (*http.Response, error)

func (f catalogProviderRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func syntheticCatalogProviderBytes(t *testing.T) []byte {
	t.Helper()
	data, d := catalog.Encode(catalog.Catalog{SchemaVersion: 2, Entries: []catalog.Entry{{ID: "11111111111111111111111111111111", Name: "demo", Kind: "command", Portable: &catalog.Portable{Program: "printf", Args: []string{"synthetic"}}}}})
	if len(d) > 0 {
		t.Fatal(d)
	}
	return data
}
