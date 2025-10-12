# Unified Effect System — Contract-Driven Architecture (CDE)

This document defines the unified framework for all combat and visual effects in the Mine & Die engine.
It merges legacy ad-hoc mechanisms (melee swings, projectiles, burning ticks, blood decals) under a single **contract-driven** system where the server is authoritative and clients are purely visual.

For a phase-by-phase execution plan see the [Unified Effect System — Synthesis Roadmap](./unified-effects-roadmap.md).
Progress and open action items live in the [Unified Effects Migration Tracker](../notes/unified-effects-tracker.md).

---

## 1. Goals

1. **Single source of truth** — The server owns all effect lifecycles, motion, and collisions. Clients never fabricate or predict entities.
2. **Explicit contract** — A strict `EffectContract` governs spawn/update/end ordering, idempotency, and resync behavior. Anything not in the contract is an implementation detail.
3. **Minimal surface area** — Three core types (`EffectIntent`, `EffectInstance`, `EffectDefinition`) and one orchestrator (`EffectManager`). No plugin buses or dependency injection.
4. **Deterministic by construction** — All motion and AoE math use fixed-timestep integer coordinates.
5. **Unified for all archetypes** — Arrows, beams, curses, heals, auras, decals—all run through the same event pipeline.

---

## 2. Core Types

```text
EffectIntent
  - TypeID (lookup into definition registry)
  - DeliveryKind (Area | Target | Visual)
  - SourceActorID / optional TargetActorID
  - Geometry payload (offset, facing, arc, projectile arc, etc.)
  - Duration / tick cadence hints

EffectInstance
  - EffectID (stable, generated on spawn)
  - Definition pointer
  - DeliveryState (AoE transform, attached actor, projectile state)
  - BehaviourState (cooldowns, ticks left, accumulated damage)
  - FollowActorID (for Target delivery)
  - Snapshot params for client replication
```

`EffectDefinition` objects describe how an intent behaves:

```text
EffectDefinition
  - DeliveryKind (Area | Target | Visual)
  - Shape: circle / rect / arc / segment / capsule
  - Motion: none | instant | linear | parabolic | follow
  - ImpactPolicy: first-hit | all-in-path | pierce N
  - Lifetime: duration or range
  - Params: small numeric/config map (damage, tint, speed, etc.)
  - Hooks: OnSpawn / OnTick / OnHit / OnExpire
  - ClientReplication: which events to send (spawn, updates, end)
```

---

## 3. Delivery Kinds

| Kind       | Description                                            | Examples                                    |
| ---------- | ------------------------------------------------------ | ------------------------------------------- |
| **Area**   | Occupies space in world; resolved via spatial queries. | sword arc, projectile, explosion, lava pool |
| **Target** | Anchored to one actor; follows it or applies on hit.   | curse, heal, arrow impact                   |
| **Visual** | Fire-and-forget cosmetic; no gameplay effect.          | blood splatter, ambient ember               |

### Geometry and Motion

* **Shape** defines the collision volume (circle/rect/arc/segment/capsule).
* **Motion** defines how it moves or ticks:

  * *instant* — resolved immediately (rays/beams)
  * *linear/parabolic* — projectiles
  * *none/follow* — static AoEs or auras

---

## 4. Server Orchestration

### EffectManager (authoritative owner)

```go
type EffectManager struct {
    active map[string]*EffectInstance
    intents []EffectIntent
}

func (em *EffectManager) EnqueueIntent(intent EffectIntent)
func (em *EffectManager) Tick(world *World)
```

#### Tick flow

1. **Command resolution**
   Simulation converts player/NPC actions into intents via helpers:

   ```go
   NewMeleeIntent(source)
   NewProjectileIntent(source, target)
   NewConditionTickIntent(actor)
   ```

2. **Spawn phase**
   Each tick, `EffectManager` dequeues intents, allocates `EffectInstance`, registers with the world, and emits an `EffectSpawned` event:

   ```go
   world.EmitEffectSpawned(instance)
   ```

   This event carries the full authoritative payload and is journaled with `(tick, seq)` for diff replay.

3. **Update phase**
   For each active instance:

   * **Area**: advance motion, query victims via spatial index, invoke `OnHit/OnTick`.
   * **Target**: follow target or expire if invalid.
   * **Visual**: just decrement lifetime; may emit chained triggers.

   Each motion or param change passes through world write barriers, which generate corresponding `effect_update` patches.

4. **Expiry phase**
   On completion or cancellation:

   ```go
   world.EmitEffectEnd(effectID, reason)
   ```

   Removes instance from registry; diff journal records removal.

### Journal integration & replay contract

