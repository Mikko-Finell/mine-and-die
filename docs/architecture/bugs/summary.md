# Consolidated Bug Register

## Projectile velocity quantization corrupts replicated direction
- **Tags:** `server`, `effects`, `high`
- **Summary:** `EffectManager.syncProjectileInstance` rounds projectile velocity unit vectors to integers before replication, erasing diagonal components and desynchronising client playback and downstream contract logic that depends on accurate direction. [docs/architecture/bugs/0575d14b05.md](0575d14b05.md)

## Equip rollback loses gear when reinsertion fails
- **Tags:** `server`, `inventory`, `critical`
- **Summary:** `World.EquipFromInventory` removes the previously equipped item and attempts to stash it back into inventory without checking the result; if the reinsertion fails the player permanently loses the item and its stats. [docs/architecture/bugs/0575d14b05.md](0575d14b05.md)

## Join retry timer keeps running after success
- **Tags:** `client`, `networking`, `medium`
- **Summary:** The client schedules reconnect retries with `setTimeout` but never cancels the timer on success, so delayed callbacks re-run `joinGame`, resetting local state and spawning duplicate players. [docs/architecture/bugs/0575d14b05.md](0575d14b05.md)

## Effect trigger dedupe never forgets processed IDs
- **Tags:** `client`, `effects`, `medium`
- **Summary:** `queueEffectTriggers` appends every processed trigger ID to a persistent set with no pruning, so reused identifiers are discarded on sight and long sessions stop rendering some triggers. [docs/architecture/bugs/0575d14b05.md](0575d14b05.md)

## Path-following emits unbounded intent vectors
- **Tags:** `server`, `movement`, `high`
- **Summary:** `World.followPlayerPath` feeds raw waypoint deltas into `SetIntent`, producing patches with values well outside the normalised input range and confusing replay or diagnostic consumers. [docs/architecture/bugs/531f885f0f.md](531f885f0f.md)

## Inventory diffing ignores fungibility keys
- **Tags:** `server`, `inventory`, `high`
- **Summary:** `inventoriesEqual` omits `FungibilityKey` when comparing stacks, suppressing patches for identity-only changes and leaving clients with stale metadata about unique or bound items. [docs/architecture/bugs/531f885f0f.md](531f885f0f.md) [docs/architecture/bugs/b9b0837b69.md](b9b0837b69.md) [docs/architecture/bugs/be7300b337.md](be7300b337.md)

## Client inventory clones strip fungibility metadata
- **Tags:** `client`, `inventory`, `medium`
- **Summary:** `cloneInventorySlots` rebuilds inventory payloads with only type and quantity, discarding `fungibility_key` fields and causing UI tools to merge stacks that the server keeps distinct. [docs/architecture/bugs/531f885f0f.md](531f885f0f.md) [docs/architecture/bugs/b9b0837b69.md](b9b0837b69.md)

## Ground item normalisation rewrites metadata
- **Tags:** `client`, `loot`, `medium`
- **Summary:** `normalizeGroundItems` defaults every stack to gold and drops fungibility keys, so new ground item types or personalised loot render incorrectly and lose identity. [docs/architecture/bugs/531f885f0f.md](531f885f0f.md)

## Effect patches filtered out of hub payloads
- **Tags:** `server`, `effects`, `critical`
- **Summary:** `Hub.marshalState` only whitelists player, NPC, and ground-item IDs, so effect patches are dropped from diff messages and clients never receive incremental effect updates. [docs/architecture/bugs/58a6e5474c.md](58a6e5474c.md) [docs/architecture/bugs/be7300b337.md](be7300b337.md)

## Ground item removals skip diff emission
- **Tags:** `server`, `loot`, `high`
- **Summary:** Ground item deletions bypass patch journalling and the immediate broadcasts omit refreshed stack lists, leaving clients to display phantom loot until a keyframe arrives. [docs/architecture/bugs/58a6e5474c.md](58a6e5474c.md)

## Player removals suppressed between keyframes
- **Tags:** `server`, `networking`, `high`
- **Summary:** Player deletions are applied locally without emitting patches and non-keyframe broadcasts omit the player list, causing ghost actors to linger for clients and telemetry. [docs/architecture/bugs/58a6e5474c.md](58a6e5474c.md)

