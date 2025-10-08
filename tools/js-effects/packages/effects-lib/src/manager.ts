import {
  type DecalSpec,
  type EffectDefinition,
  type EffectFrameContext,
  type EffectInstance,
  EffectLayer,
  type EffectPreset,
} from "./types.js";

interface ManagedEffect {
  instance: EffectInstance<any>;
  layer: EffectLayer;
  sublayer: number;
  creationIndex: number;
  culled: boolean;
}

interface ManagedDecal {
  spec: DecalSpec;
  layer: EffectLayer;
  sublayer: number;
  creationIndex: number;
  culled: boolean;
  expiresAt: number;
  aabb: { x: number; y: number; w: number; h: number };
}

interface ViewBounds {
  x: number;
  y: number;
  w: number;
  h: number;
}

interface FrameStats {
  updated: number;
  drawn: number;
  culled: number;
}

type TriggerPayload = Record<string, unknown> | null | undefined;

type TriggerHandler = (params: {
  manager: EffectManager;
  trigger: TriggerPayload;
  context?: Record<string, unknown> | null;
}) => void;

const intersects = (a: ViewBounds, b: ViewBounds): boolean =>
  a.x < b.x + b.w && a.x + a.w > b.x && a.y < b.y + b.h && a.y + a.h > b.y;

const asPresetOptions = (
  preset?: EffectPreset | Partial<EffectPreset>
): Record<string, unknown> => {
  if (!preset || typeof preset !== "object") {
    return {};
  }
  const options = (preset as EffectPreset).options;
  if (options && typeof options === "object") {
    return options as Record<string, unknown>;
  }
  return {};
};

const resolveLayerFromHint = (hint?: string): EffectLayer => {
  if (typeof hint !== "string") {
    return EffectLayer.GroundDecal;
  }
  const normalized = hint.trim().toLowerCase();
  if (normalized === "actoroverlay" || normalized === "actor-overlay") {
    return EffectLayer.ActorOverlay;
  }
  return EffectLayer.GroundDecal;
};

const buildDecalAABB = (spec: DecalSpec) => {
  const centerX = Number.isFinite(spec.x) ? (spec.x as number) : 0;
  const centerY = Number.isFinite(spec.y) ? (spec.y as number) : 0;

  const fromShape = () => {
    const shape = spec.shape;
    if (!shape || typeof shape !== "object") {
      return null;
    }
    if (shape.type === "oval") {
      const rx = Number.isFinite(shape.rx) ? shape.rx : 0;
      const ry = Number.isFinite(shape.ry) ? shape.ry : 0;
      return { x: centerX - rx, y: centerY - ry, w: rx * 2, h: ry * 2 };
    }
    if (shape.type === "rect") {
      const w = Number.isFinite(shape.w) ? shape.w : 0;
      const h = Number.isFinite(shape.h) ? shape.h : 0;
      return { x: centerX - w / 2, y: centerY - h / 2, w, h };
    }
    if (shape.type === "poly" && Array.isArray(shape.points)) {
      const points = shape.points;
      if (points.length >= 4 && points.length % 2 === 0) {
        let minX = Infinity;
        let minY = Infinity;
        let maxX = -Infinity;
        let maxY = -Infinity;
        for (let i = 0; i < points.length; i += 2) {
          const px = Number(points[i]);
          const py = Number(points[i + 1]);
          if (Number.isFinite(px) && Number.isFinite(py)) {
            if (px < minX) minX = px;
            if (py < minY) minY = py;
            if (px > maxX) maxX = px;
            if (py > maxY) maxY = py;
          }
        }
        if (minX !== Infinity && minY !== Infinity && maxX !== -Infinity && maxY !== -Infinity) {
          return { x: centerX + minX, y: centerY + minY, w: maxX - minX, h: maxY - minY };
        }
      }
    }
    return null;
  };

  const fromTexture = () => {
    const texture = spec.texture as HTMLCanvasElement | ImageBitmap | string | undefined;
    if (!texture || typeof texture === "string") {
      return null;
    }
    const width = Number.isFinite((texture as HTMLCanvasElement).width)
      ? (texture as HTMLCanvasElement).width
      : Number.isFinite((texture as ImageBitmap).width)
      ? (texture as ImageBitmap).width
      : 0;
    const height = Number.isFinite((texture as HTMLCanvasElement).height)
      ? (texture as HTMLCanvasElement).height
      : Number.isFinite((texture as ImageBitmap).height)
      ? (texture as ImageBitmap).height
      : 0;
    return { x: centerX - width / 2, y: centerY - height / 2, w: width, h: height };
  };

  return (
    fromShape() ??
    fromTexture() ?? {
      x: centerX - 6,
      y: centerY - 6,
      w: 12,
      h: 12,
    }
  );
};

