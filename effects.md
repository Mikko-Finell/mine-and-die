# Visual Effects System Review

## Dedicated decal support
The runtime ships with a `DecalSpec` concept inside the shared `EffectManager`; finished instances can expose `handoffToDecal()` and the manager returns those specs from `collectDecals()`.【F:client/js-effects/manager.js†L130-L144】【F:client/js-effects/effects/bloodSplatter.js†L134-L135】【F:client/js-effects/effects/bloodSplatter.js†L275-L326】 The client now consumes that pipeline by collecting finished decals each frame, queueing them in `store.activeDecals`, and redrawing them before updating transient effects so stains and scorch marks persist independently of their parent animations.【F:client/render.js†L40-L213】【F:client/render.js†L456-L609】

## Animation-to-decal handoff
Blood splatter instances continue to convert their final state into a decal canvas, but the render loop now promotes that handoff into a long-lived ground mark. Finished splatters yield a decal through `EffectManager.collectDecals()`, and `queueDecals` adds the spec to the local cache so it can be drawn every frame until its optional `ttl` expires.【F:client/js-effects/effects/bloodSplatter.js†L275-L326】【F:client/render.js†L40-L213】【F:client/render.js†L456-L609】

## Animation completion signalling
The effect manager still drives lifecycle via `isAlive()`, but fire-and-forget triggers have replaced the bespoke `syncBloodSplatterEffects` bookkeeping. A dedicated handler spawns the blood splatter animation, lets it run to completion, and then relies on the shared decal queue to keep the resulting stain visible with no ad-hoc state mirrors.【F:client/render.js†L40-L213】【F:client/render.js†L456-L609】

## Server fire-and-forget capability
When melee attacks land on goblins or rats the simulation now enqueues a blood-splatter `EffectTrigger` instead of adding a long-lived effect record. The helper still records the hit position and duration, but it passes that context through `QueueEffectTrigger`, letting clients render the animation immediately and manage its decal locally.【F:server/effects.go†L211-L336】

## Server-triggered fire-and-forget events
Clients continue to mirror the `effects` array from each snapshot,【F:client/network.js†L200-L296】 but fire-and-forget triggers are drained separately. `processFireAndForgetTriggers` forwards each trigger to registered handlers—such as the new blood splatter hook—which create animations on demand through the shared `EffectManager`. Once those animations finish they hand off decals that the renderer stores and replays without further network input.【F:client/render.js†L1-L213】【F:client/render.js†L456-L609】
