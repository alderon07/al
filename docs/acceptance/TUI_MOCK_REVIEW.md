# Terminal UI mock review

## What the mocks get right

- The file path, alias count, and issues appear together, so the source and its condition are clear.
- Search is the strongest control and its keyboard shortcut is visible before typing.
- Accent color marks selection and action; muted text carries supporting detail; warning color means a real problem.
- Tight rows and aligned values make aliases easier to compare than large cards.
- The footer states keys available on the current screen.

The mocks also show usage, sync, and health figures that cannot be assumed from an alias file. The terminal view must display only data it has loaded and keep the configured theme.

## Acceptance criteria for the first UI pass

- The main Bubble Tea view keeps search and alias selection as its primary workflow.
- The heading, search field, and list have a clear visual order at wide and narrow terminal widths.
- A compact overview states actual alias, favorite, and affected-alias counts when there is room; an empty file has no misleading statistics.
- Each list row keeps the alias name, command, description, category, favorite, context, and issue information available without executing the alias.
- Focus and warning states remain distinguishable without relying on color alone.
- The footer and maker credit remain in the viewport at 48x18, 80x24, and 120x36.
- `make fmt check` passes and the compiled UI is checked in a PTY at narrow and wide widths.

## Tall terminal acceptance criteria

- The alias list uses the vertical space available above the fixed footer instead of reserving a blank row between compact alias entries.
- A taller terminal shows more aliases from the same selection than a shorter terminal when more aliases exist.
- The selected alias, visible range, controls, and maker credit remain inside the viewport after resizing.