const drawDecal = (frame: EffectFrameContext, decal: ManagedDecal): void => {
  const { ctx, camera } = frame;
  const spec = decal.spec;
  const x = Number.isFinite(spec.x) ? (spec.x as number) : 0;
  const y = Number.isFinite(spec.y) ? (spec.y as number) : 0;
  const rotation = Number.isFinite(spec.rotation) ? (spec.rotation as number) : 0;
  const texture = spec.texture;
  const shape = spec.shape;
  const defaultColor =
    typeof spec.averageColor === "string" && spec.averageColor
      ? spec.averageColor
      : "rgba(127, 29, 29, 0.85)";

  ctx.save();
  ctx.translate(camera.toScreenX(x), camera.toScreenY(y));
  ctx.scale(camera.zoom, camera.zoom);
  if (rotation !== 0) {
    ctx.rotate(rotation);
  }

  const hasCanvas =
    typeof HTMLCanvasElement !== "undefined" && texture instanceof HTMLCanvasElement;
  const hasBitmap = typeof ImageBitmap !== "undefined" && texture instanceof ImageBitmap;

  if (hasCanvas || hasBitmap) {
    const width = Number.isFinite((texture as HTMLCanvasElement | ImageBitmap).width)
      ? (texture as HTMLCanvasElement | ImageBitmap).width
      : 0;
    const height = Number.isFinite((texture as HTMLCanvasElement | ImageBitmap).height)
      ? (texture as HTMLCanvasElement | ImageBitmap).height
      : 0;
    ctx.drawImage(texture as CanvasImageSource, -width / 2, -height / 2, width, height);
    ctx.restore();
    return;
  }

  if (shape && typeof shape === "object") {
    ctx.fillStyle = defaultColor;
    if (shape.type === "oval") {
      const rx = Number.isFinite(shape.rx) ? shape.rx : 0;
      const ry = Number.isFinite(shape.ry) ? shape.ry : 0;
      if (rx > 0 && ry > 0) {
        ctx.beginPath();
        ctx.ellipse(0, 0, rx, ry, 0, 0, Math.PI * 2);
        ctx.fill();
      }
    } else if (shape.type === "rect") {
      const w = Number.isFinite(shape.w) ? shape.w : 0;
      const h = Number.isFinite(shape.h) ? shape.h : 0;
      if (w > 0 && h > 0) {
        ctx.fillRect(-w / 2, -h / 2, w, h);
      }
    } else if (shape.type === "poly" && Array.isArray(shape.points)) {
      const points = shape.points;
      if (points.length >= 4 && points.length % 2 === 0) {
        ctx.beginPath();
        ctx.moveTo(points[0], points[1]);
        for (let i = 2; i < points.length; i += 2) {
          ctx.lineTo(points[i], points[i + 1]);
        }
        ctx.closePath();
        ctx.fill();
      }
    }
    ctx.restore();
    return;
  }

  if (typeof texture === "string" && texture) {
    ctx.fillStyle = texture;
    const size = 12;
    ctx.fillRect(-size / 2, -size / 2, size, size);
    ctx.restore();
    return;
  }

  ctx.fillStyle = defaultColor;
  ctx.beginPath();
  ctx.arc(0, 0, 6, 0, Math.PI * 2);
  ctx.fill();
  ctx.restore();
};

const getTimestamp = (nowSeconds?: number | null): number => {
  if (typeof nowSeconds === "number" && Number.isFinite(nowSeconds)) {
    return nowSeconds;
  }
  return Date.now() / 1000;
};

/**
 * EffectManager owns the lifecycle of all visual EffectInstance objects.
 * Hosts feed it simulation state, triggers, and frame context; the manager
 * handles spawning, culling, updating, drawing, and decal ownership so
 * callers never track effect instances themselves.
 */
export class EffectManager {
  private effects: ManagedEffect[] = [];

  private finished: EffectInstance[] = [];

  private pendingRemovals: Set<EffectInstance<any>> = new Set();

  private iterating = false;

  private creationCounter = 0;

  private viewBounds: ViewBounds | null = null;

  private stats: FrameStats = { updated: 0, drawn: 0, culled: 0 };

  private effectIndex: Map<string, ManagedEffect> = new Map();

  private decals: ManagedDecal[] = [];

  private triggerHandlers: Map<string, TriggerHandler> = new Map();

