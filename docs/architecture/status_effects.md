# Status effect system

The status effect system powers persistent gameplay states (burning, poison, frozen, etc.) on the server. Status effects are defined in `server/status_effects.go` and integrate with the existing effect pipeline rather than duplicating damage or visuals.

## Definitions
- `StatusEffectDefinition` captures duration, tick interval, and optional handlers for apply/tick/expire events. Handlers receive the owning `World`, the target `actorState`, and the tracked `statusEffectInstance` so they can spawn effects, refresh timers, or perform cleanup.
- `StatusEffectType` values are registered via `newStatusEffectDefinitions`. Add new entries there when introducing a status effect.
- `statusEffectInstance` stores per-target state: timestamps, the next scheduled tick, and any attached looping effect.

## Runtime flow
1. Systems call `World.applyStatusEffect` to apply or refresh a status effect on an actor. The helper creates an instance, runs the optional `OnApply` hook, and ensures ticks are scheduled.
2. Each simulation step calls `World.advanceStatusEffects(now)`. The method:
   - Executes any due ticks (`OnTick`) so gameplay effects happen on schedule.
   - Extends or expires attached visuals via helpers like `extendAttachedEffect`.
   - Invokes `OnExpire` once the status effect timer completes and removes the instance from the actor.
3. `effectState` gained a `FollowActorID`, allowing the `advanceNonProjectiles` hook to keep looping effects aligned with the actor every frame.

## Burning example
- Lava hazards call `applyStatusEffect` with `StatusEffectBurning` when an actor overlaps the obstacle.
- Fireball impacts also call `applyStatusEffect` so direct hits ignite the target before the lava timer kicks in.
- The `OnApply` hook spawns a looping `fire` effect that follows the actor and refreshes while the status effect is active.
- Every `200ms` the `OnTick` handler spawns a `burning-tick` effect that uses `healthDeltaBehavior` to deduct health, so the damage path reuses the existing effect behaviours.
- After three seconds without refresh, `OnExpire` cleans up the attached fire effect and the actor stops taking damage.

Add future status effects by extending the registry, supplying appropriate effect hooks, and invoking `applyStatusEffect` from the relevant gameplay system.
