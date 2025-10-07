# Effect and condition integration

**Question.** Is the engine already equipped with RPG-style "conditions" (poisoned, burning, etc.) so that an entity like a goblin could be set on fire, take ongoing damage, and have a `FireEffectDefinition` animation anchored to it?

## Server-side capabilities
- Actors now carry a dedicated condition map (`actorState.conditions`). Condition lifecycles are defined in `server/conditions.go`, which registers behaviours such as duration, tick cadence, and optional follow-on visuals.
- `World.applyCondition` instantiates a `conditionInstance`, schedules its ticks, and lets the instance drive existing effect behaviours. Damage-over-time is dealt by spawning lightweight `effectState`s that call `healthDeltaBehavior`, so conditions reuse the effect system for health changes.
- `advanceConditions` evaluates active instances each tick, invoking `OnTick` and `OnExpire` handlers while keeping any attached effects in sync. `effectState` gained a `FollowActorID` that `advanceNonProjectiles` uses to snap visuals to the owning actor every frame.

## Lava burning condition
- Entering a lava obstacle calls `applyCondition` with `ConditionBurning`. The definition applies immediately, ticking every `200ms` for three seconds and refreshing while the actor remains in lava.
- Each tick spawns a `burning-tick` effect that delivers damage through the existing `healthDeltaBehavior`. The condition also creates a looping `fire` effect instance that follows the actor until the timer expires, so the client renders the `FireEffectDefinition` without bespoke code.
- Leaving lava no longer stops the damage instantly; the condition persists until its timer completes, at which point the helper expires the attached effect and the burning stops.

## Conclusion
The condition system now sits alongside the effect pipeline and provides the missing persistence layer. Status-style behaviours (burning, poison, etc.) can be authored as new `ConditionDefinition`s without duplicating damage or rendering code, so the current infrastructure is sufficient for the scenarios described.
