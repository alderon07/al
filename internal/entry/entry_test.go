package entry

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestAliasJSONContractAndOptionalFields(t *testing.T) {
	empty, err := json.Marshal(Alias{})
	if err != nil {
		t.Fatal(err)
	}
	if string(empty) != `{"name":"","command":"","description":"","category":""}` {
		t.Fatal("entry JSON omission contract changed")
	}
	value := Alias{CatalogID: "synthetic-id", CatalogState: "installed", Name: "demo", Command: "git status", Description: "synthetic", Category: "git", Issues: []string{"synthetic issue"}, Usage: 2, Type: "alias", Tags: []string{"work"}, Platforms: []string{"linux"}, Favorite: true}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"catalog_id":"synthetic-id","catalog_state":"installed","name":"demo","command":"git status","description":"synthetic","category":"git","issues":["synthetic issue"],"usage":2,"type":"alias","tags":["work"],"platforms":["linux"],"favorite":true}`
	if string(data) != want {
		t.Fatal("entry JSON field contract changed")
	}
	var decoded Alias
	if err := json.Unmarshal(data, &decoded); err != nil || !reflect.DeepEqual(decoded, value) {
		t.Fatal("entry JSON did not round trip", err)
	}
}

func TestMetadataNormalizationAndOwnership(t *testing.T) {
	metadata, ok := ParseMetadataComment("# AL: collections=Work,work,Personal platforms=WSL,linux favorite=yes category=Tools")
	if !ok || !reflect.DeepEqual(metadata, EntryMetadata{Tags: []string{"personal", "work"}, Platforms: []string{"linux", "wsl"}, Favorite: true, Category: "tools"}) {
		t.Fatal("metadata normalization changed")
	}
	if MetadataLine(metadata) != "# al: tags=personal,work platforms=linux,wsl favorite=true category=tools" {
		t.Fatal("metadata order changed")
	}
	alias := Alias{Command: "git status", Category: Category("git status")}
	ApplyMetadata(&alias, metadata)
	metadata.Tags[0] = "changed"
	metadata.Platforms[0] = "changed"
	if alias.Tags[0] != "personal" || alias.Platforms[0] != "linux" || alias.Category != "tools" {
		t.Fatal("metadata application lost copy ownership")
	}
	if MetadataForAlias(Alias{Command: "git status", Category: "git"}).Category != "" || Describe("demo", "git status") != "Show changed files and the current branch" {
		t.Fatal("entry default metadata changed")
	}
}

func TestPlatformEligibilityUsesExplicitPlatform(t *testing.T) {
	getenv := func(key string) string {
		if key == "WSL_INTEROP" {
			return "synthetic"
		}
		return ""
	}
	if PlatformName("linux", getenv) != "wsl" || PlatformName("darwin", getenv) != "macos" {
		t.Fatal("catalog platform names changed")
	}
	if !PlatformSupported([]string{"linux"}, "wsl") || !PlatformSupported(nil, "macos") || PlatformSupported([]string{"linux"}, "macos") {
		t.Fatal("platform eligibility changed")
	}
}
