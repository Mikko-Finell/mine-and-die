# Stats Architecture — Mine & Die Engine

## Purpose and References
This document defines the server-side stats architecture that underpins combat and power progression.
It aligns with **Stat Taxonomy — Core Attributes (v2.1)** for stat semantics and **Exponential-Decay Boost System — Mechanics Spec (v1)** for long-term progression, and complements the simulation model in **Server Architecture**.
The design ensures the stats pipeline is deterministic, reproducible, and compatible with item-driven growth.

## Goals
- **Deterministic resolution** — Calculations must be repeatable per tick, supporting the RNG-free combat outlined in the taxonomy doc.
- **Authoritative source of truth** — Stats live exclusively on the server and flow to the client through existing snapshot/patch infrastructure.
- **Low churn diffs** — Derived values publish minimal patches to keep `Hub.broadcastState` payloads small.
- **Composable modifiers** — Permanent boosters, transient buffs, conditions, equipment, and environmental effects can layer predictably.
- **Go-first implementation** — Prefer structs and typed accessors over reflection; keep allocations low by reusing small arrays and pooling scratch buffers.

## Architectural Overview
```
Progression Item → Stat Mutation Command → Stat Engine → Derived Stats Cache →
Combat Systems (damage, avoidance, cooldowns) → World Mutators → Snapshot/Patch
```
1. Gameplay events (consumables, loot, scripted effects) enqueue stat mutation commands during the tick.
2. The stat engine updates base attributes, applies modifier layers, recalculates derived stats, and returns diff payloads.
3. Combat systems consume the cached derived stats to resolve damage and timing without recomputing formulas mid-tick.
4. Changes flow through `World.Set*` mutators so snapshots and patches stay consistent with existing actor workflows.

## Data Model
### Package Layout
Introduce a dedicated `server/stats/` package housing stat types, formula helpers, and mutation logic. Keep `player.go` focused on actor transport; embed the stats component into `playerState`/`npcState`.

```
server/
  stats/
    stats.go          // base enums, structs, modifiers
    formulas.go       // derived stat calculations
    registry.go       // booster definitions, tuning values, decay ratio
    serialization.go  // helpers for snapshot payloads and diff packing
```

### Core Types
- `type StatID uint8` — enumerates primary attributes (`Might`, `Resonance`, `Focus`, `Speed`) and any derived IDs we explicitly track.
- `type Layer uint8` — defines modifier layers: `Base`, `Permanent`, `Equipment`, `Temporary`, `Environment`, `Admin`.
- `type ValueSet [StatCount]float64` — fixed-size array for cache-friendly storage of stat vectors.
- `type OverrideValue struct { Active bool; Value float64 }` — gates overrides per stat.
- `type OverrideSet [StatCount]OverrideValue` — parallel array where an inactive entry means “ignore”.
- `type LayerStack struct {
    add ValueSet
    mul ValueSet // neutral element filled with 1.0
    override OverrideSet
    version uint64
}` — tracks per-layer contributions and increments the version whenever a layer changes.
- `type Snapshot struct { Total ValueSet; Derived DerivedSet; LastRecalcTick uint64 }` — computed aggregate cached on actors.
- `type SourceKind uint8` and `type SourceKey struct { Kind SourceKind; ID string }` — identify individual modifier sources (items, auras, scripts) for deterministic add/remove.
- `type StatDelta struct { Add ValueSet; Mul ValueSet; Override OverrideSet }` — additive/multiplicative/override contributions supplied by each source. Constructors fill `Mul` with `1.0` so that omitted stats remain neutral.

### Actor Integration
Embed a `stats.Component` inside `playerState` and `npcState`.
```
type playerState struct {
    actorState
    stats stats.Component
    ...
}
```
The component owns:
- Current base attributes (defaults seeded from species archetype or `playerMaxHealth`).
- Layer stacks for permanent boosters, equipment, transient effects, environment buffs, and admin overrides.
- `sources map[Layer]map[SourceKey]StatDelta` storing per-source additive/multiplicative/override payloads for deterministic equip/unequip. Removal simply deletes the key, causing a layer version bump and recompute.
- Derived stat cache (HP, Mana, Accuracy, etc.).
- Dirty flags to trigger recalculation before combat systems read the values in the same tick.
- `expiresBySource map[SourceKey]uint64` for temporary sources keyed by `ExpiresAtTick`.

### Derived Stats
Use deterministic functions (exponential decay, additive scaling) defined in `formulas.go` to compute:
- `MaxHealth`, `MaxMana`
- `DamageScalarPhysical`, `DamageScalarMagical`
- `Accuracy`, `Evasion`
- `CastSpeed`, `CooldownRate`, `StaggerResist`
Expose getters returning cached values to avoid mid-tick recomputation, while still allowing systems to request recalculation explicitly (e.g., after mass updates from world reset).

