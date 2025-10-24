# Mine & Die Architecture

## Overview

The Mine & Die runtime is organized around a Go server (`server/`) and a React-based
client (`client/`). The server owns the simulation, networking, and observability
infrastructure while the client renders state updates that the server streams over
websockets.

The Go workspace (`go.work`) places the server in its own module (`server/`). The
`cmd/server` package provides the entry point that wires dependencies together and
runs the simulation. Everything below `server/internal` is internal-only and follows
a strict layering model to keep the simulation decoupled from transports and tooling.

## Package Layers

| Layer | Packages | Responsibilities |
| --- | --- | --- |
| Process wiring | `server/cmd/server`, `server/internal/app` | Bootstrap configuration, dependency injection, process lifecycle. |
| Networking | `server/internal/net` | HTTP + websocket handlers, protocol translation, debug endpoints. |
| Observability | `server/internal/observability`, `server/internal/telemetry` | Logging, metrics, runtime profiling flags. |
| Simulation façade | `server/internal/sim` | Command queue, simulation loop, snapshot generation, patch typing. |
| Simulation internals | `server/internal/world`, `server/internal/effects`, `server/internal/items`, `server/internal/journal`, `server/internal/stats`, etc. | Game rules, world state mutation, effect bookkeeping. |

Supporting utilities that span multiple layers live under narrowly-scoped packages
(e.g. `server/internal/ai`, `server/internal/combat`, `server/internal/simutil`).

## Dependency Rules

The layering is enforced both socially and by tooling:

1. `internal/net` may depend on `internal/app`, protocol helpers, and shared
   utilities, but **must not** import simulation internals. The `make deps-check`
   target runs `server/tools/depscheck` to ensure no package under
   `internal/net/...` imports `internal/sim/internal/...`.
2. `internal/sim` exposes stable façades (`Engine`, `CommandBuffer`, typed patch
   structs) that callers must use instead of reaching into world state. Downstream
   packages use these façades to preserve deterministic behavior.
3. Simulation internals (`internal/world`, `internal/effects`, etc.) own their own
   data structures and must not depend on networking packages. Shared contracts are
   promoted to dedicated packages (e.g. `internal/journal`, `internal/sim/patches`).
4. Observability helpers (`internal/telemetry`, `internal/observability`) accept
   interfaces so callers inject logging and metrics implementations without adding
   new globals.

When creating new packages, choose the lowest layer that can own the logic and
prefer injecting dependencies instead of adding backdoors across layers.

## Testing Expectations

The following checks keep architectural guarantees and behavioral determinism in
place:

- `make test` — runs the TypeScript unit suite (`npm test`) and `go test ./...` in
  the server module. These tests must stay green before merging.
- `make deps-check` — enforces the networking → simulation dependency rule.
- `go test ./server -run DeterminismHarness` (automatically exercised by
  `go test ./...`) — validates that the golden simulation checksums remain
  unchanged, flagging any behavioral drift in the refactor.

Before landing changes, run `make test` and `make deps-check`. If you adjust
package boundaries, update `server/tools/depscheck` and this document to keep the
rules explicit.
