# Logging Architecture

The logging subsystem decouples simulation code from telemetry fan-out. Gameplay modules
publish semantic `logging.Event` values while the router asynchronously pushes them to
configured sinks, preserving tick order without stalling the hub.

## Package Layout
- `server/logging/publisher.go` – core types (`Event`, `Publisher`, `EntityRef`) plus
  helpers such as `WithFields` for attaching shard metadata.
- `server/logging/router.go` – buffered dispatcher that enriches events, filters by
  severity/category, and forwards them to enabled sinks.
- `server/logging/config.go` – runtime configuration, default settings, and the
  pluggable clock interface used for timestamping.
- `server/logging/sinks/` – output adapters (`console`, `json`, `memory`).
- `server/logging/combat/helpers.go` – combat event helpers.
- `server/logging/status_effects/helpers.go`, `.../economy/helpers.go`, `.../lifecycle/helpers.go`, and `.../network/helpers.go` – additional domain packages covering status effects, item flow, session lifecycle, and acknowledgement telemetry. [server/logging/status_effects/helpers.go](../../server/logging/status_effects/helpers.go) [server/logging/economy/helpers.go](../../server/logging/economy/helpers.go) [server/logging/lifecycle/helpers.go](../../server/logging/lifecycle/helpers.go) [server/logging/network/helpers.go](../../server/logging/network/helpers.go) New domains should add their own packages under `server/logging/` with similar patterns.

Simulation code imports **only** domain helper packages (e.g. `logging/combat`) and the
`logging.Publisher` interface. The router and sink wiring happens at process startup in
`main.go`.

## Event Lifecycle
```
[Simulation Helpers] → Publish(Event) → Router (async, FIFO) → Sinks
```
1. **Producers** construct rich payloads via domain helpers and call `Publisher.Publish`.
   Helpers must set `Type`, `Tick`, `Actor`, `Severity`, `Category`, and any domain
   payload/extra fields.
2. **Router** accepts events through a bounded channel. It fills in `Event.Time` when the
   helper leaves it zero, merges static metadata from `Config.Metadata`, and applies
   severity/category filters before queueing.
3. **Fan-out** occurs on dedicated goroutines per sink. Each sink receives events in the
   same order the router accepted them. If a sink queue backs up, the router drops the
   newest event for that sink and increments `events_dropped_total`.
4. **Shutdown** drains the router queue, waits for sinks to finish, and flushes buffered
   writers. Always call `Router.Close` during process teardown to avoid losing events.

The router exports counters via `Router.MetricsSnapshot()` so diagnostics endpoints can
report throughput, drop rate, and sink failures.

## Configuration
`logging.Config` is the authoritative source for runtime behaviour:

| Field | Description |
| --- | --- |
| `EnabledSinks []string` | Names of sinks to wire (keys in the `available` map passed to `NewRouter`). Duplicate names are ignored. |
| `BufferSize int` | Queue size for the router and each sink. Must be positive. |
| `MinSeverity logging.Severity` | Lower bound for accepted events. Anything below is dropped before enqueueing. |
| `Categories []logging.Category` | Optional allow-list. When non-empty, events outside the list are ignored. |
| `JSON.MaxBatch` / `JSON.FlushInterval` | Sink-specific options for the JSON writer. Batching is currently handled within the sink by using a buffered writer and optional periodic flush. |
| `JSON.FilePath` | Placeholder for file-backed JSON output (the current startup wiring only uses stdout, but the sink honours any `io.Writer`). |
| `Metadata map[string]string` | Static key/value pairs merged into `Event.Extra` when not already present (useful for shard IDs, build info, etc.). |

`logging.DefaultConfig()` mirrors the legacy stdout behaviour: console sink only, buffer
size 1024, debug severity, and no category filter. Adjust the struct before constructing
the router in `main.go` to enable JSON or memory sinks.

### Enabling Sinks
Create the sink instances and hand them to `logging.NewRouter`:
```go
sinks := map[string]logging.Sink{
        "console": sinks.NewConsole(os.Stdout),
        "json":    sinks.NewJSON(jsonFile, cfg.JSON.FlushInterval),
        "memory":  sinks.NewMemory(),
}
router, err := logging.NewRouter(cfg, clock, fallbackLogger, sinks)
```
Sinks missing from `EnabledSinks` are closed immediately and counted in
`sink_disabled_total`. The router shares its buffer size with per-sink queues; increase
`BufferSize` if you expect bursty ticks.

## Available Sinks
- **Console (`sinks.Console`)** – matches the previous `log.Printf` format. Thread-safe
  and writes to any `io.Writer` (defaults to `io.Discard` when nil).
- **JSON (`sinks.JSON`)** – emits newline-delimited JSON objects with stable keys. Uses a
  buffered writer and optional periodic flush when `FlushInterval > 0`.
