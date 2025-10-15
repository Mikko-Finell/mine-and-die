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
| Equip rollback loses gear when reinsertion fails | Critical   | 🟢 Done  | `EquipFromInventory` drops previous gear if reinsertion fails, permanently deleting items; repro: `TestEquipFromInventoryRollsBackWhenReinsertionFails`. |
| Join retry timer keeps running after success     | Medium     | 🟢 Done  | Retry timeout never cleared, so delayed callbacks re-run `joinGame` and duplicate players; fixed by tracking and clearing the handle. |
| Effect trigger dedupe never forgets processed IDs | Medium     | 🟢 Done  | Persistent trigger ID set grows forever and blocks recycled triggers from rendering. |
| Path-following emits unbounded intent vectors    | High       | 🟢 Done  | Raw waypoint deltas feed into `SetIntent`, exceeding normalized ranges and breaking consumers. |
| Effect attachments jump back to caster on target death | Medium     | ⚪ Planned | Burning effect snaps to player when rat dies; should stay on dead target; repro: cast fireball at sewer rat. |
| Effect anchors offset to screen corners | Medium     | 🟢 Done | Fireball, fire, and melee-swing effects spawn at screen edges after 52eab83; repro: trigger any of those effects. |
| Inventory diffing ignores fungibility keys       | High       | 🟢 Done   | `inventoriesEqual` omits `FungibilityKey`, leaving clients with stale identity metadata; repro: `TestMutateInventoryEmitsPatchWhenFungibilityChanges`. |
| Client inventory clones strip fungibility metadata | Medium     | 🟢 Done   | `cloneInventorySlots` now preserves `fungibility_key`, keeping unique stacks distinct. |
| Ground item normalisation rewrites metadata      | Medium     | 🟢 Done  | `normalizeGroundItems` preserves type/fungibility metadata so new loot renders correctly. |
| Effect patches filtered out of hub payloads      | Critical   | 🟢 Done  | `Hub.marshalState` now whitelists active effect IDs so effect patches survive filtering (TestMarshalStateRetainsEffectPatches). |
| Ground item removals skip diff emission          | High       | 🟢 Done  | Deletions bypass journalling, so broadcasts omit refreshed stacks until a keyframe. |
| Equipment patches unsupported on the client      | Critical   | 🟢 Done  | Patch handlers now hydrate `player_equipment`/`npc_equipment` payloads so loadouts reach the UI. |
| Player removals suppressed between keyframes     | High       | 🟢 Done  | `World.RemovePlayer` now emits `player_removed` diffs (TestRemovePlayerEmitsRemovalPatch). |
| Projectile rehydration restores full travel distance | High       | 🟢 Done  | `spawnContractProjectileFromInstance` ignores saved `remainingRange`, extending projectile reach. |
| Contract projectile definitions skip damage payloads | Critical   | 🟢 Done | `TestContractProjectileDefinitionsApplyDamage` now passes after inheriting fireball damage params from the projectile template. |
| Projectile resurrection resets lifetime ticks    | High       | 🟢 Done  | Recreated projectiles use template lifetime instead of persisted ticks, causing overlong effects. |
| Failed state marshals drop drained patches       | Critical   | 🟢 Done  | `marshalState` restores drained patch/effect buffers when encoding fails, preserving data until retry. |
| Command queue lacks flow control per client      | High       | 🔴 Todo  | `enqueueCommand` accepts unlimited commands, allowing a single client to flood the queue. |
| NPC gold rewards bypass patch emission           | Medium     | 🟢 Done  | Routed NPC mining rewards through inventory mutators so patches broadcast (`TestNPCMiningEmitsInventoryPatch`). |
| Blood splatter applies to attacker instead of victim | Low        | 🟢 Done   | Contract translator now uses quantized center coords so decals stick to the victim; repro: rat bite vs. player. |
| Blood splatter decals ignore configured sizing   | Low        | ⚪ Planned | Decal handoff yields oversized stains; should match animation params; repro: watch blood decal settle after hit. |
| Broadcast logging leaks full state payloads      | Medium     | 🟢 Done  | Debug path now summarizes markers/size instead of dumping full JSON payloads. |
| Version counters misuse pointer increment syntax | Critical   | 🟢 Done  | Mutator helpers now call `incrementVersion` so pointer arithmetic no longer corrupts patch sequencing. |
| Contract tick cadence hint is ignored            | Medium     | 🟢 Done  | Intent cadence now persists to instances and throttles updates (TestEffectManagerRespectsTickCadence). |
| World config normalisation drops NPC totals      | High       | 🟢 Done  | `worldConfig.normalized` overwrites aggregate `NPCCount`, leaving worlds without spawns. |
| Effect ticks halt when no emitter is provided    | Medium     | 🟢 Done  | `EffectManager.RunTick` now runs hooks/end-of-life even without an emitter; added regression test. |
| Client ignores NPC equipment patches             | High       | 🟢 Done  | Client patch handlers now accept `npc_equipment` so NPC loadouts update on the HUD. |
| Client rejects effect_pos/effect_params patches  | Medium     | 🟢 Done | Added `effect_pos`/`effect_params` handlers to `client/patches.js` so fireball contract projectiles stop logging `unsupported patch kind`; repro: cast fireball and watch browser console. |

(Add new rows as bugs are logged. When you start one, set 🟡 Doing; when merged and verified, set 🟢 Done. If obsolete or duplicate, strike through with a short note.)

## Quality Goals

* Reproducible: every bug entry includes a minimal repro (command, test name, or scenario).
* Deterministic: simulation/replication paths avoid nondeterministic branches.
* No zombies: entities/items removed on server are removed on clients without keyframe reliance.
* Tests with fixes: every fix lands with a failing test turned green.
* Minimal surface area: prefer single code paths per behavior to reduce bug vectors.
