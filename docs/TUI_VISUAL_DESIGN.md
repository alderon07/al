# Alias Lens TUI visual design

Status: Core design specification

This document defines the visual identity of the Alias Lens terminal interface. New screens and visual changes must follow it. The interface can improve, but it must remain recognizably Alias Lens.

`Must` marks a requirement. `Should` marks the expected choice unless a documented constraint prevents it. `May` marks an allowed choice. Each requirement has a stable ID so acceptance criteria and tests can cite it.

## Acceptance criteria

This specification is complete when it does all of the following:

- Names the visual traits that make Alias Lens recognizable.
- Separates fixed identity rules from parts that can change.
- Defines the default palette by semantic role.
- Defines page anatomy, spacing, cards, controls, icons, and interaction states.
- Covers narrow terminals, reduced color, Unicode cell width, and keyboard-only use.
- Gives reviewers an objective way to accept or reject a visual change.

## The identity in one sentence

Alias Lens is a compact command instrument: dark terminal chrome, a bright phosphor signal, focused terminal art, readable symbols, and plain language that keeps the user's aliases in focus.

The interface should feel native to a terminal, not like a web dashboard drawn with box characters. It uses color and character, but it stays quiet enough for commands and alias names to dominate.

## The core must survive every change

These traits are the product's visual signature. A redesign that removes most of them is a new identity, even if every feature still works.

| Core trait | Required expression | Reason |
| --- | --- | --- |
| Dark working field | The default theme uses a near-black green background with low-contrast panels. | Bright content reads as a terminal signal instead of app chrome. |
| Phosphor accent | The default accent is `#B8FF6A`. It marks the brand, focus, primary names, and positive status. | This is the strongest recognition cue in the TUI. |
| Cyan action channel | Cyan marks commands, paths, and the actionable words in control hints. | It separates "what this is" from "what you can do." |
| Focused terminal art | A three-row mark appears only in spacious welcome and empty states. Headings use configurable one-cell symbols or ASCII. | The art keeps a handmade signature without turning dense screens into block fragments. |
| Left-edge selection | Cards and command blocks use a thick left border. The active border changes to the accent color. | Focus remains visible without filling a whole card with color. |
| Command-first hierarchy | Alias name, description, then command form the scan order. | The user's content remains more important than the application frame. |
| Keyboard footer | The current controls remain visible near the bottom. | The interface teaches itself while preserving fast keyboard use. |
| Human footer | Interactive pages end with the configured maker credit. | The small personal note is part of the product's voice. |
| Text with every symbol | Icons, colors, and badges always have a nearby text equivalent. | Meaning remains available with limited color or glyph support. |

The terminal-prompt lens logo remains the product mark. The full SVG can use its blue-to-teal gradient outside the TUI. Inside the TUI, the full terminal mark gets several rows and appears only when there is room. Standard headers use the fixed `ALIAS LENS` label or compact mark. Do not place a multicolor raster or sixel logo in the standard TUI header.

## What can change

Evolution is expected. Keep the core traits above and use this change budget.

| Safe to change | Change with a design review | Do not change as part of routine feature work |
| --- | --- | --- |
| Copy length and wording | Card density or vertical rhythm | The Phosphor default as the starting identity |
| Additional semantic markers | Search-field geometry | The full terminal-mark silhouette |
| Theme presets | Header information order | The left-edge active-selection pattern |
| Responsive omissions | Global page anatomy | Keyboard-first operation |
| Chart forms and empty-state content | Semantic meaning of a color role | The visible brand name |
| Secondary screen layouts | Minimum terminal dimensions | Text labels beside icons and status colors |

A reviewed change can alter one major identity trait when the change solves a measured usability problem. It must preserve the other traits and include before-and-after terminal evidence. Changing two or more core traits in one feature requires an explicit revision to this document.

## Design foundations

### Page-frame specification

| ID | Requirement | Exact rule | Verification |
| --- | --- | --- | --- |
| `FRAME-01` | Minimum viewport | The interactive TUI must require at least 48 columns and 18 rows. | Render at 47 by 18, 48 by 17, and 48 by 18. Only the last case opens the page. |
| `FRAME-02` | Size error | The fallback must state both the 48 by 18 requirement and the detected size. | Assert both values in the plain rendered string. |
| `FRAME-03` | Outer inset | Standard pages must use one row above and below, plus three cells at the left and right. Stats may use two horizontal cells to protect chart width. | Measure the first visible content cell at 80 by 24. |
| `FRAME-04` | Content bounds | Standard-page content must be at least 40 and at most 108 cells wide. | Render at 48, 80, 116, and 160 columns. Content stops growing after 108 cells. |
| `FRAME-05` | Region order | Header, title, primary control, content, local controls, global controls, and maker credit must appear in that order when present. | Golden-test one primary and one secondary page. |
| `FRAME-06` | Vertical gaps | Major regions should have one empty row between them. Components must not add multiple decorative empty rows. | Inspect at 80 by 24 and 48 by 18. |
| `FRAME-07` | Height pressure | The page must reduce the visible list window before removing the focused item or local controls. | Shrink a populated page and assert that selection and controls remain visible. |
| `FRAME-08` | Resize stability | A resize must preserve the current object, query, entered form text, and modal state. | Update the model with smaller and larger window messages. |

