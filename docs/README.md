# Mine & Die Documentation Hub

Welcome to the documentation set for the Mine & Die prototype. The project explores a browser-based, PvP-focused mining arena with an authoritative Go backend and a vanilla JavaScript client. This hub gives you the broad context and points you toward deeper dives for each part of the stack.

## Tech Stack at a Glance
- **Server:** Go 1.24, standard library HTTP + Gorilla WebSocket, in-memory simulation that broadcasts full snapshots with diff metadata for the patch-testing pipeline.【F:server/messages.go†L3-L33】
- **Client:** Static HTML, ES modules, `<canvas>` rendering, no build tooling required.
- **Protocol:** `/join` POST handshake followed by a single WebSocket channel for intents, actions, state snapshots, and heartbeats.

## Repository Layout
- `server/` – Authoritative simulation, HTTP handlers, and WebSocket hub. See [Server Architecture](./architecture/server.md).
- `client/` – UI, input, networking, and rendering modules served as static assets. See [Client Architecture](./architecture/client.md).
- `docs/` – Living documentation (this folder).
  - `architecture/` – Technical specs and engineering-focused docs.
  - `gameplay-design/` – Concept pillars, combat intentions, and progression loops.
- `README.md` – Quickstart instructions and high-level pitch.
- `AGENTS.md` – Contributor guidelines and conventions.

## Runtime Flow
1. The browser POSTs `/join` to create a player record and fetch the world snapshot, including actors, ground loot, and the current world configuration.【F:server/main.go†L175-L189】【F:server/messages.go†L3-L14】【F:server/ground_items.go†L12-L59】
2. The client immediately connects to `/ws?id=<player-id>` and receives a real-time `state` payload containing full entities, queued effect triggers, incremental patches, and sequence/resync metadata so the diff journal stays aligned.【F:server/main.go†L199-L302】【F:server/messages.go†L17-L32】
3. Keyboard changes emit `input` commands; click-to-move adds `path` / `cancelPath`; combat keys send `action`; debug helpers issue `console`; and the browser annotates each payload with its last-applied tick acknowledgement before enqueueing it for the next simulation step.【F:client/network.js†L905-L1200】【F:server/hub.go†L324-L407】
4. The Go hub advances the world ~15 times per second by draining staged commands, updating actors, replaying AI, resolving collisions, applying effects, updating hazards, journaling patches, and broadcasting the latest `state` frame.【F:server/constants.go†L5-L18】【F:server/hub.go†L589-L633】【F:server/simulation.go†L200-L372】
5. Both sides exchange heartbeats every ~2 seconds; acknowledgements refresh latency diagnostics, feed `/diagnostics`, and disconnect players that miss three intervals (~6 s).【F:client/network.js†L732-L1141】【F:server/constants.go†L15-L18】【F:server/main.go†L33-L70】【F:server/hub.go†L253-L283】

## Simulation Quick Facts
- World bounds: 2400×1800 pixels with obstacles and ore nodes generated from a configurable deterministic seed.
- Player speed: ~160 px/s with server-side clamping and separation to avoid overlap.
- Effects: Melee swings and fireballs live as time-bound rectangles; fire-and-forget triggers let the server hand off one-shot visuals without keeping placeholder effects alive.【F:server/constants.go†L5-L33】【F:server/effects.go†L192-L245】【F:server/world_mutators.go†L266-L301】
- Ground loot: defeated actors and debug commands drop inventory stacks that are merged per tile and mirrored to clients alongside other broadcast entities.【F:server/hub.go†L409-L495】【F:server/ground_items.go†L70-L166】【F:client/render.js†L338-L405】
- Disconnect policy: three missed heartbeats (~6s) remove the player from the hub.【F:server/constants.go†L15-L18】【F:server/simulation.go†L344-L372】

## Development Workflow
1. Install Go 1.24 (matching `go.mod`).【F:server/go.mod†L1-L6】
2. From `server/`, run `go run .` to start the combined API + static file server on `:8080`.
3. Open `http://localhost:8080` in a browser to load the client.
4. Edit JS or CSS directly; the Go server serves the files on refresh.
5. Run `go test ./...` to execute hub regression tests.

## Additional References
- [Networking](./architecture/networking.md) – Join flow, message contracts, diff metadata, and heartbeat policy.
- [Server Architecture](./architecture/server.md) – Tick loop, data structures, actions, and diagnostics.
- [Client Architecture](./architecture/client.md) – Store layout, rendering flow, and network helpers.
- [Effects & Conditions](./architecture/effects.md) – How the authoritative effect pipeline maps to client visuals.
- [Item System](./architecture/items.md) – Canonical definitions, catalog export, and fungibility-aware runtime integration.
- [Logging](./architecture/logging.md) – Publisher/router design plus event catalog.
- [AI System](./architecture/ai.md) – Behaviour configs, executor flow, and extension tips.
- [Stats System](./architecture/stats.md) – Layered attributes, mutation flow, and integration points for combat.
- [Testing & Troubleshooting](./architecture/testing.md) – Available test suites and common debugging tips.
- [Patch Migration Plan](./architecture/patch-migration-plan.md) – Current coverage and roadmap for the diff pipeline.
- [Playground Animations](./architecture/playground-animations.md) – How to author effect definitions and surface them inside the playground tool.

Keep this folder updated as the simulation evolves so newcomers can get productive quickly.
