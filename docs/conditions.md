# Condition system

The condition system powers persistent gameplay states (burning, poison, frozen, etc.) on the server. Conditions are defined in `server/conditions.go` and integrate with the existing effect pipeline rather than duplicating damage or visuals.

## Definitions
- `ConditionDefinition` captures duration, tick interval, and optional handlers for apply/tick/expire events. Handlers receive the owning `World`, the target `actorState`, and the tracked `conditionInstance` so they can spawn effects, refresh timers, or perform cleanup.
- `ConditionType` values are registered via `newConditionDefinitions`. Add new entries there when introducing a status effect.
- `conditionInstance` stores per-target state: timestamps, the next scheduled tick, and any attached looping effect.

## Runtime flow
1. Systems call `World.applyCondition` to apply or refresh a condition on an actor. The helper creates an instance, runs the optional `OnApply` hook, and ensures ticks are scheduled.
2. Each simulation step calls `World.advanceConditions(now)`. The method:
   - Executes any due ticks (`OnTick`) so gameplay effects happen on schedule.
   - Extends or expires attached visuals via helpers like `extendAttachedEffect`.
   - Invokes `OnExpire` once the condition timer completes and removes the instance from the actor.
3. `effectState` gained a `FollowActorID`, allowing the `advanceNonProjectiles` hook to keep looping effects aligned with the actor every frame.

## Burning example
- Lava hazards call `applyCondition` with `ConditionBurning` when an actor overlaps the obstacle.
- Fireball impacts also call `applyCondition` so direct hits ignite the target before the lava timer kicks in.
- The `OnApply` hook spawns a looping `fire` effect that follows the actor and refreshes while the condition is active.
- Every `200ms` the `OnTick` handler spawns a `burning-tick` effect that uses `healthDeltaBehavior` to deduct health, so the damage path reuses the existing effect behaviours.
- After three seconds without refresh, `OnExpire` cleans up the attached fire effect and the actor stops taking damage.

Add future conditions by extending the registry, supplying appropriate effect hooks, and invoking `applyCondition` from the relevant gameplay system.