### Work in terminal cells

Measure width and height in terminal cells, not bytes or runes. Use grapheme-aware truncation and wrapping for wide characters, emoji, and combining marks. Never align a view with raw byte counts.

The supported interactive viewport starts at 48 columns by 18 rows. Below that size, show the size requirement and the current dimensions. Do not render a clipped version of the application.

The standard page has these bounds:

- One row and three columns of outer padding in the alias browser and its secondary pages.
- A content width no smaller than 40 cells.
- A content width capped at 108 cells on wide terminals.
- One blank row between major regions when height permits.
- One blank row between alias cards.

Do not stretch text and cards across an ultrawide terminal. The 108-cell cap keeps scanning distances short.

### Use a stable page frame

Most screens use the same vertical order:

```text
▟█  ALIAS LENS  ~/.zsh_aliases  •  24 aliases

▟▄  Screen title
    One sentence that explains the screen.

╭──────────────────────────────────────────╮
│ ▟▄ Search or primary control             │
╰──────────────────────────────────────────╯

▌ Content, list, form, chart, or decision

enter run  ·  tab edit  ·  ? help  ·  esc quit
             Made with ♥ by Naqi
```

The title and control can be absent when they do not fit the task, but the header, content, contextual controls, and maker credit remain in that order. Confirmations replace the normal page content. They do not appear as floating modal boxes because terminal size and background rendering vary.

### Keep the hierarchy shallow

Use no more than four visual levels on one screen:

1. Brand and screen title.
2. Selected item or primary result.
3. Supporting description and command text.
4. Metadata, counts, help, and the maker credit.

Bold text marks brands, titles, primary names, selected tabs, and compact badges. Do not bold paragraphs or every control hint. Uppercase is reserved for the brand, short section labels, form labels, and badges. Sentence case is the default for titles and messages.

## Color system

Colors have jobs. A theme may replace the values, but it must preserve these roles.

| Token | Phosphor value | Use |
| --- | --- | --- |
| `Background` | `#0D1211` | Full-page background and dark text on bright badges |
| `Panel` | `#151C1A` | Command review blocks and contained regions |
| `Selected` | `#21302A` | Selected rows where a filled state is clearer than a border |
| `Border` | `#33443F` | Inactive structure and card edges, never normal text |
| `Text` | `#F3F6EE` | Titles, descriptions, and primary prose |
| `Muted` | `#8EA6A2` | Hints, counts, metadata, and the maker credit |
| `Accent` | `#B8FF6A` | Brand, focus, alias names, primary status, and selected borders |
| `Secondary` | `#72DDF7` | Commands, paths, links, and action words |
| `Git` | `#FF8A65` | Risk, errors, health issues, and Git category marks |
| `Docker` | `#72DDF7` | Docker category marks |
| `Files` | `#C6A0F6` | File category marks and function labels |
| `Dev` | `#FFD166` | Warnings, favorites, suggestions, and development category marks |

The Phosphor palette has these contrast ratios against `Background`: `Text` 17.30:1, `Accent` 15.79:1, `Secondary` 12.05:1, `Dev` 13.10:1, `Files` 8.78:1, `Git` 8.16:1, and `Muted` 7.31:1. These ratios are calculated from the sRGB values with the WCAG relative-luminance formula. Actual terminal output varies with color-profile downsampling and user settings.

Apply these rules to every theme:

- Keep normal text at 4.5:1 or better against its background.
- Keep active controls, large marks, and borders at 3:1 or better where practical.
- Never use hue as the only status or selection cue. Pair color with text, an icon, a border, a pointer, or a position change.
- Reserve coral or the theme's `Git` role for risk, failure, conflict, and destructive confirmation. A Git category badge may also use it because the badge includes `GIT` text.
- Reserve the accent for focus and identity. If most of the screen is accented, nothing appears focused.
- Render bright badges with `Background` text instead of white text.
- Keep `Border` structural. Its low contrast is intentional and is not sufficient for content.

Honor `NO_COLOR` without removing content or keyboard behavior. When `TERM=dumb`, skip the interactive interface and print plain output. Let Lip Gloss downsample colors for the detected terminal profile.

### Theme specification

Every theme must supply the same 12 roles used by `Theme`. A missing optional surface value may derive from the background, but a theme must not invent a new role for one page.

| ID | Requirement | Exact rule | Failure behavior |
| --- | --- | --- | --- |
| `COLOR-01` | Main text contrast | `Text` against `Background` must be at least 4.5:1. | `al theme --check` reports the theme and measured ratio. |
| `COLOR-02` | Accent contrast | `Accent` against `Background` must be at least 3:1. | Theme validation fails. |
| `COLOR-03` | Badge text | Badge text must use `Background` over the badge color. | Do not choose white or black independently per badge. |
| `COLOR-04` | Focus redundancy | Accent focus must also use a pointer, border change, brackets, or filled state. | Reject a color-only focus state in review. |
| `COLOR-05` | Status redundancy | Error, warning, success, and conflict states must include literal status text. | Do not render an unlabeled colored dot. |
| `COLOR-06` | No-color behavior | `NO_COLOR` must remove ANSI styling without removing words, icons made from standard block characters, selection pointers, or controls. | Compare ANSI-stripped color and no-color views for the same labels. |
| `COLOR-07` | Limited profiles | ANSI and ANSI-256 downsampling must leave foreground and background distinguishable. | Inspect one 16-color and one 256-color profile. |
| `COLOR-08` | Custom colors | New component code must use a `Theme` role. | A hard-coded hex value outside the built-in theme definitions fails review. |

