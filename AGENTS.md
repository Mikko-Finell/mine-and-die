# Agent Guidelines

This document applies to the entire repository. Use it as a quick onboarding reference when making changes.

## Project Summary
Mine & Die is a small realtime prototype:
- **Server:** Go 1.24 HTTP service with Gorilla WebSocket. It keeps all world state in memory, advances the simulation at ~15 Hz, and serves the static client.
- **Client:** Vanilla HTML/JS rendered on a `<canvas>`. The browser connects via WebSocket for intents and receives authoritative snapshots from the server.

## Directory Layout
- `server/` – Go module containing the hub, simulation loop, HTTP handlers, and regression tests.
  - `constants.go` – Shared world dimensions, tick rate, and heartbeat constants.
  - `main.go` – HTTP setup, route registration, and WebSocket session loop.
  - `hub.go` – Hub state, join/subscription lifecycle, simulation ticker, and broadcast plumbing.
  - `player.go` – Player structs plus facing derivation and intent helpers.
  - `movement.go` – Movement integration, obstacle resolution, and player collision separation.
  - `obstacles.go` – Procedural obstacle/ore generation and geometry utilities.
  - `effects.go` – Ability cooldown tracking, projectile advancement, and effect pruning.
  - `messages.go` – JSON payload contracts shared across HTTP and WebSocket endpoints.
  - `main_test.go` – Behavioural tests covering joins, intents, effects, and heartbeats.
- `client/` – Static assets served by the Go process.
  - `main.js` – Builds the shared state store, hooks up diagnostics UI, and kicks off input/render/network flows.
  - `network.js` – `/join` handshake, WebSocket management, outbound message helpers, heartbeats, and reconnect logic.
  - `input.js` – Keyboard handling that normalizes movement vectors and triggers actions.
  - `render.js` – Canvas drawing, interpolation, obstacle/effect rendering helpers.
  - `styles.css` & `index.html` – Minimal layout and markup.
- `docs/` – Living documentation for architecture, modules, and testing.
- `README.md` – Quick pitch, documentation map, and setup guide.

## Technologies
- Go 1.24 with the standard library and `github.com/gorilla/websocket`.
- ES modules running directly in the browser (no bundler).
- HTML/CSS for layout and diagnostics readouts.

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

## Pull Request Expectations
- Include a brief summary plus testing notes in your PR body.
- Run automated tests relevant to your change set (`cd server && go test ./...` at minimum).
- Document new features, endpoints, or gameplay rules in the appropriate doc file under `docs/`.
