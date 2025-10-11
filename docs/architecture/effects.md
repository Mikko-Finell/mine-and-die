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

## Client Runtime

The client stores the latest `effects` array and drains `effectTriggers` when it
receives a new snapshot or the initial `/join` response.
`render.js` ensures there is a shared `EffectManager`, registers trigger
handlers, and calls `syncEffectsByType` for each definition so authoritative
payloads become tracked instances inside the js-effects runtime.
During each render pass the effect manager consumes pending triggers, updates
all tracked instances, and draws layered visuals on the canvas.

## Effect Producer Inventory

Run `npm run build:effects-producer-map` whenever you need an up-to-date inventory of every server helper that spawns an effect, enqueues a trigger, or applies condition-driven damage. The command executes `tools/effects/build_producer_map`, which walks the `server/` package and writes `effects_producer_map.json` at the repository root.

The generated JSON lists each method, the kinds of payloads it produces (`active-effect`, `trigger`, or `direct-application`), the effect or trigger types it instantiates, and whether it touches cooldown, logging, or journal guardrails. Pass `--csv <path>` to the tool when you also need a spreadsheet-friendly export.

Use this inventory before changing producer behaviour so you can confirm downstream systems (logging, cooldown enforcement, journal guards) stay in sync.

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
