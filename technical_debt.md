# Technical Debt Backlog

## Server

### Events are produced but never reach clients
- `Hub.advance` returns the `StepOutput` events from `World.Step`, yet `RunSimulation` discards them and `broadcastState` only sends the snapshot. The README still promises that the hub can fan events out alongside snapshots, so the current implementation leaves that feature half-finished and wastes the extra allocations each tick. Tighten this by either wiring an event channel to clients or deleting the unused plumbing to simplify maintenance.【F:server/hub.go†L296-L338】【F:README.md†L47-L52】

### Command queue has no backpressure or per-client limits
- Every network handler appends to `pendingCommands` under a mutex and the slice is only cleared once per tick. A single misbehaving client can spam movement or path commands and grow the slice without bound before the next simulation step. Add per-player caps or drop-older policies so the hub cannot be trivially DoS'd.【F:server/hub.go†L24-L25】【F:server/hub.go†L206-L237】【F:server/hub.go†L408-L425】

### World resets destroy all player progress
- `ResetWorld` rebuilds the world and reseeds every connected player via `seedPlayerState`, resetting inventory, health, and position to the defaults. The HTTP handler exposes this endpoint without documentation or confirmation, so a click in the diagnostics UI wipes everyone. Either persist player state across resets or document that this endpoint is destructive and meant for tests only.【F:server/hub.go†L92-L116】【F:server/main.go†L40-L118】【F:client/main.js†L315-L360】

### Procedural generation ignores the world RNG
- `World` owns an RNG for deterministic AI behaviour, but `generateObstacles` and `generateGoldOreNodes` spin up a fresh `rand.NewSource(time.Now())` each time. Resets therefore produce different obstacle layouts and make regression tests brittle despite the docs recommending deterministic layouts. Refactor these helpers to use `w.rng` (seeded during `newWorld`) so the configuration can be reproduced by injecting a known seed.【F:server/obstacles.go†L23-L160】【F:server/simulation.go†L68-L93】【F:docs/testing.md†L13-L18】

### Broadcast fan-out spawns ad-hoc goroutines
- Joins, disconnects, and even failed writes spawn `go hub.broadcastState(...)`. During churn we can end up with many concurrent broadcasters all snapshotting the world and looping over subscribers. Centralise broadcasting in the simulation loop (or queue work onto a single dispatcher) to avoid goroutine storms and duplicated snapshots.【F:server/hub.go†L72-L89】【F:server/main.go†L165-L189】【F:server/hub.go†L393-L405】

### HTTP surface is untested
- The regression suite instantiates `Hub` directly and never exercises the HTTP handlers for `/join`, `/ws`, `/world/reset`, or `/diagnostics`. Add integration tests with `httptest.Server` so routing, JSON contracts, and error paths stay covered—especially for reset world toggles and heartbeat acknowledgements.【F:server/main_test.go†L1-L80】【F:server/main.go†L18-L259】

## Client & Documentation

### Feature docs oversell current functionality
- The README markets permadeath, guild hierarchies, and persistent economies even though no corresponding code or data structures exist. This confuses contributors and players. Trim the marketing copy or add roadmap disclaimers so expectations match reality.【F:README.md†L1-L4】【F:README.md†L95-L126】

### World reset UX lacks guard rails
- The client exposes a "Restart world" form that calls `/world/reset` immediately and disables controls only after submission. There is no confirmation dialog or warning that inventories will be wiped (see the destructive behaviour above). Add UX affordances and contributor docs to prevent accidental nukes while debugging.【F:client/main.js†L315-L360】【F:client/network.js†L420-L452】

### Missing guidance on event stream usage
- Documentation still implies events can be consumed by the client, but the client only handles `state` and `heartbeat` payloads. Once the server-side event pipeline is wired (see above), remember to extend `docs/client.md` and `client/network.js` with the expected schema and rendering strategy so engineers know how to surface them.【F:client/network.js†L200-L276】【F:README.md†L47-L52】【F:docs/client.md†L18-L44】
