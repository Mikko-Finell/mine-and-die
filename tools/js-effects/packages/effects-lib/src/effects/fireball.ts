import {
  EffectLayer,
  type EffectDefinition,
  type EffectFrameContext,
  type EffectInstance,
  type EffectPreset,
} from "../types.js";

const TAU = Math.PI * 2;

const clamp01 = (value: number): number =>
  Math.max(0, Math.min(1, value));

interface TrailPoint {
  x: number;
  y: number;
  age: number;
}

interface Orbiter {
  angle: number;
  distance: number;
  speed: number;
  phase: number;
}

export interface FireballOptions {
  speed: number;
  range: number;
  radius: number;
  glowRadius: number;
  tailLength: number;
  tailTaper: number;
  wobbleAmplitude: number;
  wobbleFrequency: number;
  swirlSpeed: number;
  emberCount: number;
  emberSize: number;
  emberDrift: number;
  additive: boolean;
  heading: number;
  fadeOutDuration: number;
  coreColor: string;
  midColor: string;
  rimColor: string;
  trailColor: string;
  emberColor: string;
  decalColor: string;
}

class FireballInstance implements EffectInstance<FireballOptions> {
  readonly id: string;
  readonly type = FireballEffectDefinition.type;
  layer = EffectLayer.ActorOverlay;
  sublayer = 0;
  kind: "once" = "once";

  private readonly opts: FireballOptions;
  private readonly origin: { x: number; y: number };
  private readonly direction: { x: number; y: number };
  private readonly normal: { x: number; y: number };
  private readonly aabb = { x: 0, y: 0, w: 0, h: 0 };
  private readonly trail: TrailPoint[] = [];
  private readonly orbiters: Orbiter[] = [];

  private readonly wobblePhase: number;
  private readonly trailDuration: number;
  private readonly trailMaxSamples: number;
  private readonly fadeDuration: number;

  private position: { x: number; y: number };
  private elapsed = 0;
  private distanceTravelled = 0;
  private finished = false;
  private finishing = false;
  private fadeElapsed = 0;
  private coreOpacity = 1;
  private decalEmitted = false;

  constructor(opts: Partial<FireballOptions> & { x: number; y: number }) {
    this.opts = { ...FireballEffectDefinition.defaults, ...opts };
    this.id = `fireball-${Math.random().toString(36).slice(2)}`;
    this.origin = { x: opts.x, y: opts.y };
    this.position = { x: opts.x, y: opts.y };

    const heading = this.opts.heading ?? 0;
    const dirX = Math.cos(heading);
    const dirY = Math.sin(heading);
    const length = Math.hypot(dirX, dirY) || 1;
    this.direction = { x: dirX / length, y: dirY / length };
    this.normal = { x: -this.direction.y, y: this.direction.x };

    this.wobblePhase = Math.random() * TAU;
    this.trailDuration = Math.max(
      0.08,
      this.opts.tailLength / Math.max(1, this.opts.speed)
    );
    this.trailMaxSamples = Math.max(12, Math.ceil(this.trailDuration / 0.016));
    this.fadeDuration = Math.max(0.05, this.opts.fadeOutDuration);

    const orbiters = Math.max(0, Math.round(this.opts.emberCount));
    for (let i = 0; i < orbiters; i += 1) {
      const ratio = i / Math.max(1, orbiters);
      const baseAngle = ratio * TAU;
      const distance = this.opts.radius * (0.45 + 0.25 * (i % 3));
      const speed = this.opts.swirlSpeed * (0.5 + 0.25 * (i % 2));
      const phase = Math.random() * TAU;
      this.orbiters.push({ angle: baseAngle, distance, speed, phase });
    }

    this.trail.push({ x: this.position.x, y: this.position.y, age: 0 });
    this.recalculateBounds();
  }

  isAlive(): boolean {
    return !this.finished;
  }

  getAABB() {
    return this.aabb;
  }

  dispose(): void {
    this.trail.length = 0;
    this.orbiters.length = 0;
  }

  handoffToDecal() {
    if (!this.finished || this.decalEmitted) {
      return null;
    }
    this.decalEmitted = true;
    const rx = this.opts.radius * 1.6;
    const ry = this.opts.radius * 1.1;
    return {
      x: this.position.x,
      y: this.position.y,
      averageColor: this.opts.decalColor,
      ttl: 6,
      layerHint: "scorch",
      shape: {
        type: "oval" as const,
        rx,
        ry,
      },
    };
  }

