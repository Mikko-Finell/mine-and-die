# JS Effects Workspace Guidelines

These instructions apply to every file beneath `tools/js-effects/`.

## Purpose
The workspace houses the TypeScript source for the canvas effects runtime (`packages/effects-lib`) and the interactive playground (`apps/playground`). The compiled ESM output is consumed by `client/js-effects` in the main game.

## Architecture Notes
- **Runtime (`packages/effects-lib/`)**
  - Export deterministic `EffectDefinition` objects that spawn `EffectInstance`s through the shared `EffectManager`.
  - Instances advance with `update(frameContext)` and render via `draw(frameContext)`. Keep simulation logic out of draw calls.
  - Always provide reasonable bounds from `getAABB()` to keep culling efficient.
  - Document any new `EffectLayer`/`sublayer` values in both this file and the workspace README so host apps can respect draw order.
  - Use the pooled helpers (`createPooled`) instead of ad-hoc object recycling.
- **Playground (`apps/playground/`)**
  - React app that exercises the runtime. `App.tsx` owns the animation loop and seeds the RNG each frame to preserve determinism.
  - Wire new preset or option fields into the control schema so they can be tweaked live.
  - Keep playground-only utilities isolated; reusable helpers should live in the runtime package.
- **Shared config**
  - `package.json` defines workspace scripts. New scripts must run from the workspace root and respect npm workspaces.
  - `tsconfig.base.json` feeds both packages. Coordinate any compiler option changes with the game client before landing them.

## Contribution Workflow
- Run `npm install` once to hydrate the workspace, then use `npm run build` to emit the ESM artefacts consumed by the main repo.
- Prefer TypeScript strictness—add typings instead of suppressing errors.
- Update `README.md` or `tutorial.md` whenever you add capabilities, lifecycle hooks, or notable debugging tips.
- Keep example assets lightweight and under version control; do not check in large binaries.
- Ensure RNG-driven demos rely on the provided frame context RNG so behaviour matches the game client.
- If you rename or add exports that the client relies on, update the generated build (`npm run build` at the game repo root) within the same PR.
