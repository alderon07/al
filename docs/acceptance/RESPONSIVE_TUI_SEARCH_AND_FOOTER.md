# Keep search and footer proportional to the terminal

## Acceptance criteria

- Alias, help, and repository search fields have a maximum rendered width and shrink to fit the main content column on narrower terminals.
- Search input has a bounded query length. Long queries show their trailing text and cursor inside the field without widening the page.
- The main content column keeps its readable maximum width. A configured footer rule spans the full usable terminal width inside the frame padding.
- The maker credit aligns against that full footer width. Controls and navigation remain directly above the rule, and the credit remains directly below it.
- Main pages and the repository picker render within the viewport at 48×18, 80×24, 120×36, 160×40, and 200×50. Footer rows remain visible after live resizes.
- Search typing, cursor visibility, footer configuration, and page shortcuts keep working. PTY tests cover narrow and wide widths, and `make fmt check` passes.
