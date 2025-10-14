# Unified Effects Runtime

The Mine & Die effect system now runs end-to-end on the shared contract between
Go and JavaScript. The server quantises every ability into `EffectIntent`
records, the authoritative `EffectManager` spawns and advances
`EffectInstance`s, and lifecycle events flow through the journal so clients can
replay the exact spawn/update/end stream before handing the data to the
browser's effect manager.【F:server/effects_manager.go†L26-L172】【F:server/simulation.go†L433-L481】

## Server pipeline

### Intent production
- `World.Step` translates gameplay into contract intents. When melee attacks or
  projectiles trigger, the world enqueues intents for the manager instead of
  directly spawning legacy effects.【F:server/simulation.go†L433-L455】
- `effect_intents.go` mirrors every legacy trigger (melee swings, fireballs,
  burning visuals/damage, blood decals) into structured `EffectIntent`
  payloads so geometry, durations, and numeric parameters stay deterministic
  across the contract.【F:server/effect_intents.go†L48-L284】

### Manager execution
- `EffectManager.EnqueueIntent` collects intents each tick, while
  `RunTick` drains the queue, instantiates `EffectInstance` records, emits
  spawn/update/end envelopes, and evaluates end policies. Sequence counters per
  instance guarantee monotonic transport ordering.【F:server/effects_manager.go†L70-L172】
- `instantiateIntent` seeds delivery metadata (follow targets, geometry clones,
  replication policy) using the registered definition so the runtime knows which
  payloads to broadcast and how long the instance should live.【F:server/effects_manager.go†L174-L220】

### Definition hooks
- `defaultEffectDefinitions` enumerates contract behaviour for melee swings,
  fireballs, burning ticks/visuals, and blood decals, including client
  replication specs and lifecycle policies. Definitions that produce
  fire-and-forget visuals set `managedByClient: true` so the renderer keeps
  them alive just long enough to finish their local animation, while
  server-driven projectiles, auras, and status visuals leave the flag false so
  clients retire them immediately when the contract signals completion.【F:server/effects_manager.go†L805-L894】
- `defaultEffectHookRegistry` wires definition hooks into the legacy world.
  Hooks call `resolveMeleeImpact`, spawn contract-managed projectiles, keep
  status visuals attached, apply burning damage, and ensure blood decals are
  registered so gameplay stays authoritative while the contract drives
  lifecycles.【F:server/effects_manager.go†L377-L592】
- World helpers reuse shared state when hooks fire: projectiles sync via
  `spawnContractProjectileFromInstance`, burning visuals attach to status
  records, and blood decals register through the existing effect list to keep
  telemetry aligned.【F:server/effects.go†L779-L870】

## Journal and transport

- `World.recordEffectLifecycleEvent` forwards spawns, updates, and ends into the
  journal (after recording telemetry source metadata) whenever the manager emits
  an event.【F:server/simulation.go†L198-L212】
- The world runs the manager before legacy `advanceEffects`, optionally piping
  events to the hub so broadcasts and tests see the same batch the journal
  recorded.【F:server/simulation.go†L433-L481】
- The journal assigns monotonic sequences, drops out-of-order updates, remembers
  recently ended IDs, and captures the final per-instance cursor for transport
  idempotency. `DrainEffectEvents` clears staged batches while
  `SnapshotEffectEvents` exposes a copy for snapshot-only marshals.【F:server/patches.go†L237-L447】
- Resync hints are raised from the journal when lost-spawn patterns occur;
  `Hub.scheduleResyncIfNeeded` consumes the hint, forces the next keyframe, and
  marks the upcoming state message with `resync` so clients rebuild cleanly.【F:server/patches.go†L449-L459】【F:server/hub.go†L1138-L1157】
- `Hub.marshalState` no longer ships the legacy `effects` snapshot; state and
  keyframe payloads rely solely on contract lifecycle batches. When both the
  manager and transport flags are enabled the message bundles `effect_spawned`,
  `effect_update`, `effect_ended`, and `effect_seq_cursors` alongside the usual
  world payloads.【F:server/hub.go†L960-L1126】
- Joining the game returns a snapshot without lifecycle arrays; the hub
  immediately broadcasts a state message (flagged as a resync) so the client
  receives a keyframe followed by the first contract batch.【F:server/hub.go†L206-L244】
- Runtime guardrails remain toggleable at build time: `enableContractEffect*`
  flags keep the contract path on by default but allow targeted rollbacks when
  debugging transport or gameplay regressions.【F:server/constants.go†L35-L62】

## Client lifecycle ingestion

- The join handshake resets the cached lifecycle state, applies the batch that
  arrived with `/join` (often empty), and stores the summary so diagnostics know
  how many events landed during the handshake.【F:client/network.js†L1120-L1159】
- Every `state` message reuses the same path: patches are applied, triggers are
  queued, and `applyEffectLifecycleBatch` processes the lifecycle arrays while
  recording dropped/unknown events for debugging.【F:client/network.js†L1270-L1344】
- `applyEffectLifecycleBatch` normalises payloads, applies spawns, retries
  updates after the spawn pass, retires ended instances, merges cursor hints,
  and invokes an `onUnknownUpdate` callback for diagnostics when updates arrive
  before their spawn.【F:client/effect-lifecycle.js†L168-L415】

## Rendering integration

- Contract-derived effects are tagged with `__contractDerived` metadata and
  grouped by type so the renderer can prioritise authoritative data over legacy
  fallbacks when building render buckets.【F:client/render.js†L180-L215】
- `ensureEffectManager` keeps a single shared js-effects `EffectManager`, hooks
  up default triggers (blood splatter), and mirrors instances into the store's
  registry for cross-module lookups.【F:client/render.js†L225-L304】
- `syncEffectsByType` hydrates js-effects definitions from lifecycle entries, so
  definitions with `fromEffect`/`onUpdate` implementations receive the exact
  contract payload and can react to spawn/update/end metadata without inventing
  state locally.【F:client/render.js†L306-L360】

## Telemetry and troubleshooting

- Server telemetry tracks spawned/updated/ended counts, active gauges, trigger
  throughput, journal drop reasons, and parity aggregates, exposing the data via
  `/diagnostics` so rollouts can monitor contract health.【F:server/telemetry.go†L501-L565】
- Client diagnostics capture the latest lifecycle summary (including unknown
  updates) after every batch so observers can spot transport gaps quickly.【F:client/network.js†L1284-L1289】【F:client/effect-lifecycle.js†L405-L412】

With this pipeline in place, new gameplay or cosmetic work should focus on
adding intents, extending definitions, and letting the existing lifecycle
transport deliver authoritative events end-to-end.