Theme previews apply on cursor movement. `Enter` saves the preview. `Esc`, a global page change, or a canceled picker restores the theme that was active before the picker opened. The picker must show five swatches in this order: `Accent`, `Secondary`, `Git`, `Files`, and `Dev`.

## Type and copy

Alias Lens does not choose the terminal font. Design for a monospace grid and assume that glyph coverage varies.

Use these text roles:

| Role | Treatment | Example |
| --- | --- | --- |
| Brand | Bold, uppercase, dark on accent | `ALIAS LENS` |
| Screen title | Bold `Text`, sentence case | `Choose an alias to run.` |
| Section label | Bold semantic color, uppercase | `SUGGESTED FOR YOU` |
| Alias name | Bold `Accent` | `gpf` |
| Command | `Secondary`, normal weight | `git push --force-with-lease` |
| Description | `Text`, normal weight | `Safely force-push the current branch` |
| Metadata | `Muted` | `#git #daily` |
| Status | Semantic color, concise sentence | `Footer saved` |
| Control hint | `Muted` with actionable words in `Secondary` | `enter run  ·  esc quit` |

Write compact, literal copy. Name the object and the action. Prefer `Could not save the footer` to `Something went wrong`. State when an action changes a file, runs a command, restores a revision, or talks to a remote service.

Use `…` only for truncated content or an operation in progress. Use `·` to separate parallel metadata and footer controls. Do not use decorative ASCII banners, gradients made from characters, or emoji in product controls. A user-configured footer emoji is the exception.

## Terminal art and marker system

The full brand mark is a deliberate three-row composition used only on spacious welcome and empty states. Dense headings and rows use a semantic marker followed by a text label. The user can choose `symbols`, `ascii`, or `none`; the default is `symbols`.

Follow these rules when adding a marker:

- Provide one ordinary Unicode symbol and one ASCII fallback.
- Keep the symbol to one terminal cell when practical and test its rendered width.
- Give it one stable semantic name.
- Place a text label after it in headings and status content.
- Use the marker at the current text color unless it carries a defined semantic state.
- Do not put markers in every row. The alias-card command and exceptional metadata are enough.
- Do not use Nerd Font, private-use, or font-specific glyphs in the default interface.

Markers aid scanning. They never replace words, arrows, or keyboard labels. The old 4 by 2 bitmap renderer remains available only for existing footer settings. The footer editor does not offer bitmap input.

### Icon construction specification

| ID | Requirement | Exact rule |
| --- | --- | --- |
| `ICON-01` | Marker choices | Every semantic marker provides a symbol and ASCII form and can be hidden. |
| `ICON-02` | Label spacing | Put exactly one plain space between a heading marker and its text label. |
| `ICON-03` | Text alternative | A semantic marker must share its line with a label, except a favorite mark whose alias metadata exposes the same state. |
| `ICON-04` | Compatibility | Built-in symbols use ordinary Unicode. Nerd Font and private-use glyphs are forbidden. |
| `ICON-05` | Density | A normal heading may contain one marker. A card may contain command, function, favorite, and health markers only when those states exist. |
| `ICON-06` | Full art | Use the multirow brand mark only in a spacious welcome or empty state. |
| `ICON-08` | Custom footer icon | Offer named icons and `none` in the footer editor. Keep existing emoji and 4 by 2 bitmap values readable for compatibility. Reject controls and multiline values. |

## Components

The measurements below describe the rendered component, excluding the page's outer inset. Width values use terminal cells.

### Brand header

Render `ALIAS LENS` in the accent block. The `compact` choice adds a one-row mark; `text` and `full` use the product name alone in standard headers; `none` hides the brand block. Follow the brand with the active alias-file path in `Secondary`, then alias and issue counts in `Muted`.

Keep the header to one line. Truncate low-priority details before the path or brand.

| Property | Specification |
| --- | --- |
| Height | Exactly one row. |
| Brand padding | One cell at the left and right inside the accent background. |
| Brand label | `ALIAS LENS`, uppercase and bold. |
| Wide form | Configured compact mark and label, or label alone. |
| Narrow form | Configured label when visible. |
| Gap after brand | Two spaces. |
| Path | Active Bash or Zsh alias display path in `Secondary`. |
| Details | Two spaces, `•`, two spaces, then each muted count. |
| Priority on overflow | Keep brand, then path, then alias count, then issue count. |
| Forbidden content | Version, provider, sync animation, advertising, and multiline status. |

### Search field

Use a rounded one-cell border in `Accent`, one cell of horizontal padding, and the search icon in `Accent`. The input owns the normal typing focus on the alias and help pages. A blinking block cursor appears only while the terminal and field have focus.

Do not add a permanent label above the field when the screen title already explains the search scope. Placeholder text must describe the searchable content, not repeat `Search` alone.

