import {
  type EffectDefinition,
  type EffectFrameContext,
  type EffectInstance,
  EffectLayer,
} from "../types.js";

const nowSeconds = () =>
  typeof performance !== "undefined" ? performance.now() / 1000 : Date.now() / 1000;

export interface RectZoneOptions {
  width: number;
  height: number;
  duration: number;
  fill: string;
  stroke: string;
  lineWidth: number;
  layer: EffectLayer;
  sublayer: number;
  effectId?: string;
  spawnedAt?: number;
}

export interface RectZoneLifecycleLike {
  id?: string;
  x?: number;
  y?: number;
  width?: number;
  height?: number;
  duration?: number;
}

export interface RectZoneStoreLike {
  TILE_SIZE?: number;
}

interface Dimensions {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface RectZoneInstance extends EffectInstance<RectZoneOptions> {
  setDimensions(dimensions: Dimensions): void;
}

class RectZoneEffectInstance implements RectZoneInstance {
  static readonly baseType = "rect-zone";

  readonly id: string;
  readonly type: string;
  layer: EffectLayer;
  sublayer?: number;

  private readonly fill: string;
  private readonly stroke: string;
  private readonly lineWidth: number;
  private readonly duration: number;
  private readonly spawnedAt: number;

  private finished = false;
  private readonly aabb = { x: 0, y: 0, w: 0, h: 0 };

  constructor(effectType: string, position: { x: number; y: number }, options: RectZoneOptions) {
    this.type = effectType.length > 0 ? effectType : RectZoneEffectInstance.baseType;
    this.layer = options.layer ?? EffectLayer.ActorOverlay;
    this.sublayer = options.sublayer ?? 0;

    this.id =
      typeof options.effectId === "string" && options.effectId.length > 0
        ? options.effectId
        : `rect-zone-${Math.random().toString(36).slice(2)}`;

    this.fill = options.fill;
    this.stroke = options.stroke;
    this.lineWidth = options.lineWidth;

    const width = Math.max(0, Number.isFinite(options.width) ? options.width : 0);
    const height = Math.max(0, Number.isFinite(options.height) ? options.height : 0);
    const x = Number.isFinite(position.x) ? position.x : 0;
    const y = Number.isFinite(position.y) ? position.y : 0;
    this.aabb.x = x;
    this.aabb.y = y;
    this.aabb.w = width;
    this.aabb.h = height;

    this.duration = Math.max(0, Number.isFinite(options.duration) ? options.duration : 0);
    this.spawnedAt =
      typeof options.spawnedAt === "number" && Number.isFinite(options.spawnedAt)
        ? options.spawnedAt
        : nowSeconds();
  }

  isAlive(): boolean {
    if (this.duration === 0) {
      return !this.finished;
    }
    const age = nowSeconds() - this.spawnedAt;
    return !this.finished && age < this.duration;
  }

  getAABB() {
    return this.aabb;
  }

  update(frame: EffectFrameContext): void {
    if (this.duration === 0 || this.finished) {
      return;
    }
    const current = Number.isFinite(frame?.now) ? (frame.now as number) : nowSeconds();
    if (current - this.spawnedAt >= this.duration) {
      this.finished = true;
    }
  }

