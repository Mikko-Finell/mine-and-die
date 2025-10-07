# Repository Agent Instructions

## Purpose

This workspace contains the JS Effects runtime library and its interactive playground. The goal is to keep the runtime (`packages/effects-lib`) framework-agnostic while showcasing its capabilities through the playground (`apps/playground`). Both packages share a TypeScript toolchain configured at the repository root.

## Architectural Notes

1. **Effects Runtime (`packages/effects-lib/`)
   - Exposes `EffectDefinition` contracts that create long-lived `EffectInstance` objects.
   - Instances advance their internal simulation via `update(frameContext)` and render with `draw(frameContext)`.
   - Bounding boxes returned by `getAABB()` are critical for culling; keep them conservative but accurate when editing effects.
   - Layering (`EffectLayer` + optional `sublayer`) controls draw order. New layers must be documented in both this file and the README.
   - One-shot effects can expose `kind: "once"` and `handoffToDecal()`; returned `DecalSpec` objects are the only decal contract the library understands.
   - Presets are represented as `EffectPreset` objects. Use `loadPreset` for validation and `createRandomGenerator` to keep RNG-driven effects deterministic.
   - The shared `EffectManager` (spawn, cull, update, draw, collect decals, expose stats) and `createPooled` helpers live here for reuse across hosts.

2. **Playground App (`apps/playground/`)
   - Uses React for UI state and the shared `EffectManager` helper to orchestrate instances.
   - The animation loop lives in `App.tsx`, seeds the shared RNG each frame, collects decals, and visualises manager stats for debugging culling behaviour.
   - UI controls map to effect option keys. When introducing new options, wire them into the control schema so they can be tuned from the playground.

3. **Shared Configuration**
   - `package.json` at the root defines workspace scripts. Always add new scripts in a way that works from the repository root.
   - `tsconfig.base.json` underpins TypeScript settings. Updates here affect every package and should be coordinated carefully.

## Contribution Guidelines

- Documentation must reflect architectural changes. If you adjust effect lifecycles, add new layers, or rework the manager flow, update both this file and `README.md`.
- Avoid mixing playground-only utilities into `effects-lib`. Shared helpers should live in the library; debug-only or UI logic belongs in the app.
- Respect the effect lifecycle separation: computation during `update`, rendering during `draw`. New code should not blur these boundaries.
- Maintain deterministic behavior wherever possible. When optional randomness is required, surface it through the `EffectFrameContext` RNG hook.

## Maintenance Instruction

Always keep this AGENTS.md up to date.
