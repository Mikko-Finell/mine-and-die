# Documentation Standards

These instructions cover every file beneath `docs/`.

- Keep architecture notes, gameplay rules, and setup guides aligned with the current implementation. When code or workflows change, update the affected docs in the same pull request.
- Prefer task-focused sections over narrative history. Each page should answer "what does a contributor need to know right now?"
- Use descriptive headings, ordered lists for processes, and tables for capability matrices when appropriate.
- Call out version requirements (Go, Node, npm) when they differ from defaults in the root README.
- Link to source files or configs using relative paths so future readers can navigate quickly.
- Run a quick spell-check or Markdown linter when you touch more than a sentence or two.
- If behaviour diverges temporarily (e.g., a feature flag), annotate it with TODOs that reference the tracking issue.
