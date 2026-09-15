# Alias workflow review

This review records the decisions behind the current roadmap. It compares Alias Lens with Atuin where the products solve similar shell problems.

## Editable text in the alias card

The badge in the screenshot says `CUSTOM`. Alias Lens infers that category from the first command word. Known commands map to categories such as `git`, `docker`, `files`, and `dev`. Everything else starts in `custom`.

The sentence under the alias name is the description. A nearby shell comment supplies custom text. If no comment exists, Alias Lens generates a description.

The TUI edit form now exposes both values and the alias tags. A custom category is stored with the other structured metadata:

```bash
# Clear this terminal
# al: tags=daily,terminal category=utility
alias cl='clear'
```

`al meta` remains useful for scripts:

```bash
al meta cl tags=daily,terminal category=utility
```

## Launch shortcut

After `al setup`, press `Ctrl+G` on an empty Bash or Zsh prompt to open Alias Lens. Selecting an alias runs it. Typing `al` remains a fallback and a way to run subcommands.

`Ctrl+G` normally cancels the current Readline or ZLE operation. Alias Lens preserves that behavior when the prompt contains text. On an empty prompt, the binding opens the alias picker. This prompt-aware rule avoids taking a useful editing shortcut away from the user.

No terminal shortcut is collision-free across every shell, terminal, multiplexer, and user configuration. Set `ALIAS_LENS_NOBIND=1` before the Alias Lens integration to install no binding. The integration still defines `_alias_lens_launch`, which users can bind with their shell's normal binding command.

Atuin takes the same configurable approach. Its setup provides default bindings, flags to disable individual bindings, and shell widgets for custom keys. See the [Atuin key-binding guide](https://docs.atuin.sh/main/configuration/key-binding/).

## Search and stats

`al search [QUERY]` prints matching aliases without opening the TUI. It searches names, commands, descriptions, categories, tags, platforms, and entry types. `--json` provides stable structured output for scripts.

`al stats` ranks launches made through the Alias Lens picker. The command accepts these periods:

- `all`
- `today`, starting at local midnight
- `week`, meaning the previous 7 days
- `year`, meaning the previous 12 months

The usage log stores only a Unix timestamp and the alias name. It has mode `0600`. Existing shell history still influences TUI suggestions, but it does not provide reliable dated alias launches on every supported shell.

Atuin records richer execution context, including the directory, duration, exit code, host, and session. Its filter modes use that context to narrow search to a directory, workspace, host, or session. See [Atuin basic usage](https://docs.atuin.sh/main/guide/basic-usage/) and [advanced usage](https://docs.atuin.sh/main/guide/advanced-usage/).

## Encryption decision

Do not encrypt the live alias file or the local usage log inside Alias Lens.

The shell must source the alias file during startup. Application-level encryption would require Alias Lens to keep a decrypted copy or insert a decryption step into shell startup. Both choices add key-management and recovery problems while providing little protection after login. Full-disk encryption and file permissions are the right controls for local data. The usage log contains names and timestamps, not command text, and uses mode `0600`.

The current Git sync stores the alias file in its usable text form. Users must treat that repository as private. The secret scanner lowers the chance of a credential push, but it does not make a public repository safe.

End-to-end encryption becomes necessary if Alias Lens adds a hosted sync service whose operator should not read user aliases. Atuin encrypts synchronized history before it reaches the server and requires users to retain the encryption key. Its design is a useful reference for that future feature. See the [Atuin sync documentation](https://docs.atuin.sh/18.17/reference/sync/) and [encrypted record store](https://docs.atuin.sh/main/reference/store/).

## Go or Rust

Keep the product in Go for now.

Rust could offer lower memory use, predictable latency without garbage collection, and strong compile-time ownership checks. Its CLI ecosystem is good, and Atuin proves that Rust can support a polished cross-shell application.

A port would also replace the tested parsers, atomic writer, shell setup, Git synchronization, provider clients, and Bubble Tea interface at once. That creates a long regression window in the parts of Alias Lens that protect user files. Search over a normal alias file is too small to justify that cost. The slow parts are more likely process startup, shell history reads, Git, and network calls than Go itself.

Measure before reconsidering. Add benchmarks for cold startup, 10,000 aliases, a large history file, and TUI query latency. Consider Rust only if measured performance misses a product target and profiling shows that Go or its runtime is the cause.

## Useful Atuin ideas to adapt

The highest-value additions are about context and safe control, not copying Atuin's history database.

1. Add an insert-for-edit action. Atuin uses `Enter` to execute and `Tab` to return a command to the prompt. Alias Lens needs the same escape hatch for aliases that require one-off arguments.
2. Rank by directory and Git workspace. Project-specific aliases should rise when the user is inside that project, without hiding global aliases.
3. Record outcomes only after an explicit privacy design. Exit status and duration can identify broken or slow aliases, but collection needs exclusions and a documented retention policy.
4. Preview imports and conflicts. Show duplicate names, duplicate commands, shell incompatibilities, and proposed metadata before writing.
5. Keep bindings optional. Atuin lets users disable defaults and bind its shell widgets themselves. Alias Lens now follows this model.
6. Keep diagnostics actionable. Both products provide a doctor command. Alias Lens should extend `al doctor` as new hooks and context storage arrive.

Alias Lens now runs `al check` from `al doctor`. The standalone command validates Alias Lens metadata and definitions, then uses the configured shell's parse-only mode for native syntax. It never sources or executes the alias file.

Lower-priority ideas include an inline-height mode, per-host filters, and a local search daemon. None is worth adding until users have enough aliases or usage records for the current in-process search to feel slow.
