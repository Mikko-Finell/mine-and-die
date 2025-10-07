import {
  type EffectDefinition,
  type EffectFrameContext,
  type EffectInstance,
  EffectLayer
} from "../types";

export interface MeleeSwingOptions {
  duration: number;
  width: number;
  height: number;
  fill: string;
  stroke: string;
  strokeWidth: number;
  innerFill: string;
  innerInset: number;
  fadeExponent: number;
  effectId?: string;
}

class MeleeSwingInstance implements EffectInstance<MeleeSwingOptions> {
  readonly id: string;
  readonly type = MeleeSwingEffectDefinition.type;
  layer = EffectLayer.ActorOverlay;

  private readonly opts: MeleeSwingOptions;
  private readonly origin: { x: number; y: number };
  private readonly aabb = { x: 0, y: 0, w: 0, h: 0 };
  private elapsed = 0;
  private finished = false;

  constructor(opts: Partial<MeleeSwingOptions> & { x: number; y: number }) {
    const width = Number.isFinite(opts.width)
      ? (opts.width as number)
      : MeleeSwingEffectDefinition.defaults.width;
    const height = Number.isFinite(opts.height)
      ? (opts.height as number)
      : MeleeSwingEffectDefinition.defaults.height;
    const duration = Math.max(
      0.05,
      Number.isFinite(opts.duration)
        ? (opts.duration as number)
        : MeleeSwingEffectDefinition.defaults.duration,
    );
    const strokeWidth = Number.isFinite(opts.strokeWidth)
      ? (opts.strokeWidth as number)
      : MeleeSwingEffectDefinition.defaults.strokeWidth;
    const innerInset = Number.isFinite(opts.innerInset)
      ? (opts.innerInset as number)
      : MeleeSwingEffectDefinition.defaults.innerInset;
    const fadeExponent = Number.isFinite(opts.fadeExponent)
      ? (opts.fadeExponent as number)
      : MeleeSwingEffectDefinition.defaults.fadeExponent;

    this.opts = {
      ...MeleeSwingEffectDefinition.defaults,
      ...opts,
      width,
      height,
      duration,
      strokeWidth,
      innerInset,
      fadeExponent,
    };

    this.id =
      typeof opts.effectId === "string"
        ? opts.effectId
        : `melee-swing-${Math.random().toString(36).slice(2)}`;

    const originX = Number.isFinite(opts.x) ? opts.x : 0;
    const originY = Number.isFinite(opts.y) ? opts.y : 0;
    this.origin = { x: originX, y: originY };

    this.aabb.x = originX;
    this.aabb.y = originY;
    this.aabb.w = width;
    this.aabb.h = height;
  }

  isAlive(): boolean {
    return !this.finished;
  }

  getAABB() {
    return this.aabb;
  }

  update(frame: EffectFrameContext): void {
    if (this.finished) {
      return;
    }

    const dt = Math.max(0, frame.dt ?? 0);
    if (dt <= 0) {
      return;
    }

    this.elapsed += dt;
    if (this.elapsed >= this.opts.duration) {
      this.finished = true;
    }
  }

  draw(frame: EffectFrameContext): void {
    if (this.finished) {
      return;
    }

    const { ctx, camera } = frame;

    const duration = Math.max(this.opts.duration, 0.0001);
    const progress = Math.min(1, this.elapsed / duration);
    const intensity = 1 - Math.pow(progress, this.opts.fadeExponent);

    const screenX = camera.toScreenX(this.origin.x);
    const screenY = camera.toScreenY(this.origin.y);

    ctx.save();
    ctx.translate(screenX, screenY);
    ctx.scale(camera.zoom ?? 1, camera.zoom ?? 1);

    ctx.globalAlpha = intensity;
    ctx.fillStyle = this.opts.fill;
    ctx.fillRect(0, 0, this.opts.width, this.opts.height);

    const inset = Math.max(
      0,
      Math.min(
        this.opts.innerInset,
        Math.min(this.opts.width, this.opts.height) / 2,
      ),
    );
    if (inset > 0) {
      ctx.globalAlpha = intensity * 0.65;
      ctx.fillStyle = this.opts.innerFill;
      ctx.fillRect(
        inset,
        inset,
        Math.max(0, this.opts.width - inset * 2),
        Math.max(0, this.opts.height - inset * 2),
      );
    }

    ctx.globalAlpha = Math.min(1, intensity * 1.1);
    ctx.lineWidth = this.opts.strokeWidth;
    ctx.strokeStyle = this.opts.stroke;
    ctx.strokeRect(0, 0, this.opts.width, this.opts.height);

    ctx.restore();
  }

  dispose(): void {
    // no-op; included for symmetry with other effect instances
  }
}

export const MeleeSwingEffectDefinition: EffectDefinition<MeleeSwingOptions> = {
  type: "melee-swing",
  defaults: {
    duration: 0.18,
    width: 48,
    height: 48,
    fill: "rgba(239, 68, 68, 0.32)",
    stroke: "rgba(239, 68, 68, 0.9)",
    strokeWidth: 3,
    innerFill: "rgba(254, 242, 242, 0.9)",
    innerInset: 6,
    fadeExponent: 1.5,
  },
  create: (opts) => new MeleeSwingInstance(opts),
};
