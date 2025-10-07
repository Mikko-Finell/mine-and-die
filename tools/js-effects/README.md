# JS Effects

JS Effects is a monorepo that pairs a reusable canvas effects runtime with a playground for exploring how individual effects behave. The repository is designed for two complementary use cases:

1. **Authoring reusable visual effects** that can be embedded in any canvas-driven application.
2. **Iterating on effect parameters in real time** through an opinionated UI that mirrors in-game tooling.

Both packages are written in modern TypeScript and share a common build toolchain so that effect logic behaves the same in production builds as it does inside the playground.

## Repository Layout

The workspace is managed with npm workspaces. Each package focuses on a single responsibility:

| Path | Description |
| --- | --- |
| `packages/effects-lib/` | Runtime primitives for describing, updating, and rendering effects. Ships as a library that other apps can consume. |
| `apps/playground/` | A Vite + React app that mounts the library in an interactive environment. |

Shared configuration lives at the repository root (`package.json`, `tsconfig.base.json`) so that both the library and the playground compile against the same TypeScript baseline and lint rules.

## How the Effects Runtime Works

The `effects-lib` package exposes a declarative contract for effects. An effect definition describes how to create an effect instance and what default options it expects. Once instantiated, the runtime is responsible for three key phases:

1. **State Updates** – Every frame, the runtime provides an `EffectFrameContext` containing time deltas, a camera transform helper, and the current canvas context. Each effect instance can mutate its internal state during `update` calls based on this deterministic information.
2. **Visibility Culling** – Effect instances expose axis-aligned bounding boxes via `getAABB`. Consumers, such as the playground, can use these bounds to skip drawing work when an effect is outside the current viewport.
3. **Layered Drawing** – Each instance declares an `EffectLayer` (e.g., `GroundDecal`, `ActorOverlay`) and optional numeric `sublayer`. The hosting application groups draw calls by layer, sorts by sublayer and creation order, and then dispatches `draw` calls. This keeps stacking consistent even as effects are added or removed dynamically.

Effect instances also expose lifecycle helpers like `isAlive()` and `dispose()` so that long-running hosts can recycle transient effects without leaking resources. Instances may optionally declare `kind: "once" | "loop"` to hint at their lifecycle and can provide a `handoffToDecal()` method that returns a serialisable `DecalSpec` once they finish. Hosts that understand decals can swap a one-shot effect for a static mark without juggling timers, while legacy code can ignore the hook entirely.

```ts
class ImpactBurst implements EffectInstance<ImpactBurstOptions> {
  kind: "once" = "once";

  update(frame: EffectFrameContext) {
    // ...advance particles...
    if (this.elapsed >= this.opts.duration) this.finished = true;
  }

  handoffToDecal() {
    return this.finished
      ? { x: this.x, y: this.y, shape: { type: "oval", rx: 24, ry: 16 } }
      : null;
  }
}
```

`DecalSpec` is intentionally tiny—position, optional rotation, shape data, texture hints, colour, a suggested `ttl`, and an optional `layerHint`. The library never renders decals; it simply hands the description back to the host.

### Presets and deterministic helpers

Visual presets can be shared between authoring tools and runtime hosts through the new `EffectPreset` type. The `loadPreset(source)` helper accepts a URL or plain object, validates the payload, and returns the canonical shape ready to pass into effect factories.

```ts
const preset = await loadPreset("/presets/impact-burst.json");
const instance = definition.createFromPreset?.({ x, y }, preset, {
  duration: 1.2,
});
```

Every `EffectFrameContext` now accepts a `RandomGenerator` with a `seedFrom(id)` helper. The exported `createRandomGenerator(seed?)` utility turns stable ids (e.g. `${tick}:${effectId}`) into reproducible pseudo-random sequences so visual flourishes remain deterministic between runs.

### Runtime helpers

`effects-lib` exports a headless `EffectManager` that encapsulates the playground loop:

- `spawn` or `addInstance` register effects, maintaining layer, sublayer, and creation order metadata.
- `cullByAABB(view)` toggles update/draw work when a bounding box leaves the viewport.
- `updateAll(frame)` advances every non-culled instance, while `drawAll(frame)` renders them in sorted order.
- `collectDecals()` gathers any `DecalSpec` emitted by freshly finished effects.
- `getLastFrameStats()` surfaces per-frame update/draw/cull counts.

The library also ships a `createPooled(factory)` helper for lightweight instance pooling so heavy, bursty effects can avoid GC churn.

### EffectManager Coordination

The playground uses the shared `EffectManager` helper to orchestrate multiple effect instances:

- When an effect is spawned, the manager wraps the instance with bookkeeping metadata (layer, sublayer, creation order).
- `cullByAABB` toggles update/draw work for instances that leave the active view volume.
- `updateAll` steps each effect, evicting dead instances while allowing active ones to adjust their desired layering in real time.
- `drawAll` renders every visible instance, respecting layer and sublayer ordering.
- `collectDecals` returns `DecalSpec` objects that hosts can draw separately from live effects.
- `clear` releases all instances and resets counters so that repeated runs start deterministically.

