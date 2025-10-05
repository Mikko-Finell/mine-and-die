# Server Architecture

The Go service owns the authoritative world state and exposes three responsibilities:

1. **Session management** – `/join` creates players, `/ws` streams updates, and `/diagnostics` surfaces runtime metrics.
2. **Simulation** – a fixed-rate tick loop advances movement, effects, and collision cleanup.
3. **Static hosting** – the root handler serves the `client/` directory.

## Core Packages and Types
- `main.go` – Entire server implementation including the `Hub` type, HTTP handlers, and tick loop.
- `main_test.go` – Behavioural tests covering joins, intents, effects, and heartbeat handling.
- `inventory.go` – Item definitions plus helper methods for stacking, moving, and cloning player inventories.
- Dependencies: only the Go standard library plus `github.com/gorilla/websocket`.

### Hub Overview
The `Hub` struct tracks:
- `players` – map of player IDs to their live state (`playerState`).
- `subscribers` – active WebSocket connections keyed by player ID.
- `effects` – slice of in-flight ability payloads.
- `obstacles` – immutable slice shared with clients.
- Atomic counters for player/effect IDs.

`newHub()` seeds the obstacle list (walls + gold ore) and prepares maps.

### Tick Loop
`RunSimulation` spins a `time.Ticker` at `tickRate` (15 Hz). Each tick:
1. Calls `advance` with the elapsed delta.
2. Closes stale sockets (missed heartbeats).
3. Broadcasts the latest snapshot via `broadcastState`.

`advance` handles:
- Movement: `movePlayerWithObstacles` normalizes intent, clamps to bounds, and slides along walls.
- Separation: `resolvePlayerCollisions` iteratively pushes overlapping players apart.
- Effects: `advanceEffectsLocked` moves projectiles, expires them on collision, and mirrors remaining range in `Params`.
- Cleanup: removes players who miss `disconnectAfter` and prunes expired effects.

### Actions and Cooldowns
`HandleAction` dispatches to `triggerMeleeAttack` or `triggerFireball`:
- Melee: spawns a short-lived rectangular effect, records cooldown, and immediately checks for overlapping players.
- Fireball: spawns a projectile with velocity/duration; `advanceEffectsLocked` moves and expires it.

Both helpers share the `Effect` struct (`type`, `owner`, bounding box, `Params`) that is sent to clients for rendering.

### Inventory System
- Each `Player` carries an `Inventory` composed of ordered slots. The ordering is preserved in snapshots so clients can surface drag-and-drop later on.
- `ItemStack` values automatically merge when the same `ItemType` is added twice, supporting infinite stacking for resources like gold.
- `Inventory.MoveSlot` and `Inventory.RemoveQuantity` centralize reordering and stack splitting logic. Both operate while holding the hub mutex to keep state consistent.
- `Inventory.Clone` is used when broadcasting player snapshots to avoid data races between the simulation and JSON encoding.

### Heartbeats and Diagnostics
- Clients send `{ type: "heartbeat", sentAt }` ~every 2 seconds.
- `UpdateHeartbeat` stores the last heartbeat time, computes RTT, and keeps the connection eligible.
- `DiagnosticsSnapshot` returns minimal player heartbeat info for the `/diagnostics` endpoint.

### HTTP Endpoints
- `POST /join` – allocate a player, return `{ id, players, obstacles, effects }` snapshot.
- `GET /ws?id=...` – upgrade to WebSocket; first message is an immediate state snapshot.
- `GET /diagnostics` – JSON payload with tick rate, heartbeat interval, and per-player metrics.
- `GET /health` – simple liveness string.
- `GET /` – static file server rooted at `client/`.

### Extending the Server
- Add new player fields within `Player`/`playerState` and include them in `snapshotLocked`.
- Register new actions in `HandleAction` and encode behaviour in dedicated helpers.
- Extend `Effect.Params` for additional metadata—clients simply mirror the JSON.
- Preserve locking discipline: acquire `Hub.mu` before touching shared maps/slices.
- Update or add tests in `main_test.go` whenever gameplay rules change.

### Testing
Run `go test ./...` from the repository root or within `server/`. The suite focuses on:
- Join flow and snapshot integrity.
- Intent normalization and facing resolution.
- Movement clamping, obstacle avoidance, and player separation.
- Melee/fireball effect creation and cooldown enforcement.
- Heartbeat tracking and diagnostics output.
