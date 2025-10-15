import { EffectLayer } from "../types.js";

const nowSeconds = () => (typeof performance !== "undefined" ? performance.now() / 1000 : Date.now() / 1000);

class RectZoneEffectInstance {
  constructor(opts) {
    this.type = typeof opts.effectType === "string" && opts.effectType.length > 0 ? opts.effectType : RectZoneEffectInstance.baseType;
    this.layer = opts.layer ?? EffectLayer.ActorOverlay;
    this.sublayer = Number.isFinite(opts.sublayer) ? opts.sublayer : 0;
    this.id = typeof opts.effectId === "string" && opts.effectId.length > 0
      ? opts.effectId
      : `rect-zone-${Math.random().toString(36).slice(2)}`;
    const width = Math.max(0, Number.isFinite(opts.width) ? opts.width : 0);
    const height = Math.max(0, Number.isFinite(opts.height) ? opts.height : 0);
    const x = Number.isFinite(opts.x) ? opts.x : 0;
    const y = Number.isFinite(opts.y) ? opts.y : 0;
    this.aabb = { x, y, w: width, h: height };
    this.fill = typeof opts.fill === "string" && opts.fill.length > 0 ? opts.fill : "rgba(255, 255, 255, 0.25)";
    this.stroke = typeof opts.stroke === "string" && opts.stroke.length > 0 ? opts.stroke : "rgba(255, 255, 255, 0.9)";
    this.lineWidth = Number.isFinite(opts.lineWidth) ? opts.lineWidth : 2;
    this.duration = Math.max(0, Number.isFinite(opts.duration) ? opts.duration : 0);
    this.spawnedAt = Number.isFinite(opts.spawnedAt) ? opts.spawnedAt : nowSeconds();
    this.finished = false;
  }

  getAABB() {
    return this.aabb;
  }

  isAlive() {
    if (this.duration === 0) {
      return !this.finished;
    }
    const age = nowSeconds() - this.spawnedAt;
    return !this.finished && age < this.duration;
  }

  update(frame) {
    if (this.duration === 0) {
      return;
    }
    const current = Number.isFinite(frame?.now) ? frame.now : nowSeconds();
    if (current - this.spawnedAt >= this.duration) {
      this.finished = true;
    }
  }

  draw(frame) {
    const { ctx } = frame;
    if (!ctx) {
      return;
    }
    const { x, y, w, h } = this.aabb;
    ctx.save();
    ctx.fillStyle = this.fill;
    ctx.strokeStyle = this.stroke;
    ctx.lineWidth = this.lineWidth;
    ctx.fillRect(x, y, w, h);
    ctx.strokeRect(x, y, w, h);
    ctx.restore();
  }

  dispose() {
    this.finished = true;
  }
}

RectZoneEffectInstance.baseType = "rect-zone";

function resolveDimensions(effect, store) {
  const tileSize = Number.isFinite(store?.TILE_SIZE) ? store.TILE_SIZE : 40;
  const width = Number.isFinite(effect?.width) ? effect.width : tileSize;
  const height = Number.isFinite(effect?.height) ? effect.height : tileSize;
  const x = Number.isFinite(effect?.x) ? effect.x : 0;
  const y = Number.isFinite(effect?.y) ? effect.y : 0;
  return { x, y, width, height };
}

export function makeRectZoneDefinition(effectType, defaults = {}) {
  const baseDefaults = {
    fill: "rgba(255, 255, 255, 0.25)",
    stroke: "rgba(255, 255, 255, 0.9)",
    lineWidth: 2,
    layer: EffectLayer.ActorOverlay,
    sublayer: 0,
  };
  const mergedDefaults = { ...baseDefaults, ...defaults };
  return {
    type: effectType,
    defaults: mergedDefaults,
    create: (opts) => new RectZoneEffectInstance({ ...mergedDefaults, ...opts, effectType }),
    fromEffect: (effect, store) => {
      if (!effect || typeof effect !== "object") {
        return null;
      }
      const { x, y, width, height } = resolveDimensions(effect, store);
      const duration = Number.isFinite(effect.duration) ? Math.max(0, effect.duration / 1000) : 0;
      return {
        effectId: typeof effect.id === "string" ? effect.id : undefined,
        x,
        y,
        width,
        height,
        duration,
        spawnedAt: nowSeconds(),
        fill: mergedDefaults.fill,
        stroke: mergedDefaults.stroke,
        lineWidth: mergedDefaults.lineWidth,
        layer: mergedDefaults.layer,
        sublayer: mergedDefaults.sublayer,
      };
    },
  };
}

export function updateRectZoneInstance(instance, effect, store) {
  if (!instance || typeof instance !== "object" || !effect || typeof effect !== "object") {
    return;
  }
  const { x, y, width, height } = resolveDimensions(effect, store);
  instance.aabb.x = x;
  instance.aabb.y = y;
  instance.aabb.w = width;
  instance.aabb.h = height;
}