## Mutation Flow
1. **Command enqueue** — Consumable usage or script triggers issue a `CommandStatChange` with the source, target actor, and mutation payload.
2. **Component update** — `stats.Component.Apply(change)` mutates the relevant layer (`Permanent` for boosters, `Equipment` for gear, `Temporary` for buffs/conditions, `Environment` for zone modifiers, `Admin` for GM tooling) and marks derived caches dirty. Temporary deltas store `ExpiresAtTick` and are culled deterministically inside the tick loop.
3. **Recalculation** — On the next access or at the end of the mutation batch, `Component.Resolve(tickIndex)` recomputes derived stats using tuned formulas and stores them in the cache, incrementing the component version. During resolution it folds layers in the explicit order **Base → Permanent → Equipment → Temporary → Environment → Admin**, applying all additive contributions first, then multiplying the accumulated total by per-layer `mul` values, and finally applying any overrides (highest-precedence layer wins). Overrides skip unset entries so missing data never zeros out stats. `Component.Resolve` also clears expired temporary sources before folding, ensuring deterministic decay using tick indices only.
4. **World mutation** — Health/mana cap changes call `World.SetHealth` / `World.SetNPCHealth` to clamp current values and emit patches, reusing existing mutators for diff consistency.
5. **Broadcast** — The hub includes stat payloads alongside `Actor` snapshots. Only changed stats produce patches thanks to the component version check.

### Commands and Events
Define explicit command structs in `server/messages.go` and handlers in `simulation.go`:
- `CommandConsumeBooster` — removes the item via `Inventory.RemoveQuantity`, invokes `stats.ApplyPermanent` with a stable `SourceKey`, publishes combat log events.
- `CommandEquipItem` / `CommandUnequipItem` — update equipment layer by inserting/removing the specific `SourceKey` so identical items stack predictably.
- `CommandConditionApplied` / `CommandConditionExpired` — integrate with `actorState.conditions` for on-tick buffs/debuffs, storing their `ExpiresAtTick` and removing the matching source during cleanup.
- `CommandResetStats` — used by death handling to wipe layers except archetype defaults.
These commands follow the existing queueing pattern handled in `Hub.advance`, preserving deterministic processing order.

## Formula Strategy
- **Primary attribute totals** — Fold additive layers in the sequence `Base + Permanent + Equipment + Temporary + Environment + Admin` before multiplicative and override passes.
- **Derived metrics** — Use piecewise or exponential curves matching `stat-taxonomy.md` and `stat-progression.md`. Example: `MaxHealth = baseHealth + MightScalar * MightTotal` with diminishing returns through the booster decay ratio.
- **Combined interactions** — For mechanics blending stats (e.g., cooldown reduction from Speed and Focus), precompute cross-term coefficients and cache them.
- **Clamping** — Apply deterministic clamps (e.g., minimum cast times) within the formulas package to keep simulation logic simple.
During finalization, re-clamp `Actor.Health` and `Actor.Mana` to the newly computed caps and emit resource patches if the values changed. Clamping occurs after overrides so absolute caps (e.g., freeze effects) apply immediately.
Store formula parameters in `registry.go`, enabling data-driven tuning without touching simulation code.

## Serialization
### Snapshot Payloads
Extend `messages.go` with a `StatBlock` struct containing:
- Primary totals (4 floats)
- Key derived stats required by the client (HP cap, Mana cap, attack/cast speed, dodge chance)
- Component version for delta compression
Players and NPCs embed `StatBlock` within their snapshot payloads so the client can render tooltips and diagnostics without recomputing formulas.

### Patch Emission
`World.appendPatch` receives stat diffs when the component version increments. Pack only changed entries to limit payload size. Because `Hub.broadcastState` already merges patches per tick, stat diffs piggyback on existing infrastructure without extra socket messages. Resource clamping emits paired health/mana patches in the same batch for consistency.

## Death and Reset Handling
- **On death** — Clear `Permanent`, `Equipment`, and `Temporary` layers while preserving archetype base stats. Set derived caches dirty, recompute, and clamp health/mana to their new caps.
- **On world reset** — Invoke `Component.ResetDefaults(seedConfig)` for every actor while holding the hub mutex, mirroring existing inventory and position resets.
- **Respawn** — When players rejoin, seed the component from persistence (if per-character permanence is later added) or defaults.

## Persistence Hooks
Although the current prototype wipes stats on death, persistence-ready hooks keep the design future-proof:
- `Component.Serialize()` returns a compact representation (layer contributions + version) for storage.
- `Component.Deserialize(payload)` rebuilds the component and recomputes derived caches.
- Storage integration can live in a follow-up `profile` package without changing combat systems.

## Testing Strategy
1. **Unit tests** — Validate formulas per stat and derived metric, including edge cases for high stack counts and zero values.
2. **Integration tests** — Extend `main_test.go` to simulate booster consumption, equipment toggles, and condition application, asserting health caps, damage output, and cooldown timings.
3. **Regression tests** — Add deterministic seeds to confirm identical stat progressions across runs.
4. **Performance tests** — Benchmark component recalculation to ensure negligible impact on the 15 Hz tick loop.

## Implementation Steps
1. Create `server/stats/` package with enums, value sets, component struct, and basic tests.
2. Wire components into `playerState` and `npcState`, migrating hard-coded constants (`playerMaxHealth`, etc.) into formulas referencing stats totals.
3. Extend command handlers in `simulation.go` to route stat-changing events through the component.
4. Update `World` mutators and snapshot serializers to include stat blocks and emit diffs when caps change.
5. Document formulas and tuning parameters alongside the progression docs; expose config knobs (decay ratio, scalars) via a central registry for balancing.
6. Iterate with QA to confirm client HUD updates align with server-authoritative stats before expanding combat features.