| Property | Specification |
| --- | --- |
| Height | Three rows including the top and bottom rounded borders. |
| Width | `contentWidth - 3` in the standard page. |
| Interior padding | One cell at the left and right. |
| Border | Rounded, one cell, `Accent`. |
| Prefix | Four-cell search or help icon, then one space. |
| Text | User input in the primary text role. Placeholder in `Muted`. |
| Cursor | One block cell, visible only during the focused half of the 500 ms blink cycle. |
| Empty state | Show scope-specific placeholder text. Do not show a blank box. |
| Overflow | Keep the cursor visible and clip or scroll the query by terminal-cell width. Do not split a grapheme. |
| Lost focus | Hide the cursor immediately. Keep the query unchanged. |

### Alias card

An alias card contains three rows in this order:

1. Pointer, alias name, category badge, type, favorite, and tags.
2. Plain-language description.
3. Pixel command icon and the expanded command.

Use a thick left border only. Inactive cards use `Border`. The selected card uses `Accent` and a `▶` pointer. This redundant focus signal is required.

Keep category badges compact, uppercase, and dark-on-color. Wrap the description. Truncate the command on the card rather than wrapping it into neighboring cards. Show health issues on a separate coral line below the command.

| Property | Specification |
| --- | --- |
| Width | `contentWidth - 3`, with a lower bound of 34 cells. |
| Base height | Three content rows plus any description wrap and health row. |
| Interior padding | One cell at the left and right. |
| Border | Thick left edge only. No top, right, or bottom edge. |
| Inactive state | `Border` edge and two-space marker. |
| Active state | `Accent` edge and `▶ ` marker. |
| Name | Bold `Accent`; never truncate before optional metadata. |
| Category | Uppercase badge with one cell of horizontal padding. |
| Function | Pixel function icon plus `FUNCTION` in `Files`. |
| Favorite | Pixel favorite icon in `Dev`; do not add the word when horizontal space is tight. |
| Tags | Each tag starts with `#`; join tags with one space; render in `Muted`. |
| Description | Wrap on grapheme boundaries in `Text`. Preserve words when a word fits on a line. |
| Command | Pixel command icon, one space, then the command in `Secondary`. Truncate with `…`. |
| Issue row | Pixel health icon, one space, then issue texts separated by ` · ` in `Git`. |
| Ordering | Pointer, name, category, type, favorite, tags; then description; then command; then issues. |

### Rows and tabs

Use a filled `Selected` row with a `▶` pointer for dense pickers such as themes and settings. Use bracketed accent text for top-level stats views and an accent-filled badge for the selected time period. Unselected options stay `Muted`.

Do not introduce a third selection style without revising this specification. The existing styles cover cards, dense rows, and compact tabs.

Dense rows must span `contentWidth - 2`, use one cell of horizontal padding, and remain one row tall. The selected row uses `Text` over `Selected`, bold text, and the `▶ ` marker. The unselected row uses `Muted` and a two-space marker.

Compact tabs use one of two established forms:

- A selected period uses bold `Background` text on an `Accent` fill with one cell of horizontal padding.
- A selected stats view uses `Accent` text inside literal square brackets.
- Unselected periods and views use `Muted`.
- A tab label must contain a key when a single key selects it, such as `1 all`.
- Arrow-key movement must update selection without changing the established tab order.

### Forms

Align short uppercase labels in one column. Render fields on `Panel` with a thick left border. The focused field uses the accent border and a block cursor. Empty, unfocused fields show a muted example.

Keep the save and cancel controls visible. Validation errors replace or precede status content near the controls. An invalid form stays open and preserves the user's input.

| Property | Specification |
| --- | --- |
| Label column | 14 cells in the current add and edit form. |
| Field width | At least 20 cells; normally `contentWidth - 20`. |
| Field surface | `Panel` background with one cell of horizontal padding. |
| Field edge | Thick left border in `Border`; `Accent` while focused. |
| Field cursor | Solid accent block after the current value. |
| Empty field | Muted field-specific example while unfocused. |
| Focus order | Top to bottom. `Tab` and `Enter` move forward; `Shift+Tab` moves back. |
| Save | The configured save shortcut. Save validates every field before a file write. |
| Cancel | `Esc`. Restore preview-only settings and leave saved data unchanged. |
| Error | Specific sentence near the footer in the risk or status color. Keep focus on the failing field when known. |

### Confirmations

Use a full-page decision view for risky execution, deletion, and revision restore. Name the object, show the affected command or revision, explain the specific risk, then offer explicit confirm and cancel keys.

Coral marks the reason for concern. Amber may mark the item name. The confirmation must not rely on either color. Never preselect the destructive action, and keep `Esc` as cancel.

| Property | Specification |
| --- | --- |
| Context | Keep the normal brand header so the user does not lose place. |
| Title | Pixel health or task icon plus a literal review or confirmation label. |
| Object | Name the alias, file, or revision in text. |
| Preview | Use a `Panel` block with two cells of horizontal padding and one row of vertical padding when showing a command. |
| Risk edge | Thick coral left border. |
| Reason | State the detected reason. Do not use a generic warning when a specific reason exists. |
| Confirm key | A single explicit key such as `y`, paired with the action verb. |
| Cancel keys | `n` and `Esc` where a yes or no decision applies. |
| Default | No action occurs until the confirm key arrives. Resize, blur, and unrelated keys do not confirm. |
| Completion | On cancel, return to the prior page with a short status. On confirm, perform only the named action. |

