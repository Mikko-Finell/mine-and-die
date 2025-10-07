# Getting Started with JS Effects

This guide walks through the JS Effects runtime, explains its core abstractions, and
shows how to orchestrate a complete effect loop from plain TypeScript. By the end you
will know how to spawn the built-in blood splatter, limit it to a one-shot animation,
and turn its final state into a decal.

## Installation

The runtime ships as the `effects-lib` workspace inside this monorepo. Install all
workspace dependencies once at the repository root:

```bash
npm install
```

The package exposes modern ESM builds, so you can import it from bundlers such as Vite,
Next.js, or plain Node + `ts-node` setups without extra configuration.

## Core Concepts

JS Effects follows a small set of contracts that separate simulation from rendering and
make each effect host-agnostic.

### `EffectDefinition`

Every effect starts as an `EffectDefinition<TOptions>` value exported from
`packages/effects-lib/src/effects`. A definition names the effect (`type`), exposes a
strongly typed `defaults` object, and provides factory functions:

```ts
import type { EffectDefinition } from "effects-lib";

declare const BloodSplatterDefinition: EffectDefinition<BloodSplatterOptions>;

BloodSplatterDefinition.type;      // "blood-splatter"
BloodSplatterDefinition.defaults;  // sensible option baseline
BloodSplatterDefinition.create({
  x: 0,
  y: 0,
  speed: 1,
});
```

The optional `createFromPreset` helper accepts a validated `EffectPreset` and lets hosts
inject additional overrides without knowing the full option shape.

### `EffectInstance`

Calling `definition.create(...)` returns an `EffectInstance`. Instances are long-lived
objects that own their internal state and implement a fixed set of methods defined in
[`src/types.ts`](packages/effects-lib/src/types.ts):

- `update(frame: EffectFrameContext)` – advance internal state.
- `draw(frame: EffectFrameContext)` – render to the provided `CanvasRenderingContext2D`.
- `getAABB()` – return an axis-aligned bounding box for culling.
- `isAlive()` – signal when the effect can be removed.
- `handoffToDecal?()` – (optional) provide a `DecalSpec` that replaces the live effect.
- `dispose?()` – clean up resources once the effect is removed.

Every instance also carries metadata that the host can change at runtime:

```ts
instance.layer = EffectLayer.GroundDecal;
instance.sublayer = 0;          // fine-grained ordering within a layer
instance.kind = "once";         // hint for host UX (looping vs one-shot)
```

### `EffectFrameContext`

The frame context delivers everything an effect needs per update/draw:

```ts
const frame: EffectFrameContext = {
  ctx,            // CanvasRenderingContext2D for drawing
  dt,             // delta time in seconds
  now,            // absolute timestamp in seconds
  camera: {
    toScreenX: (worldX) => worldX,
    toScreenY: (worldY) => worldY,
    zoom: 1,
  },
  rng,            // optional seeded random generator
};
```

Hosts are free to adapt the camera mapping for their coordinate system. Effects call the
helpers instead of working with screen space directly.

### `EffectManager`

The `EffectManager` orchestrates multiple instances. It sorts draw calls by layer,
removes finished effects, and collects decals so the host can render static marks
separately:

```ts
import { EffectManager } from "effects-lib";

const manager = new EffectManager();

const instance = manager.spawn(BloodSplatterDefinition, { x: 200, y: 120 });

// Per frame
manager.cullByAABB({ x: 0, y: 0, w: 1280, h: 720 });
manager.updateAll(frame);
manager.drawAll(frame);
const decals = manager.collectDecals();
```

Key helpers include:

- `spawn` / `spawnFromPreset` – create instances from definitions.
- `addInstance` – register a custom instance you built manually.
- `cullByAABB` – toggle instances on/off based on the current view.
- `updateAll` & `drawAll` – step and render visible instances.
- `collectDecals` – gather `DecalSpec` objects emitted this frame.
- `getLastFrameStats` – retrieve update/draw/cull counters for profiling.
- `clear` – dispose everything and reset the manager.