## Equipment patches unsupported on the client
- **Tags:** `client`, `equipment`, `critical`
- **Summary:** The patch handling table lacks equipment entries, so `player_equipment` and `npc_equipment` updates are logged as unsupported and never reach the HUD. [docs/architecture/bugs/58a6e5474c.md](58a6e5474c.md)

## Projectile rehydration restores full travel distance
- **Tags:** `server`, `effects`, `high`
- **Summary:** When rebuilding projectiles, `spawnContractProjectileFromInstance` ignores stored `remainingRange`, resetting shots to their template maximum and extending their reach after resync. [docs/architecture/bugs/817dd370df.md](817dd370df.md)

## Projectile resurrection resets lifetime ticks
- **Tags:** `server`, `effects`, `high`
- **Summary:** Recreated projectile effects set `expiresAt` from the template lifetime instead of the persisted `TicksRemaining`, letting them persist and damage beyond their intended duration. [docs/architecture/bugs/817dd370df.md](817dd370df.md)

## Failed state marshals drop drained patches
- **Tags:** `server`, `networking`, `critical`
- **Summary:** `marshalState` drains patch and effect buffers before calling `json.Marshal`; if encoding fails, the data is lost and clients desynchronise until the next keyframe. [docs/architecture/bugs/817dd370df.md](817dd370df.md)

## Command queue lacks flow control per client
- **Tags:** `server`, `networking`, `high`
- **Summary:** `enqueueCommand` accepts unlimited requests with only log warnings, allowing a single client to flood the queue and degrade or crash the simulation. [docs/architecture/bugs/817dd370df.md](817dd370df.md)

## NPC gold rewards bypass patch emission
- **Tags:** `server`, `loot`, `medium`
- **Summary:** NPC mining rewards call `Inventory.AddStack` directly instead of `MutateNPCInventory`, so inventories never emit patches and subscribers miss the new gold. [docs/architecture/bugs/b9b0837b69.md](b9b0837b69.md) [docs/architecture/bugs/d477523fcc.md](d477523fcc.md)

## Broadcast logging leaks full state payloads
- **Tags:** `server`, `observability`, `medium`
- **Summary:** A debug path in `broadcastState` logs entire JSON payloads when certain keywords are present, flooding logs and exposing sensitive gameplay state. [docs/architecture/bugs/b9b0837b69.md](b9b0837b69.md)

## Version counters misuse pointer increment syntax
- **Tags:** `server`, `patching`, `critical`
- **Summary:** World mutator helpers attempt to bump version numbers with `*version++`, an invalid pointer expression that would either fail to compile or corrupt version tracking, breaking patch sequencing. [docs/architecture/bugs/be7300b337.md](be7300b337.md)

## Contract tick cadence hint is ignored
- **Tags:** `server`, `effects`, `medium`
- **Summary:** Although `EffectIntent` exposes `TickCadence`, the instantiation path never stores or respects it, preventing designers from throttling expensive effect updates. [docs/architecture/bugs/be7300b337.md](be7300b337.md)

## World config normalisation drops NPC totals
- **Tags:** `server`, `configuration`, `high`
- **Summary:** `worldConfig.normalized` overwrites `NPCCount` with species-specific counts that default to zero, so callers setting only the aggregate count end up with no NPC spawns. [docs/architecture/bugs/d477523fcc.md](d477523fcc.md)

## Effect ticks halt when no emitter is provided
- **Tags:** `server`, `effects`, `medium`
- **Summary:** `EffectManager.RunTick` returns early when passed a nil emitter, preventing offline simulations and tests from advancing or expiring contract-managed effects. [docs/architecture/bugs/d477523fcc.md](d477523fcc.md)

## Client ignores NPC equipment patches
- **Tags:** `client`, `equipment`, `high`
- **Summary:** The client patch handler table lacks the `npc_equipment` key, so NPC gear updates are discarded despite being emitted by the server. [docs/architecture/bugs/d477523fcc.md](d477523fcc.md)