### Stats and charts

Stats may use denser layouts than the alias browser, but they keep the shared header, pixel title, theme roles, controls, and maker credit. Charts must include labels or values. Color can group data, but it cannot be the sole key.

Keep chart marks simple enough to survive ANSI color reduction. Avoid braille patterns when the same chart can use blocks with clearer cell width.

The stats page uses two cells of horizontal outer padding. It pins the header, view tabs, period tabs, source note, local controls, optional global controls, and maker credit. The chart or ranking panel consumes the flexible height between them.

| State | Required content |
| --- | --- |
| Overview | Total use count, alias count, ranked rows, selected-row detail when space permits, and period. |
| Activity | Labeled time buckets, visible magnitude, and selected timeframe. |
| Cleanup | Alias name, age or last-use meaning, and the active age filter. |
| Groups | Category or group labels plus values. Color alone cannot identify a group. |
| Empty | Name the selected period and state that no use data exists for it. |
| Error | `Stats could not load.` plus a safe, truncated reason. |
| Source note | `Counts come only from the active terminal history`. |

### Footer

Place context-specific controls first. Show global navigation on a second line only when the width supports it. Keep the most important actions at narrow widths: primary action, help, and cancel or quit.

Separate controls with two spaces, `·`, and two spaces. Use the platform's configured shortcut labels. Do not claim that a shortcut works if the terminal or active mode cannot deliver it.

Place the maker credit on the last available line. Render its text in `Muted` and its icon in coral. User customization can change the text, icon, and left, center, or right alignment, but cannot move the credit above the control footer.

| ID | Requirement | Exact rule |
| --- | --- | --- |
| `FOOT-01` | Local controls | The first footer row lists actions available in the current context only. |
| `FOOT-02` | Order | Navigation or movement, primary action, secondary action, help, then cancel or quit. Omit unavailable groups. |
| `FOOT-03` | Separator | Use `  ·  ` between groups. Do not use vertical bars. |
| `FOOT-04` | Highlight | Local action words may use `Secondary`; keys and surrounding text remain readable without color. |
| `FOOT-05` | Narrow priority | At the narrowest width keep the primary action, `? help`, and `esc` behavior. |
| `FOOT-06` | Global controls | Show global page navigation on the next row only at 79 content cells or wider and only outside restricted modes. |
| `FOOT-07` | Status replacement | A transient status may replace local controls for one render, but destructive confirmation text must remain visible until resolved. |
| `FOOT-08` | Maker position | Place the maker credit below every control and status row using the configured alignment. |
| `FOOT-09` | Maker validation | Limit configured copy to 80 cells, reject controls and line breaks, and allow `{icon}` at most once. |
| `FOOT-10` | Plain output | Never add the maker credit to noninteractive output. |

## Responsive behavior

Responsive changes remove secondary information before they compress primary content.

| Available content width | Behavior |
| --- | --- |
| 40 to 47 cells | Omit the brand icon. Use the shortest control footer. Keep one useful result visible. |
| 48 to 78 cells | Keep page icons and normal cards. Omit the global navigation line. Shorten verbose control labels. |
| 79 to 95 cells | Add the global navigation line when the current mode allows it. |
| 96 to 108 cells | Use full control wording and metadata. Do not exceed the 108-cell content cap. |

Height controls list windows, not component scale. Preserve the header, current task, focused item, local controls, and maker credit. Page or scroll the list around the cursor. Do not shrink card padding or merge semantic rows to fit one more result.

A resize must preserve the selected object, query, form contents, preview state, and pending confirmation.

### Overflow and truncation specification

| Content | Required behavior |
| --- | --- |
| Brand | Never truncate `ALIAS LENS`. Omit the brand icon first. |
| Alias-file path | Prefer a recognizable display path. Truncate at a grapheme boundary if the header cannot fit. |
| Counts | Remove issue count before alias count. Do not wrap header counts. |
| Alias name | Preserve the full name when possible. Remove optional tags and favorite display before truncating the name. |
| Description | Wrap to additional lines. Do not truncate a description merely to keep every card three rows tall. |
| Command in a card | Keep one row and truncate with `…`. The detail or confirmation view may wrap the full command. |
| Tags | Remove tags from the right as complete units. Never split a tag. |
| Status | Wrap when the next action depends on the message. Truncate low-priority success status to the content width. |
| Footer controls | Switch to the defined shorter footer. Never cut a control label at the terminal edge. |
| Table or chart label | Truncate labels with `…`, but keep the associated numeric value visible. |

Truncation must reserve one cell for `…`. If only one cell is available, render `…`. A wrap or truncation operation must cap its work on hostile zero-width or combining input.

## Screen contracts

Every screen uses the shared components above. This section defines what each screen must communicate and which variation it may use.

### Alias browser

Purpose: find, inspect, create, edit, or run an alias.