  spawn<TOptions>(
    definition: EffectDefinition<TOptions>,
    options: Partial<TOptions> & { x: number; y: number }
  ): EffectInstance<TOptions> {
    const instance = definition.create(options);
    return this.track(instance);
  }

  spawnFromPreset<TOptions>(
    definition: EffectDefinition<TOptions>,
    position: { x: number; y: number },
    preset?: EffectPreset | Partial<EffectPreset>,
    overrides?: Record<string, unknown>
  ): EffectInstance<TOptions> {
    if (definition.createFromPreset) {
      const instance = definition.createFromPreset(position, preset, overrides);
      return this.track(instance);
    }

    const presetOptions = asPresetOptions(preset);
    const instance = definition.create({
      ...presetOptions,
      ...(overrides ?? {}),
      x: position.x,
      y: position.y,
    } as Partial<TOptions> & { x: number; y: number });
    return this.track(instance);
  }

  addInstance<TOptions>(instance: EffectInstance<TOptions>): EffectInstance<TOptions> {
    return this.track(instance);
  }

  clear(): void {
    for (const entry of this.effects) {
      entry.instance.dispose?.();
    }
    for (const finished of this.finished) {
      finished.dispose?.();
    }
    this.effects = [];
    this.finished = [];
    this.creationCounter = 0;
    this.viewBounds = null;
    this.stats = { updated: 0, drawn: 0, culled: 0 };
    this.effectIndex.clear();
    this.decals = [];
  }

  cullByAABB(view: ViewBounds): void {
    this.viewBounds = view;
    for (const managed of this.effects) {
      const bounds = managed.instance.getAABB();
      managed.culled = !intersects(bounds, view);
    }
    for (const decal of this.decals) {
      decal.culled = !intersects(decal.aabb, view);
    }
  }

  updateAll(frame: EffectFrameContext): void {
    this.stats.updated = 0;
    this.stats.culled = 0;
    this.iterating = true;

    for (let i = 0; i < this.effects.length; ) {
      const managed = this.effects[i];

      if (this.pendingRemovals.has(managed.instance)) {
        this.pendingRemovals.delete(managed.instance);
        managed.instance.dispose?.();
        this.effects.splice(i, 1);
        continue;
      }

      if (managed.culled) {
        this.stats.culled += 1;
        i += 1;
        continue;
      }

      managed.instance.update(frame);
      this.stats.updated += 1;

      if (!managed.instance.isAlive()) {
        this.finished.push(managed.instance);
        this.pendingRemovals.delete(managed.instance);
        if (managed.instance.id) {
          this.effectIndex.delete(managed.instance.id);
        }
        this.effects.splice(i, 1);
        continue;
      }

      managed.layer = managed.instance.layer;
      managed.sublayer = managed.instance.sublayer ?? 0;
      i += 1;
    }

    this.iterating = false;

    if (this.pendingRemovals.size > 0) {
      for (const instance of this.pendingRemovals) {
        if (this.removeActiveInstance(instance)) {
          instance.dispose?.();
        }
      }
      this.pendingRemovals.clear();
    }
  }

  drawAll(frame: EffectFrameContext): void {
    this.stats.drawn = 0;
    const view = this.viewBounds;
    const nowSeconds = frame.now ?? Date.now() / 1000;
    this.decals = this.decals.filter((decal) => {
      if (nowSeconds >= decal.expiresAt) {
        return false;
      }
      if (view && !intersects(decal.aabb, view)) {
        decal.culled = true;
      }
      return true;
    });

    const sorted = [...this.effects, ...this.decals].sort((a, b) => {
      const layerA = "instance" in a ? a.layer : a.layer;
      const layerB = "instance" in b ? b.layer : b.layer;
      if (layerA !== layerB) {
        return layerA - layerB;
      }
      const sublayerA = "instance" in a ? a.sublayer : a.sublayer;
      const sublayerB = "instance" in b ? b.sublayer : b.sublayer;
      if (sublayerA !== sublayerB) {
        return sublayerA - sublayerB;
      }
      return a.creationIndex - b.creationIndex;
    });

    for (const managed of sorted) {
      if ("instance" in managed) {
        if (managed.culled) {
          this.stats.culled += 1;
          continue;
        }
        if (view && !intersects(managed.instance.getAABB(), view)) {
          managed.culled = true;
          this.stats.culled += 1;
          continue;
        }
        managed.instance.draw(frame);
        this.stats.drawn += 1;
        continue;
      }

      if (managed.culled && view) {
        this.stats.culled += 1;
        continue;
      }
      drawDecal(frame, managed);
    }
  }

