# Server Logging System

The server exposes a structured logging pipeline that decouples simulation code from concrete output sinks.

## Architecture Overview

```text
[Simulation] → Publish(Event) → Router → Sinks
```

* **Publishers** live next to gameplay logic. They construct `logging.Event` instances and call `Publish` on the injected publisher.
* **Router** owns a bounded queue, enriches events with wall-clock timestamps, and fans the events out to enabled sinks asynchronously. It drops new events when the buffer is full and records a counter for observability.
* **Sinks** render events to a destination. The initial implementation ships with:
  * Console sink – parity with `log.Printf` style output.
  * JSON sink – newline-delimited structured events.
  * Memory sink – in-memory collector for unit/integration tests.

## Event Schema

Events share a common envelope via `logging.Event`:

| Field     | Description                                                      |
|-----------|------------------------------------------------------------------|
| `Type`    | Namespaced event type (e.g. `combat.attack_overlap`).             |
| `Tick`    | Simulation tick that triggered the event.                        |
| `Time`    | Wall clock timestamp populated by the router.                    |
| `Actor`   | Primary entity reference (`ID`, `Kind`).                         |
| `Targets` | Optional additional entities affected by the event.              |
| `Severity`| Severity hint (`debug`, `info`, `warn`, `error`).                 |
| `Category`| High-level subsystem key (combat, gameplay, system, …).          |
| `Payload` | Domain specific struct with deterministic fields.                |
| `Extra`   | Map for additive metadata attached by `WithFields`.              |

### Combat Events

| Event Type              | Payload                                   |
|-------------------------|-------------------------------------------|
| `combat.attack_overlap` | `ability`, `playerTargets`, `npcTargets`. |

The payload structure is defined in `server/logging/combat/attack.go`.

## Configuration

`logging.Config` controls router behaviour:

* `EnabledSinks` – list of sink identifiers to activate (`console`, `json`, `memory`).
* `BufferSize` – router channel capacity before events are dropped.
* `MinimumSeverity` – lower bound for forwarded events.
* `DropWarnInterval` – throttle interval for overflow warnings.
* `Fields` – contextual metadata automatically merged into `Event.Extra`.
* `JSON` and `Console` nested structs hold sink-specific options.

`logging.DefaultConfig()` mirrors the previous stdout behaviour by enabling only the console sink.

## Testing Support

The memory sink (`sinks.NewMemorySink`) stores events in-process and exposes `Events()`/`Reset()` helpers so tests can assert on emitted sequences.

## Shutdown

`Router.Close(ctx)` flushes the queue, waits for sink goroutines, and closes each sink with the provided context. The server defers this during shutdown to ensure no telemetry is lost.

