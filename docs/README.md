# Mine & Die Documentation Hub

Welcome to the documentation set for the Mine & Die prototype. The project explores a browser-based, PvP-focused mining arena with an authoritative Go backend and a vanilla JavaScript client. This hub gives you the broad context and points you toward deeper dives for each part of the stack.

## Tech Stack at a Glance
- **Server:** Go 1.24, standard library HTTP + Gorilla WebSocket, in-memory simulation.
- **Client:** Static HTML, ES modules, `<canvas>` rendering, no build tooling required.
- **Protocol:** `/join` POST handshake followed by a single WebSocket channel for intents, actions, state snapshots, and heartbeats.

## Repository Layout
- `server/` – Authoritative simulation, HTTP handlers, and WebSocket hub. See [Server Architecture](./server.md).
- `client/` – UI, input, networking, and rendering modules served as static assets. See [Client Architecture](./client.md).
- `docs/` – Living documentation (this folder).
- `README.md` – Quickstart instructions and high-level pitch.
- `AGENTS.md` – Contributor guidelines and conventions.

## Runtime Flow
1. The browser POSTs `/join` to create a player record and fetch the world snapshot.
2. The client immediately connects to `/ws?id=<player-id>` and receives a real-time `state` payload.
3. Keyboard changes emit `input` messages, while combat keys send `action` messages. Each inbound message becomes a typed command queued for the next tick.
4. The Go hub advances the world ~15 times per second by draining queued commands, resolving collisions, applying effects, updating hazards, and broadcasting the latest `state`.
5. Both sides exchange heartbeats every ~2 seconds; the acknowledgements update latency diagnostics and missed heartbeats enqueue disconnect commands.

## Simulation Quick Facts
- World bounds: 2400×1800 pixels with obstacles and ore nodes generated from a configurable deterministic seed.
- Player speed: ~160 px/s with server-side clamping and separation to avoid overlap.
- Effects: Melee swings and fireballs live as time-bound rectangles; fire-and-forget triggers let the server hand off one-shot visuals without keeping placeholder effects alive.
- Disconnect policy: three missed heartbeats (~6s) remove the player from the hub.

## Development Workflow
1. Install Go ≥ 1.22.
2. From `server/`, run `go run .` to start the combined API + static file server on `:8080`.
3. Open `http://localhost:8080` in a browser to load the client.
4. Edit JS or CSS directly; the Go server serves the files on refresh.
5. Run `go test ./...` to execute hub regression tests.

## Additional References
- [Server Architecture](./server.md) – Tick loop, data structures, actions, and diagnostics.
- [Client Architecture](./client.md) – Store layout, rendering flow, and network helpers.
- [Effects & Conditions](./effects.md) – How the authoritative effect pipeline maps to client visuals.
- [Testing & Troubleshooting](./testing.md) – Available test suites and common debugging tips.

Keep this folder updated as the simulation evolves so newcomers can get productive quickly.