- **Memory (`sinks.Memory`)** – accumulates copies of events for assertions in tests. The
  `Events()` accessor returns a snapshot slice.

When adding a new sink, implement the `logging.Sink` interface (`Write` and `Close`) and
register it in the `available` map passed to `NewRouter`.

## Domain Helpers
Domain packages hide event construction from gameplay code. For example, combat swings
use `combat.AttackOverlap`:
```go
combat.AttackOverlap(ctx, publisher, tick, actor, targets, payload, extra)
```
Helpers should:
- Expose a strongly-typed payload struct that documents required fields.
- Set `Severity`/`Category` explicitly so filters behave predictably.
- Pass through optional `Extra` metadata to allow call sites to decorate events.

When introducing a new domain, create a sibling package under `server/logging/` and place
helpers plus payload types there. Keep event types additive: renaming or removing fields
should result in a new `EventType` key.

## Event Catalog
All event types must be documented to keep downstream sinks stable. Current coverage:

| Event Type | Helper | Payload | Description |
| --- | --- | --- | --- |
| `combat.attack_overlap` | `combat.AttackOverlap` | `AttackOverlapPayload` (`ability`, `playerHits`, `npcHits`) | Emitted when a combat ability hits multiple targets during a single tick. Actor/targets identify the source and impacted entities. |
| `combat.damage` | `combat.Damage` | `DamagePayload` (`ability`, `amount`, `targetHealth`, `statusEffect`) | Fired whenever an ability reduces a target's health. `statusEffect` is set when periodic effects (e.g. burning) apply the tick. |
| `combat.defeat` | `combat.Defeat` | `DefeatPayload` (`ability`, `statusEffect`) | Fired when damage reduces a target to zero health. Targets contain the defeated entity for downstream kill feeds. |
| `status_effects.applied` | `status_effects.Applied` | `AppliedPayload` (`statusEffect`, `sourceId`, `durationMs`) | Published when a status effect is first applied to an actor. Actor references the applier (if known); target references the recipient. |
| `lifecycle.player_joined` | `lifecycle.PlayerJoined` | `PlayerJoinedPayload` (`spawnX`, `spawnY`) | Signals that a new player has joined the shard along with their spawn coordinates. |
| `lifecycle.player_disconnected` | `lifecycle.PlayerDisconnected` | `PlayerDisconnectedPayload` (`reason`) | Signals that a player left the world. `reason` differentiates manual disconnects from heartbeat timeouts. |
| `economy.item_grant_failed` | `economy.ItemGrantFailed` | `ItemGrantFailedPayload` (`itemType`, `quantity`, `reason`) | Warn-level event emitted when inventories reject a grant (player seeding, NPC rewards, mining, etc.). The error string is attached via `Event.Extra`. |
| `economy.gold_dropped` | `economy.GoldDropped` | `GoldDroppedPayload` (`quantity`, `reason`) | Records gold piles spawned on the ground along with the reason (death, manual drop, etc.). [server/logging/economy/helpers.go](../../server/logging/economy/helpers.go) |
| `economy.gold_picked_up` | `economy.GoldPickedUp` | `GoldPickedUpPayload` (`quantity`) | Captures successful pickups of ground gold stacks. [server/logging/economy/helpers.go](../../server/logging/economy/helpers.go) |
| `economy.gold_pickup_failed` | `economy.GoldPickupFailed` | `GoldPickupFailedPayload` (`reason`) | Warns when a pickup attempt fails (out of range, not found). [server/logging/economy/helpers.go](../../server/logging/economy/helpers.go) |
| `network.ack_regression` | `network.AckRegression` | `AckPayload` (`previous`, `ack`) | Emitted when a client reports an acknowledgement lower than its prior value. [server/logging/network/helpers.go](../../server/logging/network/helpers.go) [server/hub.go](../../server/hub.go) |
| `network.ack_advanced` | `network.AckAdvanced` | `AckPayload` (`previous`, `ack`) | Debug event defined for acknowledgement progress (currently unused but available for future instrumentation). [server/logging/network/helpers.go](../../server/logging/network/helpers.go) |

Extend this table whenever new helpers are added.

## Testing & Diagnostics
- Unit tests can wire a `sinks.Memory` instance and assert against `Events()` without
  touching real IO.
- `Router.MetricsSnapshot()` exposes counters (`events_total`, `events_dropped_total`,
  `sink_errors_total`, `sink_disabled_total`) for health endpoints.
- When validating flushing or drop behaviour, use a fake clock (implementing `logging.Clock`)
  to produce deterministic timestamps.

## Migration Notes
Legacy `log.Printf` usage should gradually be replaced by domain helpers. Inject the
shared `logging.Publisher` into any subsystem that needs telemetry, prefer payload structs
over loose maps, and favour additive event schemas to avoid breaking sinks.
