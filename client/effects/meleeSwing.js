import { EffectLayer } from "../js-effects/types.js";

const DEFAULT_OPTIONS = {
  duration: 0.18,
  width: 48,
  height: 48,
  fill: "rgba(239, 68, 68, 0.32)",
  stroke: "rgba(239, 68, 68, 0.9)",
  strokeWidth: 3,
  innerFill: "rgba(254, 242, 242, 0.9)",
  innerInset: 6,
  fadeExponent: 1.5,
};

class MeleeSwingInstance {
  constructor(options) {
    const width = Number.isFinite(options?.width) ? options.width : DEFAULT_OPTIONS.width;
    const height = Number.isFinite(options?.height)
      ? options.height
      : DEFAULT_OPTIONS.height;
    const duration = Math.max(
      0.05,
      Number.isFinite(options?.duration) ? options.duration : DEFAULT_OPTIONS.duration,
    );

    this.opts = {
      ...DEFAULT_OPTIONS,
      ...options,
      width,
      height,
      duration,
    };

    this.id =
      typeof options?.effectId === "string"
        ? options.effectId
        : `melee-swing-${Math.random().toString(36).slice(2)}`;
    this.type = MeleeSwingEffectDefinition.type;
    this.layer = EffectLayer.ActorOverlay;

    const originX = Number.isFinite(options?.x) ? options.x : 0;
    const originY = Number.isFinite(options?.y) ? options.y : 0;
    this.origin = { x: originX, y: originY };

    this.elapsed = 0;
    this.finished = false;

    this.aabb = { x: originX, y: originY, w: width, h: height };
  }

  isAlive() {
    return !this.finished;
  }

  getAABB() {
    return this.aabb;
  }

  update(frame) {
    if (this.finished) {
      return;
    }
    const dt = Math.max(0, frame?.dt ?? 0);
    if (dt <= 0) {
      return;
    }
    this.elapsed += dt;
    if (this.elapsed >= this.opts.duration) {
      this.finished = true;
    }
  }

  draw(frame) {
    if (this.finished) {
      return;
    }
    const { ctx, camera } = frame;
    if (!ctx || !camera) {
      return;
    }

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

    const inset = Math.max(0, Math.min(this.opts.innerInset, Math.min(this.opts.width, this.opts.height) / 2));
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

  dispose() {
    // no-op; included for symmetry with other effect instances
  }
}

export const MeleeSwingEffectDefinition = {
  type: "melee-swing",
  defaults: DEFAULT_OPTIONS,
  create: (options) => new MeleeSwingInstance(options ?? {}),
};