### Deterministic randomness

When you need reproducible variation (for replays or networking), use the bundled
`createRandomGenerator` helper:

```ts
import { createRandomGenerator } from "effects-lib";

const rng = createRandomGenerator("session-42");
const frame = { ...baseFrame, rng };

manager.updateAll(frame);
```

Within an effect you can reseed the generator using `frame.rng?.seedFrom?.(id)` so that
per-particle randomness stays stable between runs.

### Presets

`loadPreset` validates presets from URLs or inline objects before they reach your effect
code:

```ts
import { loadPreset } from "effects-lib";

const preset = await loadPreset("/presets/gore.json");
manager.spawnFromPreset(BloodSplatterDefinition, { x: 160, y: 220 }, preset, {
  speed: 0.8,
});
```

### Instance pooling

If you create transient effects in bursts, `createPooled` gives you a resettable object
pool:

```ts
import { createPooled } from "effects-lib";

const bloodPool = createPooled(() =>
  BloodSplatterDefinition.create({ x: 0, y: 0 })
);

const splatter = bloodPool.acquire();
manager.addInstance(splatter);
```

Call `bloodPool.release(instance)` once an effect finishes to recycle it.

## Wiring the runtime into a canvas loop

The runtime expects a host-controlled render loop. A minimal implementation looks like
this:

```ts
import {
  EffectManager,
  createRandomGenerator,
  BloodSplatterDefinition,
  type EffectFrameContext,
} from "effects-lib";

const canvas = document.querySelector("canvas")!;
const ctx = canvas.getContext("2d")!;
const manager = new EffectManager();
const rng = createRandomGenerator();

manager.spawn(BloodSplatterDefinition, {
  x: canvas.width / 2,
  y: canvas.height / 2,
});

let lastTime = performance.now();

function frame(now: number) {
  const dt = (now - lastTime) / 1000;
  lastTime = now;

  const frameCtx: EffectFrameContext = {
    ctx,
    dt,
    now: now / 1000,
    camera: {
      toScreenX: (x) => x,
      toScreenY: (y) => y,
      zoom: 1,
    },
    rng,
  };

  ctx.clearRect(0, 0, canvas.width, canvas.height);

  manager.cullByAABB({ x: 0, y: 0, w: canvas.width, h: canvas.height });
  manager.updateAll(frameCtx);
  manager.drawAll(frameCtx);

  requestAnimationFrame(frame);
}

requestAnimationFrame(frame);
```

This loop keeps the blood splatter running as a perpetual effect, mirroring the
behaviour in the playground application.

## Customising the blood splatter

`BloodSplatterDefinition` exposes a rich option set:

```ts
manager.spawn(BloodSplatterDefinition, {
  x: 320,
  y: 180,
  spawnInterval: 1.2,
  minDroplets: 10,
  maxDroplets: 18,
  speed: 0.7,
  colors: ["#a30b11", "#400508"],
  maxStains: 60,
});
```

- `spawnInterval` and `speed` control how frequently droplets appear.
- `minDroplets`/`maxDroplets` determine burst density.
- `dropletRadius` tweaks the elliptical droplets drawn in `draw`.
- `minStainRadius`/`maxStainRadius` govern the size of the pooled stains.
- `maxStains` caps the circular buffer used for decals on the ground.

Because the effect is defined as `kind: "loop"`, it keeps emitting droplets forever by
default. The next section demonstrates how to wrap it into a one-shot animation that
hands off to a decal.

## Building a non-looping splatter that turns into a decal

The runtime allows you to provide your own `EffectInstance` implementations. We can wrap
the stock blood splatter so that it runs for a fixed duration, captures the final frame,
and returns a static decal via `handoffToDecal`.