  update(frame: EffectFrameContext): void {
    if (this.finished) {
      return;
    }

    const dt = Math.max(0, frame.dt);
    if (dt <= 0) {
      return;
    }

    const prevX = this.position.x;
    const prevY = this.position.y;

    this.elapsed += dt;

    if (!this.finishing) {
      const remaining = this.opts.range - this.distanceTravelled;
      const step = this.opts.speed * dt;
      const applied = Math.min(step, Math.max(0, remaining));
      this.distanceTravelled += applied;
      if (remaining <= 0.0001 || applied < step * 0.5) {
        this.finishing = true;
      }
    } else {
      this.fadeElapsed += dt;
      this.coreOpacity = clamp01(1 - this.fadeElapsed / this.fadeDuration);
      if (this.coreOpacity <= 0.001 && this.trail.length === 0) {
        this.finished = true;
        return;
      }
    }

    this.updatePosition();
    this.updateOrbiters(dt);
    this.updateTrail(prevX, prevY, dt);
    this.recalculateBounds();
  }

  draw(frame: EffectFrameContext): void {
    if (this.finished || this.coreOpacity <= 0) {
      return;
    }

    const { ctx, camera } = frame;
    const zoom = camera.zoom;

    ctx.save();
    if (this.opts.additive) {
      ctx.globalCompositeOperation = "lighter";
    }

    this.drawTrail(ctx, camera, zoom);
    this.drawCore(ctx, camera, zoom);
    this.drawOrbiters(ctx, camera, zoom);

    ctx.restore();
  }

  private updatePosition(): void {
    const wobble = Math.sin(this.elapsed * this.opts.wobbleFrequency + this.wobblePhase);
    const wobbleOffset = this.opts.wobbleAmplitude * wobble;
    this.position = {
      x:
        this.origin.x +
        this.direction.x * this.distanceTravelled +
        this.normal.x * wobbleOffset,
      y:
        this.origin.y +
        this.direction.y * this.distanceTravelled +
        this.normal.y * wobbleOffset,
    };
  }

  private updateOrbiters(dt: number): void {
    if (this.orbiters.length === 0) {
      return;
    }

    for (const orbiter of this.orbiters) {
      orbiter.angle = (orbiter.angle + (this.opts.swirlSpeed + orbiter.speed) * dt) % TAU;
    }
  }

  private updateTrail(prevX: number, prevY: number, dt: number): void {
    const dx = this.position.x - prevX;
    const dy = this.position.y - prevY;
    const distance = Math.hypot(dx, dy);
    const spacing = Math.max(2, this.opts.radius * 0.65);
    const samples = distance > 0 ? Math.max(1, Math.ceil(distance / spacing)) : 1;

    const shouldEmitTrail = !this.finishing || distance > 0.001;

    if (shouldEmitTrail) {
      for (let i = 1; i <= samples; i += 1) {
        const t = i / samples;
        this.trail.push({
          x: prevX + dx * t,
          y: prevY + dy * t,
          age: 0,
        });
      }
    }

    for (const point of this.trail) {
      point.age += dt;
    }

    const maxAge = this.trailDuration + this.fadeDuration;
    while (this.trail.length > this.trailMaxSamples) {
      this.trail.shift();
    }
    while (this.trail.length > 0 && this.trail[0].age > maxAge) {
      this.trail.shift();
    }

    if (this.finishing && this.trail.length === 0 && this.coreOpacity <= 0.001) {
      this.finished = true;
    }
  }

  private recalculateBounds(): void {
    let minX = this.position.x;
    let maxX = this.position.x;
    let minY = this.position.y;
    let maxY = this.position.y;

    for (const point of this.trail) {
      if (point.x < minX) minX = point.x;
      if (point.x > maxX) maxX = point.x;
      if (point.y < minY) minY = point.y;
      if (point.y > maxY) maxY = point.y;
    }

    const pad = Math.max(
      this.opts.glowRadius,
      this.opts.radius + this.opts.emberDrift + this.opts.wobbleAmplitude
    );

    this.aabb.x = minX - pad;
    this.aabb.y = minY - pad;
    this.aabb.w = maxX - minX + pad * 2;
    this.aabb.h = maxY - minY + pad * 2;
  }

  private drawTrail(
    ctx: CanvasRenderingContext2D,
    camera: EffectFrameContext["camera"],
    zoom: number
  ): void {
    if (this.trail.length === 0) {
      return;
    }

    const baseRadius = this.opts.radius;
    const taper = clamp01(this.opts.tailTaper);

    for (const point of this.trail) {
      const life = clamp01(1 - point.age / (this.trailDuration + this.fadeDuration));
      if (life <= 0) {
        continue;
      }
      const radius = baseRadius * (0.4 + (1 - taper) * life);
      const screenRadius = Math.max(0.5, radius * zoom);
      const alpha = life * this.coreOpacity * 0.9;
      const x = camera.toScreenX(point.x);
      const y = camera.toScreenY(point.y);

      ctx.globalAlpha = alpha;
      ctx.fillStyle = this.opts.trailColor;
      ctx.beginPath();
      ctx.arc(x, y, screenRadius, 0, TAU);
      ctx.fill();
    }
  }