  collectDecals(nowSeconds?: number | null): DecalSpec[] {
    if (this.finished.length === 0) {
      return [];
    }
    const decals: DecalSpec[] = [];
    const timestamp = getTimestamp(nowSeconds);
    for (const instance of this.finished) {
      const decal = instance.handoffToDecal?.() ?? null;
      if (decal) {
        decals.push(decal);
        this.enqueueDecal(decal, timestamp);
      }
      instance.dispose?.();
      if (instance.id) {
        this.effectIndex.delete(instance.id);
      }
    }
    this.finished = [];
    return decals;
  }

  getLastFrameStats(): FrameStats {
    return { ...this.stats };
  }

  removeInstance<TOptions>(instance: EffectInstance<TOptions> | null | undefined): boolean {
    if (!instance) {
      return false;
    }

    let disposed = false;

    const finishedIndex = this.finished.indexOf(instance);
    if (finishedIndex !== -1) {
      this.finished.splice(finishedIndex, 1);
      instance.dispose?.();
      disposed = true;
    }

    let foundActive = false;

    if (this.iterating) {
      for (const managed of this.effects) {
        if (managed.instance === instance) {
          this.pendingRemovals.add(instance);
          foundActive = true;
          break;
        }
      }
    } else {
      foundActive = this.removeActiveInstance(instance);
      if (foundActive && !disposed) {
        instance.dispose?.();
        disposed = true;
      }
    }

    if ((disposed || foundActive) && instance?.id) {
      this.effectIndex.delete(instance.id);
    }

    return disposed || foundActive;
  }

  getInstanceById<TOptions>(id: string): EffectInstance<TOptions> | null {
    const managed = this.effectIndex.get(id);
    return managed?.instance ?? null;
  }

  getInstancesByType(type: string): EffectInstance[] {
    return this.effects
      .filter((entry) => entry.instance.type === type)
      .map((entry) => entry.instance);
  }

  registerTrigger(type: string, handler: TriggerHandler): void {
    if (typeof type !== "string" || type.length === 0) {
      return;
    }
    if (typeof handler !== "function") {
      return;
    }
    this.triggerHandlers.set(type, handler);
  }

  trigger(type: string, trigger: TriggerPayload, context?: Record<string, unknown> | null): void {
    if (typeof type !== "string" || type.length === 0) {
      return;
    }
    const handler = this.triggerHandlers.get(type);
    if (!handler) {
      return;
    }
    try {
      handler({ manager: this, trigger, context: context ?? null });
    } catch (err) {
      console.error("effect trigger handler failed", err);
    }
  }

  triggerAll(
    triggers: Array<Record<string, unknown>> | null | undefined,
    context?: Record<string, unknown> | null
  ): void {
    if (!Array.isArray(triggers) || triggers.length === 0) {
      return;
    }
    for (const trigger of triggers) {
      if (!trigger || typeof trigger !== "object") {
        continue;
      }
      const type = typeof trigger.type === "string" ? trigger.type : "";
      if (!type) {
        continue;
      }
      this.trigger(type, trigger, context);
    }
  }

  private track<TOptions>(
    instance: EffectInstance<TOptions>
  ): EffectInstance<TOptions> {
    const managed: ManagedEffect = {
      instance,
      layer: instance.layer,
      sublayer: instance.sublayer ?? 0,
      creationIndex: this.creationCounter++,
      culled: false,
    };
    this.effects.push(managed);
    if (typeof instance.id === "string" && instance.id.length > 0) {
      this.effectIndex.set(instance.id, managed);
    }
    return instance;
  }

  private removeActiveInstance(instance: EffectInstance<any>): boolean {
    let removed = false;
    for (let index = this.effects.length - 1; index >= 0; index -= 1) {
      if (this.effects[index].instance === instance) {
        this.effects.splice(index, 1);
        removed = true;
      }
    }
    return removed;
  }

  private enqueueDecal(spec: DecalSpec, timestamp: number): void {
    const ttl =
      typeof spec.ttl === "number" && Number.isFinite(spec.ttl) && spec.ttl >= 0
        ? spec.ttl
        : null;
    const layer = resolveLayerFromHint(spec.layerHint);
    const managed: ManagedDecal = {
      spec,
      layer,
      sublayer: 0,
      creationIndex: this.creationCounter++,
      culled: false,
      expiresAt: ttl === null ? Number.POSITIVE_INFINITY : timestamp + ttl,
      aabb: buildDecalAABB(spec),
    };
    this.decals.push(managed);
  }
}
