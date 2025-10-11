# Effects & Conditions

This document explains how Mine & Die models combat effects, server-driven
conditions, and the client runtime that renders them. Use it as the reference
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
environmental hazards, and then updates both conditions and effects before the
snapshot is emitted.
Helpers such as `pruneEffects` and `maybeExplodeOnExpiry` remove expired
instances, while `applyEffectHitNPC` invokes defeat handlers the moment an NPC
runs out of health.

## Conditions

Conditions wrap persistent status effects (burning, poison, etc.) around the
core effect system. Definitions live in `newConditionDefinitions`, which sets up
per-condition durations, tick cadences, and lifecycle callbacks.
`applyCondition` instantiates or refreshes `conditionInstance` records for the
target actor, schedules ticks, and invokes `OnApply`/`OnTick` handlers.
The default burning condition attaches a looping fire visual, deals periodic
damage based on lava DPS, and expires cleanly once its timer completes.

`advanceConditions` runs every tick to trigger scheduled ticks, extend attached
visuals, and clean up expired instances.
When a tick inflicts damage, the helper spawns a lightweight
`effectState` with `effectTypeBurningTick` so health changes reuse the shared
behaviour pipeline.

Environmental systems call `applyEnvironmentalConditions` to apply burning while
actors remain inside lava obstacles, so hazards automatically refresh the
condition timer without custom code.

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

When adding a new ability or condition:

1. **Author the behaviour** – Add or update effect behaviours / projectile
   templates in `server/effects.go` and register condition definitions in
   `server/conditions.go` as needed.
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

## Effect Producer Map

The Phase 0 guardrail work introduced an automated inventory of every server
function that spawns or mutates effects. Run the generator whenever you change
effect-producing code so the shared map stays current:

```sh
npm run effects:map
```

The script (`tools/effects/build_producer_map`) parses the Go sources in
`server/effects.go`, `server/conditions.go`, `server/world_mutators.go`, and
`server/simulation.go`, then writes `effects_producer_map.json` at the repo root.
Each entry lists the source file, function name, category tags (producer vs.
mutation), inferred delivery kinds (melee, projectile, trigger, condition, etc.),
and the invariants that function touches (cooldown guards, logging calls, journal
patches, helper invocations). This index is the authoritative reference for
effects migration planning—update it in the same commit as any gameplay change
that affects effects so downstream tooling and documentation remain trustworthy.