  private drawCore(
    ctx: CanvasRenderingContext2D,
    camera: EffectFrameContext["camera"],
    zoom: number
  ): void {
    const x = camera.toScreenX(this.position.x);
    const y = camera.toScreenY(this.position.y);
    const coreRadius = Math.max(0.5, this.opts.radius * zoom);
    const glowRadius = Math.max(coreRadius, this.opts.glowRadius * zoom);

    ctx.globalAlpha = this.coreOpacity;

    const gradient = ctx.createRadialGradient(
      x,
      y,
      coreRadius * 0.2,
      x,
      y,
      glowRadius
    );
    gradient.addColorStop(0, this.opts.coreColor);
    gradient.addColorStop(0.45, this.opts.midColor);
    gradient.addColorStop(1, this.opts.rimColor);

    ctx.fillStyle = gradient;
    ctx.beginPath();
    ctx.arc(x, y, glowRadius, 0, TAU);
    ctx.fill();

    ctx.save();
    ctx.translate(x, y);
    ctx.rotate(this.elapsed * this.opts.swirlSpeed * 0.6);
    ctx.strokeStyle = "rgba(255, 255, 255, 0.35)";
    ctx.lineWidth = Math.max(1, coreRadius * 0.35);
    ctx.globalAlpha = this.coreOpacity * 0.8;
    ctx.beginPath();
    ctx.arc(0, 0, coreRadius * 0.7, -Math.PI / 3, Math.PI / 4);
    ctx.stroke();
    ctx.restore();
  }

  private drawOrbiters(
    ctx: CanvasRenderingContext2D,
    camera: EffectFrameContext["camera"],
    zoom: number
  ): void {
    if (this.orbiters.length === 0) {
      return;
    }

    const drift = this.opts.emberDrift;
    const emberSize = Math.max(0.5, this.opts.emberSize);

    ctx.globalAlpha = this.coreOpacity * 0.85;
    ctx.fillStyle = this.opts.emberColor;

    for (const orbiter of this.orbiters) {
      const orbitRadius =
        this.opts.radius +
        orbiter.distance +
        Math.sin(this.elapsed * 3 + orbiter.phase) * drift * 0.4;
      const px = this.position.x + Math.cos(orbiter.angle) * orbitRadius;
      const py = this.position.y + Math.sin(orbiter.angle) * orbitRadius;
      const screenX = camera.toScreenX(px);
      const screenY = camera.toScreenY(py);
      const size = Math.max(0.5, emberSize * zoom * (0.7 + 0.3 * Math.sin(orbiter.angle * 2)));

      ctx.beginPath();
      ctx.arc(screenX, screenY, size, 0, TAU);
      ctx.fill();
    }
  }
}

export const FireballEffectDefinition: EffectDefinition<FireballOptions> = {
  type: "fireball",
  defaults: {
    speed: 320,
    range: 200,
    radius: 12,
    glowRadius: 26,
    tailLength: 110,
    tailTaper: 0.6,
    wobbleAmplitude: 6,
    wobbleFrequency: 9,
    swirlSpeed: 7,
    emberCount: 6,
    emberSize: 2.4,
    emberDrift: 12,
    additive: true,
    heading: 0,
    fadeOutDuration: 0.25,
    coreColor: "rgba(255, 244, 220, 1.0)",
    midColor: "rgba(255, 174, 70, 0.95)",
    rimColor: "rgba(255, 120, 20, 0.0)",
    trailColor: "rgba(255, 170, 70, 0.7)",
    emberColor: "rgba(255, 220, 160, 0.8)",
    decalColor: "rgba(120, 70, 20, 0.7)",
  },
  create: (opts: Partial<FireballOptions> & { x: number; y: number }) =>
    new FireballInstance(opts),
  createFromPreset: (position, preset?: EffectPreset, overrides?) => {
    const baseOptions =
      (preset?.options as Partial<FireballOptions> | undefined) ?? {};
    const merged = {
      ...FireballEffectDefinition.defaults,
      ...baseOptions,
      ...(overrides as Partial<FireballOptions> | undefined),
    };
    return new FireballInstance({
      ...merged,
      x: position.x,
      y: position.y,
    });
  },
};
