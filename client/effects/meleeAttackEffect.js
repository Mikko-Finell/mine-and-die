import { EffectLayer } from "../js-effects/index.js";

const DEFAULT_OPTIONS = {
  width: 40,
  height: 40,
  duration: 0.2,
  fadeInRatio: 0.25,
  fillColor: "#ef4444",
  borderColor: "#f87171",
  glowColor: "rgba(248, 113, 113, 0.65)",
  glowBlur: 18,
  inset: 6,
};

class MeleeAttackEffectInstance {
  constructor(options) {
    const opts = { ...DEFAULT_OPTIONS, ...options };
    this.id = `melee-attack-${Math.random().toString(36).slice(2)}`;
    this.type = MeleeAttackEffectDefinition.type;
    this.layer = EffectLayer.ActorOverlay;
    this.kind = "once";

    this.origin = { x: opts.x, y: opts.y };
    this.size = { width: opts.width, height: opts.height };
    this.duration = Math.max(0.05, Number(opts.duration) || DEFAULT_OPTIONS.duration);
    this.fadeInRatio = Math.min(Math.max(opts.fadeInRatio, 0), 0.95);
    this.fillColor = opts.fillColor;
    this.borderColor = opts.borderColor;
    this.glowColor = opts.glowColor;
    this.glowBlur = opts.glowBlur;
    this.maxInset = Math.max(0, opts.inset);

    this.elapsed = 0;
    this.finished = false;
    this.aabb = {
      x: this.origin.x - this.maxInset,
      y: this.origin.y - this.maxInset,
      w: this.size.width + this.maxInset * 2,
      h: this.size.height + this.maxInset * 2,
    };
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
    if (this.elapsed >= this.duration) {
      this.finished = true;
    }
  }

  draw(frame) {
    if (this.finished) {
      return;
    }
    const { ctx, camera } = frame;
    const progress = Math.min(1, this.elapsed / this.duration);
    const eased = 1 - Math.pow(1 - progress, 3);
    const fadeIn = this.fadeInRatio > 0 ? Math.min(progress / this.fadeInRatio, 1) : 1;
    const visibility = Math.min(1, fadeIn * (1 - progress * 0.85));
    const insetProgress = Math.sin(progress * Math.PI);
    const inset = this.maxInset * insetProgress;

    const screenX = typeof camera?.toScreenX === "function"
      ? camera.toScreenX(this.origin.x)
      : this.origin.x;
    const screenY = typeof camera?.toScreenY === "function"
      ? camera.toScreenY(this.origin.y)
      : this.origin.y;
    const zoom = typeof camera?.zoom === "number" && Number.isFinite(camera.zoom)
      ? camera.zoom
      : 1;

    const width = this.size.width;
    const height = this.size.height;

    ctx.save();
    ctx.translate(screenX, screenY);
    ctx.scale(zoom, zoom);

    ctx.globalAlpha = visibility * 0.45;
    ctx.fillStyle = this.fillColor;
    ctx.shadowColor = this.glowColor;
    ctx.shadowBlur = this.glowBlur * (1 - eased * 0.6);
    ctx.beginPath();
    ctx.rect(inset, inset, width - inset * 2, height - inset * 2);
    ctx.fill();

    ctx.shadowBlur = 0;
    ctx.globalAlpha = visibility;
    ctx.lineWidth = 3 - eased * 1.5;
    ctx.strokeStyle = this.borderColor;
    ctx.strokeRect(inset, inset, width - inset * 2, height - inset * 2);

    ctx.restore();
  }

  dispose() {}
}

export const MeleeAttackEffectDefinition = {
  type: "melee-attack", // client-side visualization type
  defaults: DEFAULT_OPTIONS,
  create(options) {
    return new MeleeAttackEffectInstance(options);
  },
};

export function createMeleeAttackEffect(options) {
  return MeleeAttackEffectDefinition.create(options);
}
