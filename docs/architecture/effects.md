# Unified Effects Runtime

The Mine & Die effect pipeline is now entirely contract-driven. The Go server
translates every melee swing, projectile, status rider, and cosmetic trigger
into a deterministic lifecycle stream, while the JavaScript client renders that
stream without inventing state. Use this document as the reference for how
intents flow through the authoritative manager, how lifecycle events move over
transport, and how the browser consumes them.

## Authoritative lifecycle (server)

- **Legacy hooks feed contract intents.** Helpers such as
  `NewMeleeIntent`, `NewProjectileIntent`, and
  `NewStatusVisualIntent` quantize legacy gameplay data into
  `EffectIntent` payloads so contract processing observes the exact
  geometry, params, and tick durations that the live simulation uses.【F:server/effect_intents.go†L48-L200】
- **`EffectManager` owns instances.** Each tick the manager drains the
  intent queue, instantiates `EffectInstance` records, invokes delivery
  hooks, and emits spawn/update/end envelopes with per-instance sequence
  counters. Instances track replication rules so definitions can opt out of
  updates or ends when they are cosmetic-only.【F:server/effects_manager.go†L14-L172】
- **Definitions & hooks keep parity.** The default registry wires contract
  lifecycle hooks (melee swings, fireball projectiles, burning visuals,
  blood decals) directly into the world state so telemetry and gameplay
  stay aligned without any legacy compat shim.【F:server/effects_manager.go†L47-L65】

## Lifecycle events & snapshot transport

- **Journal stores authoritative history.** `Journal.RecordEffectSpawn`,
  `.RecordEffectUpdate`, and `.RecordEffectEnd` assign monotonic sequence
  values, drop out-of-order events, and copy delivery/behaviour payloads
  into the effect event buffer. `DrainEffectEvents` returns spawns,
  updates, ends, and the current `LastSeqByID` map so downstream consumers
  can apply batches idempotently.【F:server/patches.go†L193-L431】
- **Resync hints are automatic.** The journal tracks lost-spawn patterns
  and raises a resync hint when unknown-ID updates or similar anomalies
  breach the per-ten-thousand threshold. `Hub.scheduleResyncIfNeeded`
  converts the hint into a forced keyframe and flips the `resync` flag on
  the next broadcast so clients recover deterministically.【F:server/resync_policy.go†L25-L83】【F:server/hub.go†L1144-L1172】
- **State payloads carry lifecycle arrays.** `stateMessage` now only attaches
  `effect_spawned`, `effect_update`, `effect_ended`, and
  `effect_seq_cursors` alongside players, NPCs, obstacles, and ground
  items—the legacy `effects` snapshot has been removed to reduce payload
  size. Join responses follow the same shape so every client receives a
  keyframe plus the lifecycle batch before any diffs apply.【F:server/messages.go†L3-L39】【F:server/hub.go†L1049-L1139】

## Client consumption & replay

- **Two-pass application.** `applyEffectLifecycleBatch` processes spawns
  first, retries updates once after the spawn pass, and then applies end
  events. It maintains a per-effect sequence map, records drops, and
  exposes unknown updates so diagnostics stay actionable.【F:client/effect-lifecycle.js†L272-L415】
- **Join + incremental messages share the same path.** The join handler
  resets lifecycle state, applies the initial batch, and then the WebSocket
  feed calls the same helper for each `state` payload, keeping
  `store.lastEffectLifecycleSummary` up to date for the diagnostics UI.【F:client/network.js†L1126-L1341】
- **Diff recovery stays in lockstep.** Patch replay bookkeeping uses the
  lifecycle batch to request keyframes when the journal signals a gap, and
  retries are scheduled via the keyframe retry loop so resyncs stay bounded
  even under packet loss.【F:client/network.js†L924-L1002】【F:client/network.js†L1231-L1339】

## Visual runtime expectations

- **js-effects definitions convert contract payloads.**
  `render.syncEffectsByType` checks for lifecycle entries, calls each
  definition's `fromEffect` helper to derive spawn options, and keeps
  instances updated with contract metadata. Definitions that implement
  `fromEffect` receive the authoritative payload and can hand off decals or
  apply custom transforms per update.【F:client/render.js†L320-L417】【F:client/js-effects/effects/meleeSwing.js†L1-L137】
- **Lifecycle metadata travels with effects.** When contract batches drive
  `store.effects`, helper utilities mark entries with
  `__contractDerived` so downstream consumers know which records came from
  the journal versus legacy snapshots. Rendering prefers these derived
  entries and removes stale instances automatically.【F:client/render.js†L340-L417】【F:client/effect-lifecycle.js†L168-L220】

## Telemetry & troubleshooting

- **Server exposes lifecycle metrics.** `telemetryCounters` tracks active
  effect counts, trigger throughput, journal drop reasons, and parity
  aggregates per effect type/source. The `/diagnostics` payload surfaces
  total ticks plus per-source hit/miss/damage rates so rollouts can monitor
  contract parity in real time.【F:server/telemetry.go†L501-L565】
- **Investigate unknown updates quickly.** If the client records unknown
  updates after the retry pass, `applyEffectLifecycleBatch` surfaces the
  offending events for logging. Capture the effect ID and sequence when the
  diagnostics panel shows dropped updates—this usually signals a missing
  spawn in the transport or a stale cursor during replay.【F:client/effect-lifecycle.js†L336-L411】

With the contract stream in place, new gameplay or cosmetic work should
extend the intent helpers, register a definition, and rely on the lifecycle
arrays for transport. Avoid reintroducing ad-hoc effect arrays; treat the
journal as the canonical source of truth for anything that animates in the
world.