```ts
import {
  BloodSplatterDefinition,
  EffectLayer,
  type BloodSplatterOptions,
  type DecalSpec,
  type EffectFrameContext,
  type EffectInstance,
} from "effects-lib";

const identityCamera = {
  toScreenX: (x: number) => x,
  toScreenY: (y: number) => y,
  zoom: 1,
};

class NonLoopingBloodSplatter implements EffectInstance<BloodSplatterOptions> {
  readonly type = "blood-splatter-once";
  readonly id: string;
  layer = EffectLayer.GroundDecal;
  sublayer?: number;
  kind: "once" = "once";

  private readonly inner: EffectInstance<BloodSplatterOptions>;
  private elapsed = 0;
  private finished = false;
  private decal: DecalSpec | null = null;
  private readonly offscreen = document.createElement("canvas");

  constructor(opts: Partial<BloodSplatterOptions> & { x: number; y: number; ttl?: number }) {
    this.inner = BloodSplatterDefinition.create(opts);
    this.id = `${this.inner.id}-once`;
    this.layer = this.inner.layer;
    this.sublayer = this.inner.sublayer;
  }

  isAlive() {
    return !this.finished;
  }

  getAABB() {
    return this.inner.getAABB();
  }

  update(frame: EffectFrameContext) {
    if (this.finished) {
      return;
    }

    this.elapsed += frame.dt;
    this.inner.update(frame);

    if (this.elapsed >= 0.9) {
      this.finished = true;
      this.captureDecal();
      this.inner.dispose?.();
    }
  }

  draw(frame: EffectFrameContext) {
    if (this.finished) {
      return;
    }
    this.inner.draw(frame);
  }

  dispose() {
    this.inner.dispose?.();
  }

  handoffToDecal() {
    return this.decal;
  }

  private captureDecal() {
    const bounds = this.inner.getAABB();
    const width = Math.ceil(bounds.w);
    const height = Math.ceil(bounds.h);
    if (width === 0 || height === 0) {
      return;
    }

    this.offscreen.width = width;
    this.offscreen.height = height;
    const offCtx = this.offscreen.getContext("2d")!;
    offCtx.clearRect(0, 0, width, height);

    // Render the effect once more into local space without camera transforms.
    offCtx.save();
    offCtx.translate(-bounds.x, -bounds.y);
    this.inner.draw({
      ctx: offCtx,
      dt: 0,
      now: performance.now() / 1000,
      camera: identityCamera,
    });
    offCtx.restore();

    this.decal = {
      x: bounds.x + bounds.w / 2,
      y: bounds.y + bounds.h / 2,
      rotation: 0,
      shape: { type: "rect", w: bounds.w, h: bounds.h },
      texture: this.offscreen,
      averageColor: "#6f0b10",
      ttl: 10,
      layerHint: "GroundDecal",
    };
  }
}
```

This wrapper delegates `update`, `draw`, and `getAABB` to the stock instance until the
configured duration elapses. At that point it renders one more frame into an off-screen
canvas and stores the result as a `DecalSpec`. Because it implements the `EffectInstance`
interface, you can register it with the regular `EffectManager`.

```ts
const splatter = new NonLoopingBloodSplatter({
  x: 280,
  y: 200,
  spawnInterval: 0.3,
  minDroplets: 14,
  maxDroplets: 22,
});

manager.addInstance(splatter);
```

During the main loop, call `manager.collectDecals()` after `drawAll`. The returned decal
contains the off-screen texture. Draw it in your world using your preferred decal
renderer and respect `ttl` if you want the splatter to fade out over time.

```ts
const decals = manager.collectDecals();
for (const decal of decals) {
  drawDecal(decal);
}
```

## Next steps

- Explore other definitions exported from `effects-lib/src/index.ts`, such as the
  `ImpactBurstDefinition` that already exposes a built-in `handoffToDecal()`.
- Wrap `EffectManager` inside your engine's entity system so characters can spawn effects
  without owning the lifecycle details.
- Create JSON presets with `loadPreset` to share art-directed settings between tools and
  the shipping game.

With these building blocks you can treat visual effects as reusable modules that slot
into any canvas-driven experience.
