# Agent Guidelines

This document applies to the entire repository. Use it as a quick onboarding reference when making changes.

## Project Summary
Mine & Die is a small realtime prototype:
- **Server:** Go 1.24 HTTP service with Gorilla WebSocket. It stages player commands, steps the world simulation at ~15 Hz, emits events, and serves the static client.
- **Client:** Vanilla HTML/JS rendered on a `<canvas>`. The browser connects via WebSocket, submits input/action/heartbeat commands, and receives authoritative snapshots from the server.

## Directory Layout
- `server/` – Go module containing the hub, simulation loop, HTTP handlers, and regression tests.
  - `constants.go` – Shared world dimensions, tick rate, and heartbeat constants.
  - `main.go` – HTTP setup, route registration, and WebSocket session loop.
- `hub.go` – Hub state, join/subscription lifecycle, command queue, simulation ticker, and broadcast plumbing.
- `simulation.go` – World data model plus command processing, event emission, and per-tick system execution.
- `player.go` – Player structs plus facing derivation and intent helpers.
- `npc.go` – Neutral enemy types, snapshots, and seeding utilities.
- `movement.go` – Movement integration, obstacle resolution, and player collision separation.
- `obstacles.go` – Procedural obstacle/ore generation and geometry utilities.
- `effects.go` – Ability cooldown tracking, projectile advancement, environmental hazards, and event reporting.
- `inventory.go` – Item stacks, slot management, and cloning helpers.
- `messages.go` – JSON payload contracts shared across HTTP and WebSocket endpoints.
- `main_test.go` – Behavioural tests covering joins, intents, effects, and heartbeats.
- `client/` – Static assets served by the Go process.
  - `main.js`
    - Owns the cross-module `store` describing DOM references, simulation state, network metadata, and diagnostics counters.
    - Boots the client by wiring diagnostics toggles, inventory UI, and world reset form interactions.
    - Coordinates startup sequencing: registers input handlers, initiates the WebSocket join handshake, starts render and heartbeat loops, and publishes latency/status updates.
    - Provides shared helpers (`setStatusBase`, `setLatency`, `updateDiagnostics`, etc.) consumed by other modules to mutate HUD copy and telemetry readouts.
  - `network.js`
    - Exposes `joinGame` / `resetWorld` entry points that orchestrate the `/join` fetch, instantiate the WebSocket, and request server-side world regenerations.
    - Maintains connection lifecycle: listens for open/message/close events, retries with exponential backoff, surfaces reconnect state to the `store`, and clears timers on shutdown.
    - Serializes outbound messages (intents, heartbeats, latency probes) while tracking bytes/messages sent for diagnostics.
    - Parses authoritative snapshots into the shared `store`, updates latency/heartbeat metrics, and mirrors NPC/effect/player payloads for rendering.
  - `input.js`
    - Subscribes to keyboard events, maintaining the active key set and most recent directional ordering to resolve diagonals deterministically.
    - Normalizes directional intent vectors, derives facing information, and notifies `network.js` when movement or action input changes.
    - Exposes helpers for simulated latency controls so QA can inject delays via the diagnostics panel.
  - `render.js`
    - Runs the animation frame loop, interpolating between authoritative snapshots and previous frame data for smooth motion.
    - Renders tile grid, players, NPCs, obstacles, and transient effects to the `<canvas>` using the dimensions from the shared `store`.
    - Draws HUD overlays such as player names, cooldown indicators, and inventory slots, and reconciles `store.display*` caches with real state to minimize allocations.
  - `styles.css` – Layout rules for the canvas, diagnostics drawer, and inventory controls.
  - `index.html` – Minimal markup bootstrapping the diagnostics panel, world reset form, canvas element, and ES module script tags.
- `docs/` – Living documentation for architecture, modules, and testing.
- `README.md` – Quick pitch, documentation map, and setup guide.

## Technologies
- Go 1.24 with the standard library and `github.com/gorilla/websocket`.
- ES modules running directly in the browser (no bundler).
- HTML/CSS for layout and diagnostics readouts.
- The js-effects runtime (vendored under `client/js-effects/`) powers high-fidelity combat visuals. Run `npm run build` from the
  repository root after changing anything in `tools/js-effects/` so the ESM build is synchronised into the client.

## Running & Testing
- Start the server from `server/` with `go run .`; it serves both APIs and static files on `:8080`.
- Visit `http://localhost:8080` to load the client.
- Execute `go test ./...` from within `server/` before submitting.

## Coding Conventions
- **Go:**
  - Keep functions short, direct comments describing _why_ they exist. Avoid restating obvious control flow.
  - Guard shared state with `Hub.mu`. Any multi-step mutation of hub maps/slices should happen while holding the mutex.
  - Prefer returning copies (`snapshotLocked`) when sharing state with callers.
- **JavaScript:**
  - Continue using the shared `store` object for cross-module coordination—avoid adding hidden globals.
  - ES module syntax only; no bundler-specific features.
  - Keep comments concise and purpose-driven.
- **General:**
  - Update the relevant markdown in `docs/` when changing behaviour that affects contributors or runtime assumptions.
  - Keep diagnostics (`/diagnostics`, HUD) in sync with new fields or metrics you add.
- **Effects:**
  - The js-effects `EffectManager` is the sole authority for effect lifecycles. Do not introduce
    per-type stores, timers, or cleanup sets in client code—use `syncEffectsByType` alongside
    definition-provided `fromEffect` helpers to mirror simulation state.
  - Register fire-and-forget visuals through `EffectManager.registerTrigger` and dispatch them via
    `triggerAll`. Trigger handlers should call `manager.spawn()` directly rather than creating ad-hoc
    maps or queues.
  - When adding or tweaking definitions, update the TypeScript sources under
    `tools/js-effects/packages/effects-lib`, run `npm run build`, and refresh the relevant guidance in
    `docs/architecture/client.md`.

## AI System Notes
- NPC behaviours live in JSON configs under `server/ai_configs/`. Run `gofmt` after touching any Go helpers and keep configs free of trailing comments so the embed loader stays simple.
- The runtime compiles configs into ID-based tables (`ai_library.go`) and evaluates them in `ai_executor.go`. Avoid reintroducing string lookups or dynamic dispatch inside the tick loop.
- Add table-driven tests in `server/ai_test.go` whenever you introduce new conditions, actions, or behaviours so deterministic runs remain covered.

## Pull Request Expectations
- Include a brief summary plus testing notes in your PR body.
- Run automated tests relevant to your change set (`cd server && go test ./...` at minimum).
- Documentation-only changes do not require running automated tests.
- Document new features, endpoints, or gameplay rules in the appropriate doc file under `docs/`.
