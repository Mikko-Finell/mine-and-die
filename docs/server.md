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
- `npcs` – neutral enemies keyed by ID (`npcState`).
- `subscribers` – active WebSocket connections keyed by player ID.
- `effects` – slice of in-flight ability payloads.
- `effectBehaviors` – map of effect types to collision handlers (damage, healing, etc.).
- `obstacles` – immutable slice shared with clients (walls, gold ore, lava hazards).
- Atomic counters for player/effect IDs.

`newHub()` seeds the obstacle list (walls + gold ore), prepares maps, and calls `spawnInitialNPCs()` to place a baseline set of neutral enemies.

### Tick Loop
`RunSimulation` spins a `time.Ticker` at `tickRate` (15 Hz). Each tick:
1. Calls `advance` with the elapsed delta.
2. Closes stale sockets (missed heartbeats).
3. Broadcasts the latest snapshot via `broadcastState`.

`advance` handles:
- Movement: `moveActorWithObstacles` normalizes intent, clamps to bounds, and slides along walls for both players and NPCs.
- Separation: `resolveActorCollisions` iteratively pushes overlapping actors apart.
- Hazards: `applyEnvironmentalDamageLocked` ticks lava pools that remain walkable but burn players standing inside them.
- Effects: `advanceEffectsLocked` moves projectiles, expires them on collision, and mirrors remaining range in `Params`.
- Cleanup: removes players who miss `disconnectAfter` and prunes expired effects.

### Neutral Enemies
- NPCs reuse the shared `Actor` struct for position, facing, health, and inventories, and add fields like `Type`, `AIControlled`, and `ExperienceReward`.
- `spawnInitialNPCs` currently seeds a stationary goblin with gold and a potion; additional spawns simply append `npcState` entries while holding `Hub.mu`.
- Snapshots include a `npcs` array alongside the existing `players`, enabling the client to render and later target neutral enemies without special casing.

### Actions, Health, and Cooldowns
`HandleAction` dispatches to `triggerMeleeAttack` or `triggerFireball`:
- Melee: spawns a short-lived rectangular effect, records cooldown, immediately checks for overlapping players, and awards one gold coin when the swing overlaps a gold ore obstacle.
- Fireball: spawns a projectile with velocity/duration; `advanceEffectsLocked` moves and expires it.
- Lava pools: generated via `generateLavaPools`, they are communicated as obstacles but ignored by movement collision checks so players can wade through them while taking damage (`lavaDamagePerSecond`).

Players now track `Health` and `MaxHealth`. Both helpers share the `Effect` struct (`type`, `owner`, bounding box, `Params`) that is sent to clients for rendering. The hub registers per-effect behaviour in `effectBehaviors`; melee swings and fireballs publish a `healthDelta` parameter that is applied to every overlapping target. Positive values heal (clamped to `MaxHealth`), negative values deal damage (never dropping below zero). Adding new effect types means registering another behaviour keyed by the effect's `Type`.

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
- Spawn new NPC types by creating `npcState` records and adding them to `Hub.npcs` under the mutex.
- Preserve locking discipline: acquire `Hub.mu` before touching shared maps/slices.
- Update or add tests in `main_test.go` whenever gameplay rules change.

### Testing
Run `go test ./...` from the repository root or within `server/`. The suite focuses on:
- Join flow and snapshot integrity.
- Intent normalization and facing resolution.
- Movement clamping, obstacle avoidance, and player separation.
- Melee/fireball effect creation and cooldown enforcement.
- Heartbeat tracking and diagnostics output.
