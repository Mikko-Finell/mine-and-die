# Mine & Die – Agent Handbook

This document applies to the entire repository. Use it as the primary orientation guide before making changes.

## Project Snapshot
- **Server (`server/`)** – Go 1.24 HTTP + WebSocket process that simulates the world, applies patches, and serves the static client.
- **Client (`client/`)** – Vanilla HTML/CSS/ESM bundle rendered in the browser. It connects via WebSocket, streams intents, and renders authoritative snapshots.
- **Shared data** – JSON configs under `server/ai_configs/`, `effects_producer_map.json`, and the vendored `client/js-effects/` build.
- **Tooling (`tools/`)** – Development helpers, including the TypeScript workspace that produces the `js-effects` build used by the client.

## Directory Guide
- `server/`
  - `main.go` wires HTTP handlers, static file serving, and the WebSocket session loop.
  - `hub.go` and `simulation.go` coordinate tick scheduling, command queues, patch application, and broadcast snapshots.
  - `patches.go` plus companions (`patch_apply.go`, `patch_pruning_test.go`, etc.) encode the world mutation system.
  - `effects_*.go`, `status_effects.go`, and `world_*` files describe combat math, procedural world seeds, and persistent world state.
  - `ai_*.go` load JSON configs into deterministic tables executed in `ai_executor.go`; keep them in sync with `server/ai_configs/`.
  - Tests live beside the code they exercise (e.g. `*_test.go`). Always finish by running `go test ./...` from the `server/` directory.
- `client/`
  - `main.js` boots the UI, wires DOM controls, and owns the cross-module `store` that tracks state, diagnostics, and network metadata.
  - `network.js` manages joins, WebSocket lifecycle, exponential backoff, and snapshot ingestion.
  - `input.js`, `heartbeat.js`, and `patches.js` normalise player input, drive heartbeats, and request world resets respectively.
  - `render.js` and `render-modes.js` render the tile grid, entities, transient effects, and HUD overlays to the `<canvas>`.
  - `styles.css` defines layout for the game view and diagnostics drawer; keep visual adjustments in sync with HUD expectations.
  - `__tests__/` contains Jest tests for client utilities. Run them with `npm test` from the repository root when you modify client logic.
- `docs/` holds contributor-facing documentation. Update it alongside behavioural or architectural changes.
- `tools/js-effects/` is a TypeScript workspace that outputs the `client/js-effects` bundle. Run `npm run build` at the repo root after editing anything inside `tools/js-effects/`.

## Working Agreements
- **Testing** –
  - `cd server && go test ./...` for any server-side change.
  - `npm test` for client changes that affect JS modules or shared utilities.
  - Documentation-only updates do not require automated tests, but proof-read and lint markdown when possible.
- **Formatting** –
  - Run `gofmt` (or rely on `go test`'s formatting checks) on touched Go files.
  - Client JavaScript and CSS follow the existing formatting style; keep imports sorted and prefer descriptive const names over globals.
- **Concurrency** – Guard shared hub state with `Hub.mu`. Snapshot state for readers instead of sharing mutable references.
- **Effects runtime** – Treat `EffectManager` as the source of truth for lifecycle, spawning, and cleanup. Do not create parallel effect registries in the client.
- **Diagnostics** – When you add new metrics or status fields, update both the server payloads and the client HUD/diagnostics panel so they remain consistent.
- **Documentation** – If behaviour, configuration formats, or workflows change, refresh the relevant files under `docs/` and, when necessary, this handbook.

## Pull Request Expectations
- Summarise the change and list the tests you ran in the PR body.
- Keep commits focused; group related changes and avoid drive-by edits.
- Include screenshots for noticeable UI changes when practical.
