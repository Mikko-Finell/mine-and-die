# Playground Animations Guide

This guide explains how the JS Effects playground discovers, instantiates, and
renders animations. Follow it when building a new animation so that dropping the
definition into the effects library automatically makes it available inside the
playground UI.

## Where animations live

All reusable animations live under
`tools/js-effects/packages/effects-lib/src/effects/`. Each animation exports an
`EffectDefinition` object and usually keeps its instance implementation in the
same file. The library re-exports every definition from
`packages/effects-lib/src/index.ts`, which is what the playground and the game
client import.

```text
packages/effects-lib/
└── src/
    ├── index.ts         # Re-export hub for every effect definition
    ├── manager.ts       # Runtime that owns instances
    ├── types.ts         # Shared interfaces (EffectDefinition, EffectInstance, ...)
    └── effects/
        ├── placeholderAura.ts
        ├── fire.ts
        └── ...
```

To add a new animation:

1. Create `<effect-name>.ts` in `effects/`.
2. Export a definition (for example, `export const MyEffectDefinition = ...`).
3. Re-export it from `src/index.ts` so callers can `import { MyEffectDefinition }`.
4. Run `npm run build` from the repository root to refresh the compiled ESM
   bundle served to the client.

## The required interface

Animations implement the `EffectDefinition<TOptions>` contract. The definition
is a factory that produces `EffectInstance` objects responsible for simulation
and drawing, and in the unified pipeline it also exposes a
`fromEffect` helper so contract payloads from the server can be converted into
spawn options automatically. [client/render.js](../../client/render.js) [client/js-effects/effects/meleeSwing.js](../../client/js-effects/effects/meleeSwing.js)

```ts
export interface EffectDefinition<TOptions> {
  type: string;
  defaults: TOptions;
  create(opts: Partial<TOptions> & { x: number; y: number }): EffectInstance;
  createFromPreset?(
    position: { x: number; y: number },
    preset?: EffectPreset | Partial<EffectPreset>,
    overrides?: Record<string, unknown>
  ): EffectInstance;
  fromEffect?(
    effect: Record<string, unknown>,
    store: Record<string, unknown>,
    lifecycleEntry?: Record<string, unknown>
  ): Record<string, unknown> | null;
}
```

The playground calls `create` with the current option overrides and the origin
coordinate whenever you click "Spawn". Optional `createFromPreset` helpers let
other hosts materialise the animation from serialized presets, but the
playground only requires `type`, `defaults`, and `create`. When an animation is
used inside the game client, `render.syncEffectsByType` will invoke
`fromEffect` (when provided) with the authoritative contract payload so the
definition can translate quantized geometry, durations, and custom params into
runtime-friendly options. [client/render.js](../../client/render.js)

The instance returned by `create` must implement the `EffectInstance`
interface:

```ts
export interface EffectInstance {
  readonly id: string;
  readonly type: string;
  layer: EffectLayer;      // Controls draw order (e.g. GroundDecal, ActorOverlay)
  sublayer?: number;       // Optional fine-grained ordering
  kind?: "once" | "loop";  // Defaults to "loop" when omitted

  isAlive(): boolean;      // Return false to let the manager cull the instance
  dispose?(): void;        // Optional cleanup hook when the instance is removed
  handoffToDecal?(): DecalSpec | null; // For "once" effects that leave decals behind

  getAABB(): { x: number; y: number; w: number; h: number }; // Used for culling

  update(frame: EffectFrameContext): void; // Advance internal state each frame
  draw(frame: EffectFrameContext): void;   // Render to the supplied canvas context
}
```

The `EffectFrameContext` contains the canvas context, timing data, the camera
transform, and an optional RNG seeded by the playground. Always honour the
`camera` helper when converting world coordinates to screen space so zooming and
panning work transparently.

## Example: Placeholder Aura

`placeholderAura.ts` demonstrates the minimal set of hooks required for a
looping animation:

- The constructor merges the definition defaults with caller overrides and
  caches the spawn position.
- `update` advances a sine-wave pulse that resizes the aura and refreshes the
  cached axis-aligned bounding box (`getAABB`) so the manager can cull it when
  it leaves the viewport.
- `draw` converts the stored world coordinate into screen space via the camera
  helper and renders a ring of gradients.
- `isAlive` always returns `true` because the aura loops indefinitely. One-shot
  effects instead return `false` once their duration elapses and may set
  `kind: "once"` to signal that they will expire soon.

```ts
export const PlaceholderAuraDefinition: EffectDefinition<PlaceholderAuraOptions> = {
  type: "placeholder-aura",
  defaults: {
    radius: 60,
    pulseSpeed: 2,
    particleCount: 12,
    colors: ["#ff7f50", "#ffa500"],
  },
  create: (opts) => new PlaceholderAuraInstance(opts),
};
```

You can use the same structure when creating your own effect: implement a class
that satisfies `EffectInstance`, wire it to a definition, and export the
definition.

## Making the playground discover the animation

Once the definition is exported from the library, register it with the
playground UI so that it shows up in the effect catalog and has adjustable
controls.

1. **Add catalog metadata** – Append a record to the `availableEffects` array in
   `tools/js-effects/apps/playground/src/effects.ts`. The entry must reference
   the exported definition, a unique `id`, and human-readable strings so the UI
   can display it.
2. **Expose controls (optional)** – If the animation has tunable numeric or
   palette options, add an entry in the `effectControls` map inside
   `App.tsx`. Controls map option keys to sliders or color inputs so users can
   tweak your effect at runtime.
3. **Verify in the playground** – Run `npm run dev` and select your effect from
   the list. Adjust controls and confirm the instance spawns, updates, and
   disappears as expected.

With these steps complete, your new animation can be dropped into the effects
library and will immediately be usable in both the playground and the main game
client.
