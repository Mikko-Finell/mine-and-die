# Unified Effects Runtime

The Mine & Die effect system now runs end-to-end on the shared contract between
Go and JavaScript. The server quantises every ability into `EffectIntent`
records, the authoritative `EffectManager` spawns and advances
`EffectInstance`s, and lifecycle events flow through the journal so clients can
replay the exact spawn/update/end stream before handing the data to the
browser's effect manager. [server/effects_manager.go](../../server/effects_manager.go) [server/simulation.go](../../server/simulation.go)

## Server pipeline

### Intent production
- `World.Step` translates gameplay into contract intents. When melee attacks or
  projectiles trigger, the world enqueues intents for the manager instead of
  directly spawning legacy effects. [server/simulation.go](../../server/simulation.go)
- `effect_intents.go` mirrors every legacy trigger (melee swings, fireballs,
  burning visuals/damage, blood decals) into structured `EffectIntent`
  payloads so geometry, durations, and numeric parameters stay deterministic
  across the contract. [server/effect_intents.go](../../server/effect_intents.go)

### Manager execution
- `EffectManager.EnqueueIntent` collects intents each tick, while
  `RunTick` drains the queue, instantiates `EffectInstance` records, emits
  spawn/update/end envelopes, and evaluates end policies. Sequence counters per
  instance guarantee monotonic transport ordering. [server/effects_manager.go](../../server/effects_manager.go)
- `instantiateIntent` seeds delivery metadata (follow targets, geometry clones,
  replication policy) using the registered definition so the runtime knows which
  payloads to broadcast and how long the instance should live. [server/effects_manager.go](../../server/effects_manager.go)

### Definition hooks
- `defaultEffectDefinitions` enumerates contract behaviour for melee swings,
  fireballs, burning ticks/visuals, and blood decals, including client
  replication specs and lifecycle policies. Lifecycle ownership now comes from
  the Go contract registry: fire-and-forget visuals are marked client-owned at
  the contract level, the resolver derives `managedByClient` for every catalog
  entry, and the client consumes the boolean without heuristics. Server-driven
  projectiles, auras, and status visuals remain server-owned so clients retire
  them immediately when the contract signals completion. [server/effects_manager.go](../../server/effects_manager.go)
- `defaultEffectHookRegistry` wires definition hooks into the legacy world.
  Hooks call `resolveMeleeImpact`, spawn contract-managed projectiles, keep
  status visuals attached, apply burning damage, and ensure blood decals are
  registered so gameplay stays authoritative while the contract drives
  lifecycles. [server/effects_manager.go](../../server/effects_manager.go)
- World helpers reuse shared state when hooks fire: projectiles sync via
  `spawnContractProjectileFromInstance`, burning visuals attach to status
  records, and blood decals register through the existing effect list to keep
  telemetry aligned. [server/effects.go](../../server/effects.go)

## Journal and transport

- `World.recordEffectLifecycleEvent` forwards spawns, updates, and ends into the
  journal (after recording telemetry source metadata) whenever the manager emits
  an event. [server/simulation.go](../../server/simulation.go)
- The world runs the manager before legacy `advanceEffects`, optionally piping
  events to the hub so broadcasts and tests see the same batch the journal
  recorded. [server/simulation.go](../../server/simulation.go)
- The journal assigns monotonic sequences, drops out-of-order updates, remembers
  recently ended IDs, and captures the final per-instance cursor for transport
  idempotency. `DrainEffectEvents` clears staged batches while
  `SnapshotEffectEvents` exposes a copy for snapshot-only marshals. [server/patches.go](../../server/patches.go)
- Resync hints are raised from the journal when lost-spawn patterns occur;
  `Hub.scheduleResyncIfNeeded` consumes the hint, forces the next keyframe, and
  marks the upcoming state message with `resync` so clients rebuild cleanly. [server/patches.go](../../server/patches.go) [server/hub.go](../../server/hub.go)
- `Hub.marshalState` no longer ships the legacy `effects` snapshot; state and
  keyframe payloads rely solely on contract lifecycle batches. The message
  always bundles `effect_spawned`, `effect_update`, `effect_ended`, and
  `effect_seq_cursors` alongside the usual world payloads. [server/hub.go](../../server/hub.go)
- Joining the game returns a snapshot without lifecycle arrays; the hub
  immediately broadcasts a state message (flagged as a resync) so the client
  receives a keyframe followed by the first contract batch. [server/hub.go](../../server/hub.go)
- Runtime guardrails now run unconditionally; the contract effect manager and
  transport are the only execution path for gameplay and telemetry, eliminating
  the legacy rollbacks documented earlier in the migration. [server/simulation.go](../../server/simulation.go) [server/hub.go](../../server/hub.go)

## Client lifecycle ingestion

- The join handshake resets the cached lifecycle state, applies the batch that
  arrived with `/join` (often empty), and stores the summary so diagnostics know
  how many events landed during the handshake. [client/network.js](../../client/network.js)
- Every `state` message reuses the same path: patches are applied, triggers are
  queued, and `applyEffectLifecycleBatch` processes the lifecycle arrays while
  recording dropped/unknown events for debugging. [client/network.js](../../client/network.js)
- `applyEffectLifecycleBatch` normalises payloads, applies spawns, retries
  updates after the spawn pass, retires ended instances, merges cursor hints,
  and invokes an `onUnknownUpdate` callback for diagnostics when updates arrive
  before their spawn. [client/effect-lifecycle.js](../../client/effect-lifecycle.js)

## Rendering integration

- Contract-derived effects are tagged with `__contractDerived` metadata and
  grouped by type so the renderer can prioritise authoritative data over legacy
  fallbacks when building render buckets. [client/render.js](../../client/render.js)
- `ensureEffectManager` keeps a single shared js-effects `EffectManager`, hooks
  up default triggers (blood splatter), and mirrors instances into the store's
  registry for cross-module lookups. [client/render.js](../../client/render.js)
- `syncEffectsByType` hydrates js-effects definitions from lifecycle entries, so
  definitions with `fromEffect`/`onUpdate` implementations receive the exact
  contract payload and can react to spawn/update/end metadata without inventing
  state locally. [client/render.js](../../client/render.js)

## Telemetry and troubleshooting

- Server telemetry tracks spawned/updated/ended counts, active gauges, trigger
  throughput, journal drop reasons, and parity aggregates, exposing the data via
  `/diagnostics` so rollouts can monitor contract health. [server/telemetry.go](../../server/telemetry.go)
- Client diagnostics capture the latest lifecycle summary (including unknown
  updates) after every batch so observers can spot transport gaps quickly. [client/network.js](../../client/network.js) [client/effect-lifecycle.js](../../client/effect-lifecycle.js)

With this pipeline in place, new gameplay or cosmetic work should focus on
adding intents, extending definitions, and letting the existing lifecycle
transport deliver authoritative events end-to-end.