| Region | Required content |
| --- | --- |
| Header | Brand, active alias-file path, total alias count, and issue count when nonzero. |
| Title | Current mode in one sentence: browse, select, execute, health, or empty. |
| Supporting line | Consequence of `Enter` and `Esc`, or the search scope. |
| Primary control | Search field. It receives typing focus unless a protected state is open. |
| Content | Suggested aliases for an empty query, filtered matches for a query, or health results in health mode. |
| Local controls | Movement, current `Enter` action, prompt edit when allowed, help, and cancel or quit. |

The browser has these distinct modes:

| Mode | `Enter` | `Tab` | `Esc` | Visual wording |
| --- | --- | --- | --- | --- |
| Browse | Selects according to the normal launch contract. | Returns the alias for prompt editing where supported. | Quits. | `Find the shortcut before you forget it.` |
| Execute | Runs the selected safe alias or opens risk review. | Returns the alias for prompt editing. | Leaves without running. | `Choose an alias to run.` |
| Select | Returns the selected alias without executing it. | No edit action. | Returns without changing the prompt. | `Choose an alias to use in your shell.` |
| Health | Acts on the selected problematic alias under the parent mode. | Follows the parent mode. | Returns to the full list. | `ALIAS HEALTH` section label. |

For an empty query, the list heading is `SUGGESTED FOR YOU`. For a nonempty query, do not add a redundant results heading. Show `showing X-Y of Z` when the result window omits matches.

### Empty alias library

Purpose: give the user one safe next action when the active alias file has no entries.

- Use the alias icon and `Set up your first shortcut.` outside select mode.
- State the active display path and shell at 60 content cells or wider.
- Offer `Enter` or the active profile's Add shortcut to create the first alias when creation is allowed.
- Offer reload as a secondary action.
- In select mode, state that no alias is available and direct the user to open normal Alias Lens.
- Do not show an empty card, a zero-result message, or suggested heading.

### First-run tour

Purpose: teach the smallest useful set of controls once, then get out of the way.

- Keep the shared brand header and maker credit.
- Use the brand icon and `Alias Lens is ready.`
- Keep the promise specific: `Four keys are enough to get started.`
- Show exactly the initial actions needed to search, run, choose a theme, and open full help.
- Align key labels in `Accent` and explanations in `Muted`.
- Use `Enter` to start, `?` to dismiss and open the full guide, and `Esc` to dismiss.
- Mark the tour seen only after one of those explicit dismissal actions.
- If saving the seen state fails, keep the tour open and show the error.
- Do not reopen the tour after it is seen unless a future explicit command resets it.

### No search results

Purpose: confirm the exact query and provide recovery.

- Render `No alias matched "QUERY"` as the primary line.
- When close names exist, add `Did you mean` followed by the proposed names in `Accent`.
- Keep the search field focused so the next keystroke edits the query.
- Keep the standard footer. Do not turn `Enter` into an implicit suggestion acceptance.

### Keyboard guide

Purpose: make every current action discoverable without leaving the TUI.

| Property | Specification |
| --- | --- |
| Title | Help icon plus `Keyboard guide`. |
| Search | Rounded help-search field with `filter shortcuts…`. |
| Row | Key label in `Accent`, fixed key column, then muted action description. |
| Key column | 16 cells normally and 14 below 60 content cells. |
| Visible rows | At least three, bounded by terminal height. |
| Empty filter | State the unmatched help query. |
| Close | `?` or `Esc`. `Ctrl+C` quits the application. |
| Footer | Show visible and total counts when the list is clipped. |

The guide must derive keys from the same shortcut profile and action definitions as update handling. Documentation-only shortcuts are defects.

### Theme picker

Purpose: preview and save a complete theme.

- Use the theme icon and `Choose a theme`.
- State that movement previews, `Enter` saves, and `Esc` restores.
- Render one dense row per theme with the selected-row pattern.
- End each row with the five ordered swatches defined by the theme specification.
- Keep the selected theme near the vertical center when the list pages.
- Show `showing X-Y of Z` when not all themes fit.
- Never save on cursor movement or page navigation.

### Appearance settings

Purpose: compose the maker credit with an immediate, reversible preview.

- Use the edit marker and `Compose your footer`.
- State that the preview is unsaved until the user chooses Save.
- Show `Message`, `Icon`, `Alignment`, `Tone`, and `Rule`. Keep the `ALIAS LENS` product name and interface fixed.
- Change Icon, Alignment, Tone, and Rule with Left and Right choices instead of requiring typed keywords.
- Give Message a visible insertion cursor and normal text-editing keys.
- Explain `{icon}` placement below `Message`.
- List the named icon choices below `Icon`.
- Update the pinned preview only after the candidate passes validation.
- On cancel or global page navigation, restore the saved footer.
- On save, use the private atomic config writer, close the page, and show `Footer saved`.

### Add and edit form

Purpose: change one complete alias definition without hiding stored metadata.

The fields appear in this order: `ALIAS NAME`, `COMMAND`, `WHAT IT DOES`, `TAGS`, and `CATEGORY`. Add mode uses the alias icon and `ADD AN ALIAS`. Edit mode uses the edit icon and `EDIT NAME`.

Each field has a specific example. The form must preserve platform, favorite, and other metadata that the visible edit does not change. Saving creates a backup and revision before the alias file changes. Visual success must follow a successful write, never precede it.

### Risk review

Purpose: stop a risky alias before execution and make the decision informed.

