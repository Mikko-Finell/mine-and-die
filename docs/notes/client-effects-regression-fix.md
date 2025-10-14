# Client Effects Regression Fix Report

## Summary
- Restored client-side effect rendering and lifecycle guardrails after multiple Vitest suites began failing (`blood-splatter`, `effect anchoring`, `effect ID reuse`, and `spawn/end batch` scenarios).
- Ensured blood-splatter visual contract entries spawn `EffectManager` instances every frame.
- Preserved one-tick effect visibility by mirroring ended lifecycle entries for a single render pass.
- Anchored zero-motion melee effects to their owner when the server omits explicit coordinates.
- Logged contract spawn rejections when identifiers are reused so regressions surface immediately during tests.

## Symptoms
- `client/__tests__/blood-splatter-rendering.test.js` observed zero tracked instances, indicating contract entries never reached the render manager.
- `client/__tests__/effects.anchor-zero-motion.test.js` reported melee decals spawning at world origin instead of near the owner.
- `client/__tests__/effects.id-reuse-rejected.test.js` expected a console error when a definition reused an identifier, but no log was emitted.
- `client/__tests__/effects.spawn-end-same-batch.test.js` confirmed one-tick melee effects were culled before the first render frame.

## Root Causes
1. `prepareEffectPass` never synchronized the `blood-splatter` lifecycle bucket with the `EffectManager`, leaving ground decal definitions disconnected from contract spawns.
2. The lifecycle state discarded ended entries immediately, so effects that spawned and ended in the same batch never reached the renderer.
3. `contractLifecycleToEffect` treated zeroed `positionX`/`positionY` values as authoritative world coordinates, ignoring the owning actor when the server omitted anchors.
4. Spawn dedupe logic silently ignored reused identifiers instead of surfacing a warning, making the regression invisible to tests.
5. `syncEffectsByType` assumed `EffectManager` keys matched contract identifiers; definitions such as `blood-splatter` generate their own instance IDs, so the cleanup loop immediately removed them.

## Fixes
- Added `blood-splatter` synchronization in `prepareEffectPass` so contract-derived visuals spawn via `BloodSplatterDefinition`.
- Extended lifecycle state to retain recently ended entries for one render version and taught the renderer to consume them once before culling, preserving one-tick effects.
- Updated the lifecycle translator to snap zero-velocity melee effects to their owner anchor when explicit motion coordinates are missing.
- Emitted descriptive `console.error` messages whenever a spawn is rejected because its sequence number regressed, matching test expectations.
- Tracked the authoritative contract identifier on spawned instances (`__hostEffectId`) and used it for cleanup comparisons so definitions with generated IDs remain alive through the frame.

## Validation
- All client Vitest suites now pass: `npm test -- --watch=false`.
- Manual inspection of debug logs confirmed blood-splatter instances spawn once per lifecycle entry and remain tracked for the first frame.

## Follow-up Recommendations
- Audit additional visual definitions for generated instance IDs to ensure the new host-ID tracking covers every case.
- Consider surfacing lifecycle replay diagnostics in the HUD so single-frame effects remain observable outside test runs.