Applications embedding the library can adopt the same pattern or integrate the core lifecycle hooks into their existing render loops.

### Authoring conventions

Effect implementations follow a few simple rules to stay compatible with the shared manager:

1. **Update vs. draw** – advance state in `update(frame)` and perform all canvas work inside `draw(frame)`.
2. **Finishing semantics** – "once" effects should flip their internal completion flag when visually done and, if appropriate, return a `DecalSpec` from `handoffToDecal()`.
3. **Accurate bounds** – keep `getAABB()` conservative so culling never clips visible pixels.
4. **Determinism knobs** – respect `frame.rng?.seedFrom?.(id)` so hosts can guarantee repeatable visuals.
5. **Layering** – render static marks on `EffectLayer.GroundDecal` and reserve higher layers for overlays.

## Playground Architecture

The playground demonstrates how to wire the runtime into a UI-driven experience:

1. **Effect catalog** – `apps/playground/src/effects.ts` exports metadata about available effects so the UI can render a browsable list and load the right definition.
2. **React UI layer** – `App.tsx` manages selection state, control panels, and persistence for user overrides. Each effect exposes a set of sliders mapped to specific options.
3. **Animation loop** – When the selected effect or its configuration changes, `App.tsx` rehydrates the canvas by creating a fresh effect instance through `EffectManager`. A `requestAnimationFrame` loop advances time, clears the canvas, re-seeds the shared RNG for deterministic playback, and asks the manager to update, collect decals, and draw in layer order.
4. **Camera abstraction** – The playground uses a simple 1:1 camera transform, but the `EffectFrameContext` contract anticipates more complex worlds. Hosts can provide their own `toScreenX`, `toScreenY`, and `zoom` logic without touching effect code.
5. **Persistence** – User slider adjustments are saved to `localStorage` so that returning to the playground restores personalized defaults per effect type.

## Developing Effects

When building new effects, follow this flow:

1. **Define options and defaults** that capture all tunable parameters. This ensures the playground can expose them through controls.
2. **Implement an `EffectDefinition`** with a `create` function that returns an `EffectInstance` implementing `update`, `draw`, `getAABB`, and lifecycle helpers.
3. **Register the definition** in `effects-lib/src/index.ts` so consumers can import it, and add catalog metadata to `apps/playground/src/effects.ts` with any custom controls in `App.tsx`.
4. **Validate in the playground** by tweaking sliders and confirming the effect responds correctly across layers and frame rates.

The runtime deliberately separates simulation (`update`) from rendering (`draw`). Avoid mutating canvas state outside of `draw`, and always recompute bounding boxes when the visual footprint changes so culling stays accurate.

## Getting Started

### Prerequisites

- [Node.js](https://nodejs.org/) **v18 or newer** (ships with the required npm version).
- npm **v9 or newer**.

### Install Dependencies

```bash
npm install
```

This command bootstraps both the library and the playground through npm workspaces.

### Run the Playground

```bash
npm run dev
```

This proxies to `npm run dev --workspace playground` and serves the playground at [http://localhost:5173](http://localhost:5173) by default.

### Build All Packages

```bash
npm run build
```

The build script compiles the library for distribution and produces an optimized production build of the playground.

### Lint the Workspace

```bash
npm run lint
```

Linting runs across every workspace to keep code style consistent.

## Extending the Monorepo

The project favors explicit coordination between packages over implicit shared state:

- **Adding a new effect** usually touches both `effects-lib` (for the implementation) and `apps/playground` (for catalog metadata and controls).
- **Sharing utilities** should happen inside `packages/effects-lib` so that consumers outside the monorepo can benefit from them.
- **Playground-only features** (e.g., UI affordances, debugging overlays) should stay within the `apps/playground` tree to avoid bloating the published library.

Keep this README synchronized with major architectural changes. Contributors should update documentation whenever they modify effect lifecycle semantics, add new layers, or restructure the workspace.

## Changelog

### v0.1.0

- Added `DecalSpec`, optional `EffectInstance.kind`, and `handoffToDecal()` to support one-shot effects that hand off to static decals without extra timers.
- Introduced `EffectPreset`, `loadPreset(source)`, and the seedable `RandomGenerator`/`createRandomGenerator` helpers for deterministic-yet-pretty authoring.
- Promoted a framework-agnostic `EffectManager` (`spawn`, `updateAll`, `drawAll`, `cullByAABB`, `collectDecals`, `getLastFrameStats`) alongside `createPooled` for instance recycling.
- Updated the playground to visualise decal hand-offs, long-lived looped effects, RNG determinism, and culling metrics in real time.