- Use the health icon and `Review before running`.
- Name the alias in amber text.
- State that the alias may make changes that are hard to undo.
- Show the complete command in the panel block, wrapped by cell width.
- Prefix the specific detector output with `Why:`.
- Offer `y run alias` and `n or esc cancel`.
- Never execute while parsing, rendering, resizing, or opening this screen.

### Revisions

Purpose: inspect private alias-file revisions and restore one deliberately.

- Use the history icon and a label that names revisions or versions.
- Render revision time, identity, and useful summary without exposing private command text by default.
- Use the dense selected-row pattern for revision movement.
- Require a second confirmation before restore.
- Name the revision in the confirmation and state that a new backup is created.
- On cancel, leave the live alias file unchanged.

### Sync status

Purpose: show what Alias Lens synchronizes and whether local and repository copies agree.

- Use the sync icon and `SYNC STATUS`.
- Show auto-sync state as a labeled badge. Do not use an unlabeled green or red dot.
- Show the primary alias file separately from extra tracked files.
- Each file card names both the source and repository destination when configured.
- File cards use the same left-edge selection language as alias cards.
- Error text must not include file contents, credentials, or secret values.
- Controls may refresh or start the existing safe sync action. The view itself performs no sync merely by opening.

### Repository picker

Purpose: choose a writable remote repository and make the following clone or selection action clear.

- Keep the shared brand header and use a title that names the configured provider task.
- Use the standard rounded search field.
- Show provider as an uppercase text badge with its semantic category color.
- Keep unavailable-provider explanations in a muted footer line.
- Replace controls with `Cloning and configuring repository…` during that operation.
- Keep credentials, tokens, and credential-bearing URLs out of every rendered state.

### Standalone stats

Purpose: inspect alias use without entering the main browser.

The standalone stats page omits the application header and global navigation. It retains the stats title, view tabs, period tabs, chart or table, source note, local controls, and maker credit. When stats open inside the browser, prepend the shared application header and add global navigation when width permits.

## State specification

Components must use a shared set of visual states. A component does not need to implement states that cannot apply to it.

| State | Visual treatment | Required text or behavior |
| --- | --- | --- |
| Rest | Normal semantic role. | Content remains readable without focus. |
| Focus | Accent border, pointer, brackets, fill, or cursor. | Exactly one typing or movement target owns focus. |
| Selected | Active card, row, or tab pattern. | Selection survives redraw and resize. |
| Empty | Title or message in `Text`, recovery in `Muted` and action roles. | Say what is empty and what the user can do. |
| Loading | Stable layout with an in-place status. | Name the operation in present-progressive form. |
| Success | `Accent` status text. | Name the completed action and changed object when useful. |
| Warning | `Dev` or amber with a label or sentence. | State the condition and safe next action. |
| Error | `Git` or coral with plain text. | State what failed and the command or action that can fix it. |
| Conflict | Coral text plus both sides named safely. | Leave the live alias file unchanged. |
| Disabled | `Muted`, never hidden when its absence would confuse. | Explain why the action is unavailable when selected. |
| Confirming | Full-page confirmation. | Name action, target, consequence, confirm key, and cancel key. |

Only one transient status owns the footer at a time. A destructive confirmation, validation error, or blocking conflict outranks success text. New status overwrites stale success after the next user action.

## Interaction and motion

Terminal motion should confirm state, not decorate the screen.

- Blink the search cursor at the existing 500 ms interval while the field has focus.
- Stop showing the cursor when the terminal loses focus or a secondary page opens.
- Preview a theme as the selection moves, then restore the previous theme on cancel.
- Update status text in place. Do not animate success messages.
- Use an explicit in-progress message for remote or long-running work.
- Avoid spinners for operations that usually finish before a person can read the first frame.

The alternate screen prevents the TUI from polluting shell history. Plain commands remain plain terminal text and do not inherit interactive styling.

## Accessibility and resilience

Every visual change must pass these checks:

- All actions are reachable with a keyboard.
- Focus has a non-color cue.
- Every icon that carries meaning has text.
- Every warning and error names the problem in text.
- Normal text meets the 4.5:1 target in its theme.
- `NO_COLOR=1` preserves content, selection, and controls.
- `TERM=dumb` produces useful plain output.
- Wide, combining, and emoji graphemes do not break alignment.
- Control bytes in user content render as inert text.
- A 48 by 18 terminal shows the current task, one useful content item, controls, and the maker credit.
- A screen below 48 by 18 shows the required and current sizes.

WCAG was written for the web, not terminal emulators. Alias Lens uses its color and contrast criteria as measurable targets, then verifies the result in real terminals because fonts, palettes, and color downsampling can change the output.

## Review a visual change

Before implementation, write acceptance criteria that name the affected screens, states, widths, and themes. For any change to a core trait, explain the user problem and the evidence that the current pattern causes it.

Review the implementation in this order:

1. Compare the change with the core-trait table.
2. Inspect the default Phosphor theme first.
3. Inspect success, empty, warning, error, confirmation, and in-progress states that the feature can reach.
4. Run the view at 48 by 18, a typical 80 by 24 size, and at least 120 columns wide.
5. Check one ANSI-limited terminal profile, `NO_COLOR=1`, and `TERM=dumb` where applicable.
6. Check Bash and Zsh entry paths when the screen can launch or return an alias.
7. Run `make fmt check`.
8. Record real-terminal evidence for changes to layout, focus, color, or key hints.