  draw(frame: EffectFrameContext): void {
    const ctx = frame.ctx;
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

  dispose(): void {
    this.finished = true;
  }

  setDimensions({ x, y, width, height }: Dimensions): void {
    this.aabb.x = x;
    this.aabb.y = y;
    this.aabb.w = width;
    this.aabb.h = height;
  }
}

function resolveDimensions(
  effect: RectZoneLifecycleLike | null | undefined,
  store: RectZoneStoreLike | null | undefined,
): Dimensions {
  const tileSize = Number.isFinite(store?.TILE_SIZE) ? (store?.TILE_SIZE as number) : 40;
  const width = Number.isFinite(effect?.width) ? (effect?.width as number) : tileSize;
  const height = Number.isFinite(effect?.height) ? (effect?.height as number) : tileSize;
  const x = Number.isFinite(effect?.x) ? (effect?.x as number) : 0;
  const y = Number.isFinite(effect?.y) ? (effect?.y as number) : 0;
  return { x, y, width, height };
}

export function makeRectZoneDefinition(
  effectType: string,
  defaults: Partial<RectZoneOptions> = {},
): EffectDefinition<RectZoneOptions> & {
  fromEffect?: (
    effect: RectZoneLifecycleLike,
    store?: RectZoneStoreLike,
  ) => (RectZoneOptions & { x: number; y: number; duration: number }) | null;
} {
  const baseDefaults: RectZoneOptions = {
    width: 40,
    height: 40,
    duration: 0,
    fill: "rgba(255, 255, 255, 0.25)",
    stroke: "rgba(255, 255, 255, 0.9)",
    lineWidth: 2,
    layer: EffectLayer.ActorOverlay,
    sublayer: 0,
  };

  const mergedDefaults: RectZoneOptions = {
    ...baseDefaults,
    ...defaults,
    width: Number.isFinite(defaults.width) ? (defaults.width as number) : baseDefaults.width,
    height: Number.isFinite(defaults.height) ? (defaults.height as number) : baseDefaults.height,
    duration: Number.isFinite(defaults.duration)
      ? Math.max(0, defaults.duration as number)
      : baseDefaults.duration,
    lineWidth: Number.isFinite(defaults.lineWidth)
      ? Math.max(0, defaults.lineWidth as number)
      : baseDefaults.lineWidth,
    layer:
      defaults.layer !== undefined ? (defaults.layer as EffectLayer) : baseDefaults.layer,
    sublayer:
      Number.isFinite(defaults.sublayer) && defaults.sublayer !== undefined
        ? (defaults.sublayer as number)
        : baseDefaults.sublayer,
    fill:
      typeof defaults.fill === "string" && defaults.fill.length > 0
        ? defaults.fill
        : baseDefaults.fill,
    stroke:
      typeof defaults.stroke === "string" && defaults.stroke.length > 0
        ? defaults.stroke
        : baseDefaults.stroke,
  };

  const definition: EffectDefinition<RectZoneOptions> & {
    fromEffect?: (
      effect: RectZoneLifecycleLike,
      store?: RectZoneStoreLike,
    ) => (RectZoneOptions & { x: number; y: number; duration: number }) | null;
  } = {
    type: effectType,
    defaults: mergedDefaults,
    create: (opts) => {
      const position = {
        x: Number.isFinite(opts.x) ? (opts.x as number) : 0,
        y: Number.isFinite(opts.y) ? (opts.y as number) : 0,
      };

      const width = Number.isFinite(opts.width)
        ? Math.max(0, opts.width as number)
        : mergedDefaults.width;
      const height = Number.isFinite(opts.height)
        ? Math.max(0, opts.height as number)
        : mergedDefaults.height;
      const duration = Number.isFinite(opts.duration)
        ? Math.max(0, opts.duration as number)
        : mergedDefaults.duration;

      const instanceOptions: RectZoneOptions = {
        ...mergedDefaults,
        ...opts,
        width,
        height,
        duration,
        fill:
          typeof opts.fill === "string" && opts.fill.length > 0
            ? opts.fill
            : mergedDefaults.fill,
        stroke:
          typeof opts.stroke === "string" && opts.stroke.length > 0
            ? opts.stroke
            : mergedDefaults.stroke,
        lineWidth: Number.isFinite(opts.lineWidth)
          ? Math.max(0, opts.lineWidth as number)
          : mergedDefaults.lineWidth,
        layer:
          opts.layer !== undefined
            ? (opts.layer as EffectLayer)
            : mergedDefaults.layer,
        sublayer:
          Number.isFinite(opts.sublayer) && opts.sublayer !== undefined
            ? (opts.sublayer as number)
            : mergedDefaults.sublayer,
      };

      return new RectZoneEffectInstance(effectType, position, instanceOptions);
    },
  };

  definition.fromEffect = (effect, store) => {
    if (!effect || typeof effect !== "object") {
      return null;
    }

    const { x, y, width, height } = resolveDimensions(effect, store);
    const durationMs = Number.isFinite(effect.duration)
      ? Math.max(0, (effect.duration as number) / 1000)
      : 0;

    return {
      ...mergedDefaults,
      effectId:
        typeof effect.id === "string" && effect.id.length > 0
          ? effect.id
          : mergedDefaults.effectId,
      x,
      y,
      width,
      height,
      duration: durationMs,
      spawnedAt: nowSeconds(),
    };
  };

  return definition;
}

export function updateRectZoneInstance(
  instance: RectZoneInstance,
  effect: RectZoneLifecycleLike,
  store?: RectZoneStoreLike,
): void {
  if (!instance || typeof instance !== "object" || !effect || typeof effect !== "object") {
    return;
  }

  const { x, y, width, height } = resolveDimensions(effect, store);
  instance.setDimensions({ x, y, width, height });
}
