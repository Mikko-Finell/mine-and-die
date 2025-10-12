# Effects & Status Effects

This document explains how Mine & Die models combat effects, server-driven
status effects, and the client runtime that renders them. Use it as the reference
when tuning existing abilities or introducing new ones.

## Authoritative Effect Model

The server owns the authoritative representation of every active effect. Each
effect broadcast to clients is based on the `Effect` struct, which records its
ID, type, owner, lifetime, bounding box, and any numeric parameters.
Transient server-side bookkeeping lives in `effectState`, which adds the
expiration time, optional projectile metadata, and a `FollowActorID` so visuals
can stay attached to moving actors.

Effect behaviours are registered in `newEffectBehaviors`. Behaviours read common
parameters such as `healthDelta` and apply them to target actors whenever a hit
is detected.
Damage-focused behaviours log when a target is defeated so higher level systems
can react.

The combat helpers `triggerMeleeAttack` and `spawnProjectile` create new
`effectState` instances. Melee swings spawn a short-lived rectangle in front of
the attacker, apply damage through `applyEffectHitPlayer` /
`applyEffectHitNPC`, and mine gold when the swing overlaps an ore obstacle.
Projectiles look up a template in `projectileTemplates`, enforce cooldowns, and
spawn travelling effects that advance every tick until they hit something or run
out of range.

### Fire-and-Forget Triggers

Not every visual warrants a full effect entry. Helpers call
`QueueEffectTrigger` to enqueue one-shot instructions (for example, blood
splatter decals) that the next snapshot will deliver exactly once.
Triggers and active effects are sent alongside the standard snapshot so clients
can render lingering hitboxes and single-use decals in the same frame.

### Lifecycle Management

`World.Step` advances player and NPC movement, stages abilities, applies
environmental hazards, and then updates both status effects and effects before the
snapshot is emitted.
Helpers such as `pruneEffects` and `maybeExplodeOnExpiry` remove expired
instances, while `applyEffectHitNPC` invokes defeat handlers the moment an NPC
runs out of health.

## Status Effects

Status effects wrap persistent modifiers (burning, poison, etc.) around the
core effect system. Definitions live in `newStatusEffectDefinitions`, which sets up
per-effect durations, tick cadences, and lifecycle callbacks.
`applyStatusEffect` instantiates or refreshes `statusEffectInstance` records for the
target actor, schedules ticks, and invokes `OnApply`/`OnTick` handlers.
The default burning status effect attaches a looping fire visual, deals periodic
damage based on lava DPS, and expires cleanly once its timer completes.

`advanceStatusEffects` runs every tick to trigger scheduled ticks, extend attached
visuals, and clean up expired instances.
When a tick inflicts damage, the helper spawns a lightweight
`effectState` with `effectTypeBurningTick` so health changes reuse the shared
behaviour pipeline.

Environmental systems call `applyEnvironmentalStatusEffects` to apply burning while
actors remain inside lava obstacles, so hazards automatically refresh the
status effect timer without custom code.

## Snapshot & Transport

`World.Snapshot` copies active effects into broadcast structs, while
`flushEffectTriggersLocked` drains the trigger queue.
The hub includes both arrays in every `state` payload sent over the WebSocket,
and resets the trigger queue after broadcasting.

### `marshalState` payload layout

`Hub.marshalState` orchestrates every outbound payload. It hydrates the
snapshot (players, NPCs, effects, ground items, obstacles) when
`includeSnapshot` is `true`, and otherwise sets those fields to `nil` so the
incremental payload omits them. Patches are always included, but the function
either drains or snapshots them depending on the caller. Effect triggers are
resolved regardless of whether a keyframe is being emitted, so one-shot visuals
ride alongside incremental frames.【F:server/hub.go†L842-L977】

The JSON layout mirrors the struct order in `stateMessage`, so consumers see the
arrays in the following sequence during a full keyframe broadcast:

1. `players`
2. `npcs`
3. `obstacles`
4. `effects`
5. `effectTriggers`
6. `groundItems`
7. `patches`

Both the `/join` response and keyframe refreshes include the full set. Regular
tick updates usually omit the first three arrays because their slices are `nil`
when `includeSnapshot` is `false`, leaving clients to reuse the last keyframe.
`effects` behaves the same way: when the hub decides to skip a keyframe, the
field is omitted and only `effectTriggers` deliver transient visuals for that
tick.【F:server/hub.go†L933-L977】【F:server/messages.go†L18-L46】

`marshalState` also stamps every payload with a monotonically increasing
`sequence` and tracks the most recent `keyframeSeq`. Initial state pushes set the
`resync` flag because they do not drain patches; ongoing broadcasts clear the
flag unless the hub explicitly schedules a resync. The `keyframeInterval`
reported in the payload reflects the current cadence (default 1, meaning every
tick is a keyframe) and is updated whenever the interval changes.【F:server/hub.go†L894-L1015】

#### Example payloads

Full keyframe (e.g., join handshake or forced refresh):

