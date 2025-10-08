import {
  type EffectDefinition,
  type EffectFrameContext,
  type EffectInstance,
  EffectLayer,
  type EffectPreset,
} from "../types.js";

const TAU = Math.PI * 2;
const clamp01 = (x: number): number => Math.max(0, Math.min(1, x));
const randRange = (rand: () => number, a: number, b: number): number =>
  a + rand() * (b - a);
const randInt = (rand: () => number, a: number, b: number): number =>
  Math.floor(randRange(rand, a, b + 1));

interface EmberParticle {
  x: number;
  y: number;
  vx: number;
  vy: number;
  life: number;
  age: number;
  size: number;
  color: string;
}

export interface FireOptions {
  spawnInterval: number;
  embersPerBurst: number;
  riseSpeed: number;
  windX: number;
  swirl: number;
  jitter: number;
  lifeScale: number;
  sizeScale: number;
  spawnRadius: number;
  concentration: number;
  emberPalette: string[];
  emberAlpha: number;
  additive: boolean;
}

class FireInstance implements EffectInstance<FireOptions> {
  readonly id: string;
  readonly type = FireEffectDefinition.type;
  layer = EffectLayer.ActorOverlay;
  sublayer = 0;
  kind: "loop" = "loop";
  finished = false;

  private opts: FireOptions;
  private readonly origin: { x: number; y: number };
  private readonly aabb = { x: 0, y: 0, w: 0, h: 0 };

  private readonly embers: EmberParticle[] = [];

  private spawnTimer = 0;

  constructor(opts: Partial<FireOptions> & { x: number; y: number; effectId?: string }) {
    const { effectId, ...rest } = opts;
    this.opts = { ...FireEffectDefinition.defaults, ...rest };
    this.id =
      typeof effectId === "string" && effectId.length > 0
        ? effectId
        : `fire-${Math.random().toString(36).slice(2)}`;
    this.origin = { x: opts.x, y: opts.y };
    this.updateAABB();
  }

  isAlive(): boolean {
    return !this.finished;
  }

  getAABB() {
    return this.aabb;
  }

  dispose(): void {
    this.embers.length = 0;
  }

  handoffToDecal(): null {
    return null;
  }

  setSizeScale(scale: number): void {
    if (!Number.isFinite(scale) || scale <= 0) {
      return;
    }
    this.opts = { ...this.opts, sizeScale: scale };
    this.updateAABB();
  }

  setCenter(x: number, y: number): void {
    this.origin.x = x;
    this.origin.y = y;
    this.updateAABB();
  }

  update(frame: EffectFrameContext): void {
    if (this.finished) {
      return;
    }

    const dt = Math.max(0, frame.dt);
    if (dt <= 0) {
      return;
    }

    const rand = frame.rng?.next?.bind(frame.rng) ?? Math.random;
    const {
      spawnInterval,
      embersPerBurst,
      riseSpeed,
      windX,
      swirl,
      jitter,
      lifeScale,
      sizeScale,
      spawnRadius,
      concentration,
      emberPalette,
    } = this.opts;

    this.spawnTimer += dt;

    while (this.spawnTimer >= spawnInterval) {
      this.spawnTimer -= spawnInterval;
      const n = randInt(rand, Math.max(1, embersPerBurst - 1), embersPerBurst + 1);
      for (let i = 0; i < n; i += 1) {
        const biasPow = 1 + concentration * 6;
        const r = spawnRadius * Math.pow(rand(), biasPow);
        const ang = randRange(rand, 0, TAU);

        const px =
          this.origin.x +
          Math.cos(ang) * r * (0.9 + 0.2 * rand());
        const py =
          this.origin.y +
          Math.sin(ang) * r * (0.35 + 0.15 * rand());

        const vx = windX + randRange(rand, -jitter, jitter);
        const vy = -riseSpeed * randRange(rand, 0.8, 1.25);

        const color = emberPalette.length
          ? emberPalette[Math.floor(rand() * emberPalette.length) % emberPalette.length]
          : "white";

        this.embers.push({
          x: px,
          y: py,
          vx,
          vy,
          life: randRange(rand, 0.7, 1.2) * lifeScale,
          age: 0,
          size: randRange(rand, 1.25, 2.25) * sizeScale,
          color,
        });
      }
    }

    for (let i = this.embers.length - 1; i >= 0; i -= 1) {
      const p = this.embers[i];
      p.age += dt;
      const s = Math.sin(p.age * 8) * swirl;
      p.vx += s * dt;
      p.x += p.vx * dt;
      p.y += p.vy * dt;

      if (p.age >= p.life) {
        const last = this.embers.pop();
        if (last && i < this.embers.length) {
          this.embers[i] = last;
        }
      }
    }
  }

  draw(frame: EffectFrameContext): void {
    if (this.finished) {
      return;
    }

    const { ctx, camera } = frame;
    const { emberAlpha, additive } = this.opts;

    ctx.save();
    if (additive) {
      ctx.globalCompositeOperation = "lighter";
    }

    for (const p of this.embers) {
      const x = camera.toScreenX(p.x);
      const y = camera.toScreenY(p.y);
      const r = (p.size || 1.6) * camera.zoom;
      const t = clamp01(p.age / p.life);
      const alpha = emberAlpha * (1 - t) * (0.65 + 0.35 * Math.sin(p.age * 18));

      ctx.save();
      ctx.globalAlpha = alpha;
      ctx.fillStyle = p.color;
      ctx.beginPath();
      ctx.arc(x, y, r, 0, TAU);
      ctx.fill();
      ctx.restore();
    }

    ctx.restore();
  }

  private updateAABB(): void {
    const rX = 56 * this.opts.sizeScale;
    const rY = 84 * this.opts.sizeScale;
    this.aabb.x = this.origin.x - rX;
    this.aabb.y = this.origin.y - rY;
    this.aabb.w = rX * 2;
    this.aabb.h = rY * 2;
  }
}

export const FireEffectDefinition: EffectDefinition<FireOptions> = {
  type: "fire",
  defaults: {
    spawnInterval: 0.08,
    embersPerBurst: 6,
    riseSpeed: 42,
    windX: 4,
    swirl: 6,
    jitter: 10,
    lifeScale: 1,
    sizeScale: 1,
    spawnRadius: 10,
    concentration: 0.6,
    emberPalette: [
      "rgba(255, 220, 150, 1.0)",
      "rgba(255, 180, 60, 1.0)",
      "rgba(255, 245, 200, 1.0)",
    ],
    emberAlpha: 0.9,
    additive: true,
  },
  create: (opts: Partial<FireOptions> & { x: number; y: number }) =>
    new FireInstance(opts),
  createFromPreset: (position, preset?: EffectPreset, overrides?) => {
    const baseOptions =
      (preset?.options as Partial<FireOptions> | undefined) ?? {};
    const merged = {
      ...FireEffectDefinition.defaults,
      ...baseOptions,
      ...(overrides as Partial<FireOptions> | undefined),
    };
    return new FireInstance({ ...merged, x: position.x, y: position.y });
  },
};
