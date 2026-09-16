# Alias Lens compatibility policy

Alias Lens stores a schema version in `~/.config/alias-lens/config.json`. It migrates older supported configurations before use and saves the previous file as `config.json.alias-lens.bak`. Alias Lens refuses to open a configuration written by a newer unsupported version.

The following rules apply starting with Alias Lens 1.0:

- Existing commands and flags keep their meaning throughout the 1.x series. A replacement must ship before a command or flag is removed.
- Existing `# al:` metadata fields remain readable throughout the 1.x series.
- JSON output keeps existing field names and meanings. Minor releases may add fields.
- Configuration migrations preserve repository, shell, provider, automatic sync, and tracked-file settings.
- `al setup --repair` may replace generated shell integration. It does not change user aliases.
- `al setup --remove` removes generated integration only. It keeps aliases and Alias Lens data.

The optional browser interface is not part of the 1.0 compatibility promise. Its HTML, CSS, JavaScript, and local API may change between minor releases.