```json
{
  "ver": 1,
  "type": "state",
  "players": [
    {
      "id": "player-1",
      "x": 8.5,
      "y": 4.0,
      "facing": "down",
      "health": 100,
      "maxHealth": 100,
      "inventory": {"slots": []},
      "equipment": {"slots": []}
    }
  ],
  "npcs": [
    {
      "id": "npc-keep-1",
      "x": 6.0,
      "y": 10.5,
      "facing": "left",
      "health": 50,
      "maxHealth": 50,
      "inventory": {"slots": []},
      "equipment": {"slots": []},
      "type": "goblin",
      "aiControlled": true,
      "experienceReward": 12
    }
  ],
  "obstacles": [
    {"id": "ore-17", "type": "gold-ore", "x": 9.0, "y": 4.0, "width": 1.0, "height": 1.0}
  ],
  "effects": [
    {"id": "swing-42", "type": "meleeSwing", "owner": "player-1", "start": 1728656400123, "duration": 150, "x": 9.0, "y": 4.0, "width": 1.5, "height": 1.0}
  ],
  "effectTriggers": [
    {"id": "blood-57", "type": "bloodSplatter", "start": 1728656400123, "x": 9.0, "y": 4.0}
  ],
  "groundItems": [
    {"id": "ground-3", "type": "gold", "fungibility_key": "ore", "x": 9.0, "y": 4.0, "qty": 2}
  ],
  "patches": [],
  "t": 318,
  "sequence": 882,
  "keyframeSeq": 882,
  "serverTime": 1728656400123,
  "config": {
    "obstacles": true,
    "obstaclesCount": 18,
    "goldMines": true,
    "goldMineCount": 4,
    "npcs": true,
    "goblinCount": 6,
    "ratCount": 4,
    "npcCount": 10,
    "lava": true,
    "lavaCount": 3,
    "seed": "prototype",
    "width": 64,
    "height": 64
  },
  "resync": true,
  "keyframeInterval": 1
}
```

Incremental update (no keyframe):

```json
{
  "ver": 1,
  "type": "state",
  "effectTriggers": [
    {"id": "sparks-12", "type": "oreSparks", "start": 1728656400190, "x": 9.0, "y": 4.0}
  ],
  "patches": [],
  "t": 319,
  "sequence": 883,
  "keyframeSeq": 882,
  "serverTime": 1728656400190,
  "config": {
    "obstacles": true,
    "obstaclesCount": 18,
    "goldMines": true,
    "goldMineCount": 4,
    "npcs": true,
    "goblinCount": 6,
    "ratCount": 4,
    "npcCount": 10,
    "lava": true,
    "lavaCount": 3,
    "seed": "prototype",
    "width": 64,
    "height": 64
  },
  "keyframeInterval": 1
}
```

Clients must retain the last keyframe to resolve entity state, then layer the
latest triggers and patches each tick until a new keyframe arrives.

## Client Runtime

The client stores the latest `effects` array and drains `effectTriggers` when it
receives a new snapshot or the initial `/join` response.
`render.js` ensures there is a shared `EffectManager`, registers trigger
handlers, and calls `syncEffectsByType` for each definition so authoritative
payloads become tracked instances inside the js-effects runtime.
During each render pass the effect manager consumes pending triggers, updates
all tracked instances, and draws layered visuals on the canvas.

## Extending the System

When adding a new ability or status effect:

1. **Author the behaviour** – Add or update effect behaviours / projectile
   templates in `server/effects.go` and register status effect definitions in
   `server/status_effects.go` as needed.
2. **Emit visuals** – Decide whether the feature needs a tracked effect (with a
   bounding box and lifetime) or a fire-and-forget trigger, and use
   `QueueEffectTrigger` for one-shot decals.
3. **Expose state** – Ensure any new fields required by clients are included in
   the snapshot or trigger payloads.
4. **Render on the client** – Create or update js-effects definitions under
   `tools/js-effects/packages/effects-lib`, run `npm run build`, and hook the new
   type into `render.js` via `syncEffectsByType` or a trigger handler.
5. **Document behaviour** – Update this file or other docs so future contributors
   understand the new mechanics and expected visuals.

## Legacy Effect Telemetry Plan

### Objectives

Phase 0 called for observability on the current, pre-unification effect loop so
we have baselines before dual-write rollouts begin. The instrumentation now in
place mirrors three lifecycle phases already exercised by the world:

* **Spawn** – `triggerMeleeAttack`, `spawnProjectile`, status effect handlers, and
  `spawnAreaEffectAt` append new `effectState` entries after pruning expired
  instances.【F:server/effects.go†L415-L457】【F:server/effects.go†L545-L613】【F:server/effects.go†L968-L1007】【F:server/status_effects.go†L247-L276】
* **Update** – `advanceProjectile`, `updateFollowEffect`, and the write barriers
  `SetEffectPosition` / `SetEffectParam` mutate tracked effects while the world is
  locked.【F:server/effects.go†L723-L872】【F:server/world_mutators.go†L322-L358】
