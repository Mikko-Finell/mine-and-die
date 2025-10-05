# Agent Guidelines

This file applies to the entire repository.

## Repository layout
- `server/`: Go 1.24.3 HTTP service that keeps authoritative game state. `main.go` hosts `/join` (POST join handshake), `/ws` (WebSocket for realtime state, intents, and heartbeats), `/diagnostics` (JSON snapshot of hub state), `/health` (liveness), and serves the static client. `main_test.go` contains focused regression tests around hub behaviour and heartbeat handling.
- `client/`: Static HTML + vanilla JS front-end. `main.js` wires together DOM, state store, and modules. `network.js` contains fetch/WebSocket orchestration, intent dispatch, and heartbeat timers. `render.js` holds canvas drawing + interpolation. `input.js` translates keyboard events into intents and simulated latency adjustments. `styles.css` defines the simple HUD + canvas layout. `index.html` bootstraps modules via ES modules.
- `README.md`: High-level overview and setup instructions for running server/client locally.

## Design notes
- Networking relies on `fetch` for the `/join` handshake and a single WebSocket channel for state updates, intents, and heartbeats. Avoid introducing alternate transports unless necessary.
- The Go hub keeps all player state in-memory; prefer straightforward, single-threaded reasoning aided by the existing mutex/atomic primitives when extending it.
- Client visuals intentionally stay lightweight grid-and-square rendering backed by interpolation in `render.js`. Preserve this style unless a broader art direction change is coordinated.
- Client modules are dependency-free ES modules. Keep cross-module communication flowing through the shared `store` object passed from `main.js` to avoid hidden globals.

## Contribution tips
- Update `README.md` whenever you add new endpoints, diagnostics, or significant client interactions that affect gameplay expectations.
- Prefer small, incremental commits that align with features or fixes; include succinct messages describing the change.
