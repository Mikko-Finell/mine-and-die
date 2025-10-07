import {
  EffectDefinition,
  EffectFrameContext,
  EffectInstance,
  EffectLayer,
  type EffectPreset,
} from "../types";

const TAU = Math.PI * 2;

export interface PlaceholderAuraOptions {
  radius: number;
  pulseSpeed: number;
  particleCount: number;
  colors: string[];
}

class PlaceholderAuraInstance implements EffectInstance<PlaceholderAuraOptions> {
  readonly id = `placeholder-aura-${Math.random().toString(36).slice(2)}`;
  readonly type = PlaceholderAuraDefinition.type;
  layer = EffectLayer.ActorOverlay;
  kind: "loop" = "loop";

  private readonly opts: PlaceholderAuraOptions;
  private readonly center: { x: number; y: number };
  private elapsed = 0;
  private currentPulse = 0.5;
  private currentRadius = 0;
  private readonly aabb = { x: 0, y: 0, w: 0, h: 0 };

  constructor(opts: Partial<PlaceholderAuraOptions> & { x: number; y: number }) {
    this.opts = { ...PlaceholderAuraDefinition.defaults, ...opts };
    this.center = { x: opts.x, y: opts.y };
    this.updatePulseState(0);
    this.updateAABB();
  }

  isAlive(): boolean {
    return true;
  }

  getAABB() {
    return this.aabb;
  }

  update(frame: EffectFrameContext): void {
    const dt = Math.max(0, frame.dt);
    if (dt <= 0) {
      return;
    }

    this.elapsed += dt * this.opts.pulseSpeed;
    this.updatePulseState(this.elapsed);
    this.updateAABB();
  }

  draw(frame: EffectFrameContext): void {
    const { ctx, camera } = frame;
    const particles = Math.max(1, Math.floor(this.opts.particleCount));
    const colors = this.opts.colors.length > 0 ? this.opts.colors : ["#ff7f50"];
    const particleRadius = 24 + this.currentPulse * 12;

    const screenX = camera.toScreenX(this.center.x);
    const screenY = camera.toScreenY(this.center.y);

    ctx.save();
    ctx.translate(screenX, screenY);
    ctx.scale(camera.zoom, camera.zoom);

    for (let i = 0; i < particles; i += 1) {
      const angle = (i / particles) * TAU;
      const px = Math.cos(angle) * this.currentRadius;
      const py = Math.sin(angle) * this.currentRadius;
      const colorIndex = i % colors.length;

      const gradient = ctx.createRadialGradient(px, py, 0, px, py, particleRadius);
      gradient.addColorStop(0, colors[colorIndex]);
      gradient.addColorStop(1, "rgba(255, 255, 255, 0)");

      ctx.beginPath();
      ctx.fillStyle = gradient;
      ctx.arc(px, py, particleRadius * 0.8, 0, TAU);
      ctx.fill();
    }

    ctx.restore();
  }

  dispose(): void {}

  private updatePulseState(time: number): void {
    const pulse = Math.sin(time) * 0.5 + 0.5;
    this.currentPulse = pulse;
    this.currentRadius = this.opts.radius * (0.75 + pulse * 0.5);
  }

  private updateAABB(): void {
    const particleRadius = 24 + this.currentPulse * 12;
    const reach = this.currentRadius + particleRadius;
    this.aabb.x = this.center.x - reach;
    this.aabb.y = this.center.y - reach;
    this.aabb.w = reach * 2;
    this.aabb.h = reach * 2;
  }
}

export const PlaceholderAuraDefinition: EffectDefinition<PlaceholderAuraOptions> = {
  type: "placeholder-aura",
  defaults: {
    radius: 60,
    pulseSpeed: 2,
    particleCount: 12,
    colors: ["#ff7f50", "#ffa500"],
  },
  create: (opts) => new PlaceholderAuraInstance(opts),
  createFromPreset: (position, preset?: EffectPreset, overrides?) => {
    const baseOptions =
      (preset?.options as Partial<PlaceholderAuraOptions> | undefined) ?? {};
    const merged = {
      ...PlaceholderAuraDefinition.defaults,
      ...baseOptions,
      ...(overrides as Partial<PlaceholderAuraOptions> | undefined),
    };
    return new PlaceholderAuraInstance({ ...merged, x: position.x, y: position.y });
  },
};