* **End** – Effects exit through `stopProjectile`, `expireAttachedEffect`, and the
  `pruneEffects` sweep that purges any instance whose `expiresAt` has elapsed and
  clears residual patches.【F:server/effects.go†L912-L1057】【F:server/status_effects.go†L298-L317】

### Metrics & Aggregation Surface

| Metric | Type | Cardinality | Update cadence | Aggregation | Notes |
| --- | --- | --- | --- | --- | --- |
| `effects.spawned_total` | Counter | `effect_type` + `producer` (≤6 × 4) | Increment on each spawn helper before slices append. | Extend `telemetryCounters` with atomic counters; emit optional debug log when `DEBUG_TELEMETRY=1`. | Ensures melee, projectile, status effect, and explosion spawns are counted independently.【F:server/effects.go†L415-L457】【F:server/effects.go†L545-L613】【F:server/effects.go†L968-L1007】【F:server/status_effects.go†L247-L276】 |
| `effects.updated_total` | Counter | `effect_type` + `mutation` (`position`, `param`) | Increment from `SetEffectPosition` / `SetEffectParam`. | Same counters struct; no log spam because updates are high-volume. | Measures how often projectiles move or params change per tick.【F:server/world_mutators.go†L322-L358】 |
| `effects.active_gauge` | Gauge (last sample) | unlabelled | Record once per tick after `world.Snapshot` to capture active effect count. | Store in `telemetryCounters` via `RecordEffectsActive(count)` style helper. | Gives baseline active counts alongside existing tick duration sampling.【F:server/hub.go†L711-L734】 |
| `effects.ended_total` | Counter | `effect_type` + `reason` (`duration`, `impact`, `ownerLost`, `cancelled`) | Increment in `stopProjectile`, `expireAttachedEffect`, and `pruneEffects`. | Counters struct plus structured log for unexpected reasons when debug mode enabled. | Differentiates expiry-on-impact vs. natural timeout for parity tracking.【F:server/effects.go†L912-L947】【F:server/effects.go†L1042-L1057】【F:server/status_effects.go†L298-L317】 |
| `effect_triggers.enqueued_total` | Counter | `trigger_type` | Increment inside `QueueEffectTrigger`. | Counters struct; existing trigger queue stays unchanged. | Tracks fire-and-forget usage independent of tracked effects.【F:server/effects.go†L318-L371】 |

### Availability & Baseline Capture

The metrics above now surface through the diagnostics endpoint and optional
debug logging. `/diagnostics` includes nested
`telemetry.effects` and `telemetry.effectTriggers` payloads so QA can capture
snapshots during sessions or automated smoke runs.【F:server/main.go†L55-L79】【F:server/telemetry.go†L118-L142】
When `DEBUG_TELEMETRY=1` the tick log prints the current active gauge plus the
aggregate counters, making it easy to sanity-check increments while exercising
combat loops locally.【F:server/telemetry.go†L165-L189】 To establish a baseline,
start the server with debug telemetry enabled, run a short scenario that covers
melee swings, projectile casts, and burning ticks, then archive both the
terminal output and `/diagnostics` response for comparison before larger
gameplay changes.

All metrics live in the existing hub-scoped `telemetryCounters` to piggyback on
atomic storage, `/diagnostics` snapshots, and the debug print that is already
gated behind `DEBUG_TELEMETRY`. Structured logs remain reserved for anomalies
(`ownerLost`, unexpected cancel reasons) to avoid double-counting once the
unified manager introduces richer journaling.【F:server/telemetry.go†L96-L142】

### Rollout & Validation

1. **Implementation PR** – Add the counters/gauge to `telemetryCounters`, thread
   helpers through `World`/`Hub`, and update `/diagnostics` output. No gameplay
   behaviour changes required.
2. **Smoke validation** – Run `go test ./server/...` and capture telemetry for a
   short local session (melee, projectile, burning) with `DEBUG_TELEMETRY=1` to
   confirm counter increments track expected actions.
3. **Tracker sign-off** – Once metrics appear in `/diagnostics`, flip the Phase 0
   telemetry deliverable to `Ready to Start` → `Complete` after review, and start
   capturing baseline numbers for migration monitoring.

This plan keeps the legacy loop observable without blocking the contract
rollout—once the unified manager arrives we can mirror the same metric shapes on
the new event stream for parity alerts.

## Effect Producer Map

The Phase 0 guardrail work introduced an automated inventory of every server
function that spawns or mutates effects. Run the generator whenever you change
effect-producing code so the shared map stays current:

```sh
npm run effects:map
```

The script (`tools/effects/build_producer_map`) parses the Go sources in
`server/effects.go`, `server/status_effects.go`, `server/world_mutators.go`, and
`server/simulation.go`, then writes `effects_producer_map.json` at the repo root.
Each entry lists the source file, function name, category tags (producer vs.
mutation), inferred delivery kinds (melee, projectile, trigger, statusEffect, etc.),
and the invariants that function touches (cooldown guards, logging calls, journal
patches, helper invocations). This index is the authoritative reference for
effects migration planning—update it in the same commit as any gameplay change
that affects effects so downstream tooling and documentation remain trustworthy.
