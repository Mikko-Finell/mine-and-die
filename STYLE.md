# Mine & Die Style Guide

## Go Code

1. **Standard formatting.** Always run `gofmt` (or your editor integration) before
   committing Go changes. Do not hand-format code.
2. **Explicit dependencies.** Avoid global state. Pass collaborators through
   constructor parameters or dedicated config structs (see `internal/app.Config`,
   `internal/net.HTTPHandlerConfig`).
3. **Small packages.** Keep packages focused on a single concern. Extract shared
   contracts into narrow packages (`internal/sim/patches`, `internal/journal`) and
   avoid cross-layer imports.
4. **Determinism first.** Any simulation change must preserve deterministic
   behavior; run the determinism harness and update fixtures only when behavior
   changes are intentional and reviewed.
5. **Error handling.** Return Go `error` values rather than logging inside library
   code. Callers decide whether to log, wrap, or propagate errors.

## TypeScript & React

1. **Function components + hooks.** Prefer modern React patterns over class
   components. Co-locate stateful hooks with related rendering logic.
2. **Type safety.** Use TypeScript interfaces/types sourced from the shared
   protocol definitions. Align client models with the `internal/net/proto`
   contracts to avoid drift.
3. **Testing.** Keep Jest tests near the components they validate. Ensure `npm test`
   passes before pushing changes.

## Documentation

1. **Keep mission context in sync.** When finishing a task in
   `docs/architecture/idiomaticity-mission.md`, mark it complete and note the next
   actionable follow-up.
2. **Task-focused pages.** Prefer concise guides (checklists, tables, step lists)
   over narrative history. Link to relevant packages or configs using relative paths.
3. **Version notes.** Call out deviations from the root `README.md` (Go version,
   npm version) when they matter for a workflow.