The server journal now stores lifecycle envelopes in three arrays—
`effect_spawned`, `effect_update`, and `effect_ended`—that mirror the transport
contract. Each entry is stamped with the authoritative `tick` plus a
per-effect `seq` generated inside the journal. The sequence cursor is tracked
per `EffectID`, reset on spawn, incremented for every update/end, and included
in the drained batch so replay tooling can drop duplicates and guarantee
ordering before applying state. When the world drains the journal it receives a
copy of the events and the `LastSeqByID` map; end events mark the id for
cleanup so the cursor map stays bounded after the batch is consumed. Replay
pipelines should apply a batch by:

1. Sorting envelopes by `(tick, seq)` if cross-tick buffering occurs (steady
   tick drains are already ordered).
2. Rebuilding or verifying their own `lastSeq` map using the provided
   `LastSeqByID` values.
3. Dropping any event whose sequence is ≤ the cached `lastSeq` for that id.
4. Applying spawns → updates → ends before acknowledging the cursor update.

This mirrors the deterministic replay expectations in the roadmap: the journal
is the source of truth for effect ordering, and the cursor map prevents lost
events or duplicates from corrupting client or tooling state.

---

## 5. EffectContract (Transport & Determinism)

| Rule                | Description                                                                                                 |
| ------------------- | ----------------------------------------------------------------------------------------------------------- |
| **Ordering**        | `effect_spawn(tick,seq)` **must** arrive before any `effect_update` for that ID.                            |
| **Idempotency**     | Client tracks `lastSeq`; drops stale or duplicate updates.                                                  |
| **No placeholders** | Unknown-ID updates are logged and ignored; clients never fabricate.                                         |
| **Join/resync**     | Server sends full active-effects snapshot before replaying subsequent events.                               |
| **Determinism**     | Server uses fixed-timestep integer math (e.g., 1/16-tile quantization). Client visuals are decorative only. |

---

## 6. Client Integration

1. Apply `EffectSpawned` events **before** any patches in the same tick.
2. Route to `EffectManager` (JS) for visuals:

   * **Area** — draw particles or hit arcs using instance bbox.
   * **Target** — follow actor by `FollowActorID`.
   * **Visual** — trigger animation; auto-expire on timer or `EffectEnd`.
3. Apply updates, then ends.
4. If an update references an unknown effect after one retry, log and drop.

### Two-Pass Algorithm (per batch)

1. Apply keyframe if present.
2. Partition entries: `spawns`, `updates`, `ends`.
3. Pass A: apply all `spawns`.
4. Pass B: apply `updates`; retry unknown once.
5. Apply `ends` last.

Unknowns after pass 2 = bug. Never stash across batches.

---

## 7. How Arrows and Rays Fit

| Use Case                               | DeliveryKind | Shape             | Motion              | ImpactPolicy          | Notes                                   |
| -------------------------------------- | ------------ | ----------------- | ------------------- | --------------------- | --------------------------------------- |
| **Arrow projectile**                   | Area         | Segment / Capsule | Linear              | First-hit or pierce N | OnHit spawns optional Targeted rider.   |
| **Instant ray (Eldritch-blast-style)** | Area         | Segment / Capsule | Instant             | First-hit             | Zero duration; resolves same tick.      |
| **Beam / channel**                     | Area         | Segment / Capsule | None (follow owner) | All-in-path           | Applies OnTick each frame while active. |

---

## 8. Performance & Reliability

* **Spatial index:** uniform grid with cell ≈ median AoE radius → O(K) lookups per cell.
* **Quantized motion:** `(x,y,vx,vy)` stored as ints (1/16 tile) for deterministic replay.
* **Tick budget:** process ≤ M active effects per tick (overflow queues one tick ahead).
* **Owner loss:** Targeted effects auto-expire with `reason="ownerLost"`.
* **Map/phase change:** emit `EffectEnd`; re-spawn if needed.
* **Lost spawn:** client drops updates → telemetry flag triggers → server may issue resync hint.

---

## 9. Migration Strategy

1. **Audit current producers** — replace `triggerMeleeAttack`, `spawnProjectile`, burning ticks, and `QueueEffectTrigger` with intent helpers.
2. **Introduce event payloads** — extend world broadcast structs with `EffectSpawned`/`EffectEnd` arrays.
3. **Port behaviours** — move per-type logic into `EffectDefinition` hooks; delete old code once parity achieved.
4. **Add table-driven tests** — feed intents through the subsystem and assert journaled spawn/update/end sequences for all delivery kinds.

---

## 10. Summary

At completion, all gameplay and visual effects in Mine & Die will flow through one deterministic, event-driven contract.
The server remains the only source of truth, the diff pipeline becomes perfectly authoritative, and client visuals follow cleanly from the same lifecycle events—no placeholders, no drift, no ambiguity.