Reject a change when it does any of the following:

- Removes a required text label because an icon or color looks sufficient.
- Adds a new hard-coded color outside the theme roles.
- Hides cancel, back, or help at a supported size.
- Turns the TUI into a border-heavy grid or a web-style dashboard.
- Uses a font-specific glyph without a tested fallback.
- Changes several core traits under the name of cleanup or modernization.
- Makes screenshots look cleaner by removing information needed during use.

### Required visual evidence

Capture evidence for every changed screen and state. Text snapshots can prove labels, ordering, width, and ANSI absence. A real terminal inspection is still required for focus, perceived contrast, cursor behavior, and redraw quality.

| Axis | Required cases |
| --- | --- |
| Size | 48 by 18, 80 by 24, and 120 by 30 or wider. |
| Color | Phosphor true color, one ANSI-256 profile, and `NO_COLOR=1`. |
| Content | Short ASCII, a long command, CJK text, a combining sequence, and one emoji where user content allows it. |
| Data state | Populated, empty, filtered to none, warning or error, and loading when applicable. |
| Interaction | Rest, focused, selected, confirmation, cancel, completion, blur, refocus, and resize when applicable. |
| Entry path | Main `al` TUI plus picker or standalone entry points changed by the work. |
| Platform | Linux terminal and macOS terminal for layout work. Add Windows Terminal under WSL when shortcut labels or terminal behavior change. |

Use stable fixtures with synthetic aliases. Evidence must not contain a real alias file, shell history, path, provider token, repository URL with credentials, or local config.

### Automated test mapping

| Specification area | Minimum automated evidence |
| --- | --- |
| Frame and breakpoints | Table-driven views at boundary widths and heights. Assert maximum line width and required regions. |
| Theme | Contrast calculation for every built-in preset, default-token values, save and restore behavior, and `NO_COLOR`. |
| Icons | Parse validity, exact four-cell rendered width, text label presence, and invalid custom values. |
| Cards | Golden or structured assertions for active, inactive, function, favorite, tags, health, long text, and Unicode. |
| Search | Focus and blur, cursor blink, grapheme-safe editing, placeholder, no-results recovery, and resize. |
| Forms | Focus order, examples, validation, save, cancel, retained input after error, and private atomic writes. |
| Confirmations | Unrelated keys do nothing, `Esc` cancels, confirm key acts once, risk reason renders, and resize cannot confirm. |
| Navigation | Every source page to every allowed destination, current-page toggle, protected-state restrictions, and shortcut profiles. |
| Footer | Narrow and wide controls, exact maker alignment, customization validation, and absence from plain output. |
| Plain fallback | `TERM=dumb`, redirected output where applicable, no ANSI control sequence, and actionable next command. |

### Design-change record

For a change to a core trait, add a short decision record beside the feature's acceptance criteria. Use these fields:

```text
Core trait affected:
User problem:
Evidence from the current interface:
Proposed change:
Traits kept unchanged:
48x18 result:
80x24 result:
120x30 result:
NO_COLOR result:
Migration or compatibility effect:
Approval:
```

Do not approve a core change from a single ideal-width screenshot. The record must show how the change behaves at all required widths and without color.

## Implementation anchors

The current implementation lives in these files:

- `cmd/alias-lens/tui.go` owns the main page, alias cards, forms, confirmations, responsive rules, and shared styles.
- `cmd/alias-lens/theme.go` owns theme tokens, the Phosphor default, theme validation, and contrast calculations.
- `cmd/alias-lens/tui_icons.go` owns the 4 by 2 pixel-icon format and renderer.
- `cmd/alias-lens/tui_footer.go` owns the maker credit and its configuration.
- `cmd/alias-lens/stats_tui.go` owns the stats page and chart frame.
- `cmd/alias-lens/github_picker.go` owns the provider-neutral repository picker.
- `cmd/alias-lens/accessibility_test.go` covers cell width, small terminals, plain output, and default-theme contrast.

Move repeated visual behavior into shared helpers or semantic style types before adding another local variant. A future shell adapter must not create a different visual dialect.

## Research basis

This specification uses the current Alias Lens implementation as its primary source. External guidance informed the measurable accessibility and terminal-behavior rules:

- The [Command Line Interface Guidelines](https://clig.dev/) recommend human-first output, intentional color, concise information, clear state changes, and actionable help.
- The [Lip Gloss documentation](https://github.com/charmbracelet/lipgloss) documents terminal color-profile downsampling, adaptive color, and cell-based layout tools.
- The [Bubble Tea documentation](https://github.com/charmbracelet/bubbletea) describes the state, update, and view model used by the interface.
- The [`NO_COLOR` convention](https://no-color.org/) defines the opt-out that Alias Lens honors.
- [WCAG 2.2 use of color](https://www.w3.org/WAI/WCAG22/Understanding/use-of-color) requires a non-color way to convey information.
- [WCAG minimum contrast guidance](https://www.w3.org/WAI/WCAG21/Understanding/contrast-minimum) defines the 4.5:1 normal-text target used here.
