# B.U.G.S. — Behavioral Unification & General Stabilization

This document tracks the ongoing effort to reduce defects and keep the game reliable. Developers can continue the work by using the roadmap and active bugs tables below — no other documents are needed.

## Roadmap

| Phase | Goal                                                     | Exit Criteria                                                                                   | Status         |
| ----- | -------------------------------------------------------- | ------------------------------------------------------------------------------------------------ | -------------- |
| 1     | Catalogue systemic failures across gameplay subsystems   | Architecture bug register compiled with severity tags and reproduction notes for every category | 🟢 Complete    |
| 2     | Restore replication fidelity for combat and movement     | Projectile direction, effect patch emission, and path intent tests green across client/server    | 🟡 In progress |
| 3     | Harden inventory and equipment parity end to end         | Inventory diffing/cloning fixed, NPC rewards patch correctly, equipment updates reach the HUD    | ⚪ Planned      |
| 4     | Build resilience in networking flow control and logging  | Command queue throttled, marshal retries buffered, and broadcast logging sanitized               | ⚪ Planned      |

## Active Bugs

| Bug                                              | Impact     | Status    | Notes |
| ------------------------------------------------ | ---------- | --------- | ----- |
| Projectile velocity quantization corrupts replicated direction | High       | 🟢 Done  | `syncProjectileInstance` rounds velocity vectors, erasing diagonals and desyncing clients (TestSyncProjectileInstanceQuantizesDirection). |
| Equip rollback loses gear when reinsertion fails | Critical   | 🔴 Todo  | `EquipFromInventory` drops previous gear if reinsertion fails, permanently deleting items. |
| Join retry timer keeps running after success     | Medium     | 🟢 Done  | Retry timeout never cleared, so delayed callbacks re-run `joinGame` and duplicate players; fixed by tracking and clearing the handle. |
| Effect trigger dedupe never forgets processed IDs | Medium     | 🔴 Todo  | Persistent trigger ID set grows forever and blocks recycled triggers from rendering. |
| Path-following emits unbounded intent vectors    | High       | 🔴 Todo  | Raw waypoint deltas feed into `SetIntent`, exceeding normalized ranges and breaking consumers. |
| Inventory diffing ignores fungibility keys       | High       | 🟢 Done   | `inventoriesEqual` omits `FungibilityKey`, leaving clients with stale identity metadata; repro: `TestMutateInventoryEmitsPatchWhenFungibilityChanges`. |
| Client inventory clones strip fungibility metadata | Medium     | 🟢 Done   | `cloneInventorySlots` now preserves `fungibility_key`, keeping unique stacks distinct. |
| Ground item normalisation rewrites metadata      | Medium     | 🔴 Todo  | `normalizeGroundItems` defaults to gold and drops keys, misrendering new or personal loot. |
| Effect patches filtered out of hub payloads      | Critical   | 🔴 Todo  | `Hub.marshalState` whitelist excludes effects, so incremental effect updates never broadcast. |
| Ground item removals skip diff emission          | High       | 🔴 Todo  | Deletions bypass journalling, so broadcasts omit refreshed stacks until a keyframe. |
| Player removals suppressed between keyframes     | High       | 🔴 Todo  | Player deletions fail to emit patches, leaving ghost actors alive for clients and telemetry. |
| Equipment patches unsupported on the client      | Critical   | 🔴 Todo  | Patch handler table lacks equipment entries; updates are logged and dropped before UI sync. |
| Projectile rehydration restores full travel distance | High       | 🔴 Todo  | `spawnContractProjectileFromInstance` ignores saved `remainingRange`, extending projectile reach. |
| Projectile resurrection resets lifetime ticks    | High       | 🔴 Todo  | Recreated projectiles use template lifetime instead of persisted ticks, causing overlong effects. |
| Failed state marshals drop drained patches       | Critical   | 🔴 Todo  | `marshalState` drains buffers before encode; on failure data is lost until next keyframe. |
| Command queue lacks flow control per client      | High       | 🔴 Todo  | `enqueueCommand` accepts unlimited commands, allowing a single client to flood the queue. |
| NPC gold rewards bypass patch emission           | Medium     | 🔴 Todo  | NPC mining rewards mutate inventories directly, skipping patch emission for subscribers. |
| Broadcast logging leaks full state payloads      | Medium     | 🔴 Todo  | Debug path dumps complete JSON payloads, flooding logs and exposing sensitive state. |
| Version counters misuse pointer increment syntax | Critical   | 🔴 Todo  | Mutator helpers use `*version++`, risking corruption of patch sequencing. |
| Contract tick cadence hint is ignored            | Medium     | 🔴 Todo  | `EffectIntent` exposes `TickCadence` but instantiation never persists or respects it. |
| World config normalisation drops NPC totals      | High       | 🟢 Done  | `worldConfig.normalized` overwrites aggregate `NPCCount`, leaving worlds without spawns. |
| Effect ticks halt when no emitter is provided    | Medium     | 🔴 Todo  | `EffectManager.RunTick` returns early on nil emitters, halting offline simulations. |
| Client ignores NPC equipment patches             | High       | 🔴 Todo  | Patch handler table lacks `npc_equipment`, so NPC gear updates are discarded. |

(Add new rows as bugs are logged. When you start one, set 🟡 Doing; when merged and verified, set 🟢 Done. If obsolete or duplicate, strike through with a short note.)

## Quality Goals

* Reproducible: every bug entry includes a minimal repro (command, test name, or scenario).
* Deterministic: simulation/replication paths avoid nondeterministic branches.
* No zombies: entities/items removed on server are removed on clients without keyframe reliance.
* Tests with fixes: every fix lands with a failing test turned green.
* Minimal surface area: prefer single code paths per behavior to reduce bug vectors.
