import {
  EffectLayer,
  EffectManager,
  type EffectInstance as RuntimeEffectInstance,
} from "@js-effects/effects-lib";
import { translateRenderAnimation, type EffectSpawnIntent } from "./effect-runtime-adapter";
import type { PathTarget } from "./input";
import type { DeliveryKind } from "./generated/effect-contracts";

export interface RenderDimensions {
  readonly width: number;
  readonly height: number;
}

export interface RenderLayer {
  readonly id: string;
  readonly zIndex: number;
}

export interface AnimationFrame {
  readonly effectId: string;
  readonly startedAt: number;
  readonly durationMs: number;
  readonly metadata: Record<string, unknown>;
}

export interface StaticGeometry {
  readonly id: string;
  readonly layer: RenderLayer;
  readonly vertices: readonly [number, number][];
  readonly style: Record<string, unknown>;
}

export interface RenderBatch {
  readonly keyframeId: string;
  readonly time: number;
  readonly staticGeometry: readonly StaticGeometry[];
  readonly animations: readonly AnimationFrame[];
  readonly pathTarget: PathTarget | null;
  readonly runtimeEffects?: readonly RuntimeEffectFrame[];
}

export interface RuntimeEffectFrame {
  readonly effectId: string;
  readonly intent: EffectSpawnIntent | null;
}

export interface RenderContextProvider {
  readonly canvas: HTMLCanvasElement;
  readonly context: CanvasRenderingContext2D;
}

export interface RendererConfiguration {
  readonly dimensions: RenderDimensions;
  readonly layers: readonly RenderLayer[];
}

const deliveryKinds: readonly DeliveryKind[] = ["visual", "area", "target"];

const deliveryLayerCandidates: Record<DeliveryKind, readonly string[]> = {
  area: ["area", "effect-area", "effects-area"],
  target: ["target", "effect-target", "effects-target"],
  visual: ["visual", "effect-visual", "effects-visual"],
} as const;

const runtimeLayerRank: Record<DeliveryKind, number> = {
  area: EffectLayer.ActorOverlay,
  target: EffectLayer.ActorOverlay,
  visual: EffectLayer.GroundDecal,
};

const isFiniteNumber = (value: unknown): value is number =>
  typeof value === "number" && Number.isFinite(value);

export const validateRenderLayers = (layers: readonly RenderLayer[]): void => {
  if (!Array.isArray(layers)) {
    throw new Error("Renderer layers must be provided as an array.");
  }

  const groundLayer = EffectLayer.GroundDecal;
  const actorLayer = EffectLayer.ActorOverlay;
  if (!isFiniteNumber(groundLayer) || !isFiniteNumber(actorLayer)) {
    throw new Error("Effect runtime layers are unavailable; rebuild @js-effects/effects-lib.");
  }
  if (groundLayer >= actorLayer) {
    throw new Error("Effect runtime layer ordering changed; GroundDecal must sort before ActorOverlay.");
  }

  const resolved = deliveryKinds.map((delivery) => {
    const layer = layers.find((candidate) => deliveryLayerCandidates[delivery].includes(candidate.id));
    if (!layer) {
      throw new Error(`Renderer configuration missing layer for ${delivery} delivery effects.`);
    }
    if (!isFiniteNumber(layer.zIndex)) {
      throw new Error(`Renderer layer ${layer.id} must declare a finite zIndex.`);
    }
    return { delivery, layer };
  });

  const rankFor = (delivery: DeliveryKind): number => deliveryKinds.indexOf(delivery);
  const runtimeSorted = [...resolved].sort((left, right) => {
    const diff = runtimeLayerRank[left.delivery] - runtimeLayerRank[right.delivery];
    if (diff !== 0) {
      return diff;
    }
    return rankFor(left.delivery) - rankFor(right.delivery);
  });
  const renderSorted = [...resolved].sort((left, right) => {
    const diff = left.layer.zIndex - right.layer.zIndex;
    if (diff !== 0) {
      return diff;
    }
    return rankFor(left.delivery) - rankFor(right.delivery);
  });

  for (let index = 0; index < runtimeSorted.length; index += 1) {
    const expected = runtimeSorted[index];
    const actual = renderSorted[index];
    if (expected.delivery !== actual.delivery) {
      throw new Error(
        `Renderer layer ordering for ${actual.delivery} conflicts with runtime layer ordering; ` +
          `expected ${expected.delivery} effects to render earlier.`,
      );
    }
  }
};

export interface Renderer {
  readonly configuration: RendererConfiguration;
  readonly mount: (provider: RenderContextProvider) => void;
  readonly unmount: () => void;
  readonly renderBatch: (batch: RenderBatch) => void;
  readonly resize: (dimensions: RenderDimensions) => void;
  readonly reset: () => void;
}

interface ActiveEffectState {
  instance: RuntimeEffectInstance;
  signature: string;
  retained: boolean;
}

const MAX_FRAME_DELTA_SECONDS = 0.25;

const identityCamera = {
  toScreenX: (x: number): number => x,
  toScreenY: (y: number): number => y,
  zoom: 1,
};

export class CanvasRenderer implements Renderer {
  private readonly layers: RenderLayer[];
  private readonly effectManager = new EffectManager();
  private readonly activeEffects = new Map<string, ActiveEffectState>();
  private currentDimensions: RenderDimensions;
  private provider: RenderContextProvider | null = null;
  private lastBatch: RenderBatch | null = null;
  private frameHandle: number | null = null;
  private lastFrameTime: number | null = null;

  constructor(configuration: RendererConfiguration) {
    this.currentDimensions = { ...configuration.dimensions };
    this.layers = configuration.layers.map((layer) => ({ ...layer }));
    validateRenderLayers(this.layers);
  }

  get configuration(): RendererConfiguration {
    return {
      dimensions: this.currentDimensions,
      layers: this.layers,
    };
  }

  mount(provider: RenderContextProvider): void {
    this.provider = provider;
    this.syncDimensionsFromBatch(this.lastBatch);
    this.configureCanvas();
    this.syncEffects(this.lastBatch);
    this.startLoop();
  }

  unmount(): void {
    this.stopLoop();
    this.reset();
    this.provider = null;
  }

  renderBatch(batch: RenderBatch): void {
    this.lastBatch = batch;
    this.syncDimensionsFromBatch(batch);
    this.syncEffects(batch);
  }

  resize(dimensions: RenderDimensions): void {
    this.currentDimensions = { ...dimensions };
    this.configureCanvas();
  }

  reset(): void {
    this.clearEffects();
    this.effectManager.clear();
    this.lastBatch = null;
  }

  private configureCanvas(): void {
    const provider = this.provider;
    if (!provider) {
      return;
    }
    provider.canvas.width = Math.max(0, Math.floor(this.currentDimensions.width));
    provider.canvas.height = Math.max(0, Math.floor(this.currentDimensions.height));
  }

  private startLoop(): void {
    if (this.frameHandle !== null) {
      return;
    }
    const raf = typeof window !== "undefined" && typeof window.requestAnimationFrame === "function"
      ? window.requestAnimationFrame.bind(window)
      : null;
    if (!raf) {
      return;
    }
    this.lastFrameTime = null;
    const step = (timestamp: number): void => {
      this.stepFrame(timestamp);
      this.frameHandle = raf(step);
    };
    this.frameHandle = raf(step);
  }

  private stopLoop(): void {
    if (this.frameHandle === null) {
      return;
    }
    const caf = typeof window !== "undefined" && typeof window.cancelAnimationFrame === "function"
      ? window.cancelAnimationFrame.bind(window)
      : null;
    if (caf) {
      caf(this.frameHandle);
    }
    this.frameHandle = null;
    this.lastFrameTime = null;
  }

  private stepFrame(timestamp: number): void {
    const provider = this.provider;
    if (!provider) {
      return;
    }

    const { context, canvas } = provider;
    if (this.lastFrameTime === null) {
      this.lastFrameTime = timestamp;
    }

    const deltaMs = Math.max(0, timestamp - this.lastFrameTime);
    this.lastFrameTime = timestamp;
    const deltaSeconds = Math.min(deltaMs / 1000, MAX_FRAME_DELTA_SECONDS);

    context.save();
    context.clearRect(0, 0, canvas.width, canvas.height);

    const batch = this.lastBatch;
    if (batch) {
      this.drawStaticGeometry(context, batch.staticGeometry);
    }

    this.effectManager.cullByAABB({ x: 0, y: 0, w: canvas.width, h: canvas.height });
    const frameContext = {
      ctx: context,
      dt: deltaSeconds,
      now: timestamp / 1000,
      camera: identityCamera,
    } as const;
    this.effectManager.updateAll(frameContext);
    this.effectManager.drawAll(frameContext);
    context.restore();

    if (batch?.pathTarget) {
      this.drawPathTarget(context, batch.pathTarget);
    }
  }

  private syncDimensionsFromBatch(batch: RenderBatch | null): void {
    if (!batch) {
      return;
    }
    const dimensions = this.extractDimensionsFromGeometry(batch.staticGeometry);
    if (!dimensions) {
      return;
    }
    if (
      dimensions.width === this.currentDimensions.width &&
      dimensions.height === this.currentDimensions.height
    ) {
      return;
    }
    this.currentDimensions = dimensions;
    this.configureCanvas();
  }

  private extractDimensionsFromGeometry(geometry: readonly StaticGeometry[]): RenderDimensions | null {
    for (const entry of geometry) {
      const style = entry.style;
      if (!style || typeof style !== "object") {
        continue;
      }
      const record = style as Record<string, unknown>;
      if (record.kind !== "world-background") {
        continue;
      }
      const width = record.width;
      const height = record.height;
      if (!isFiniteNumber(width) || width <= 0 || !isFiniteNumber(height) || height <= 0) {
        continue;
      }
      return { width, height };
    }
    return null;
  }

  private drawStaticGeometry(
    context: CanvasRenderingContext2D,
    geometry: readonly StaticGeometry[],
  ): void {
    if (!Array.isArray(geometry) || geometry.length === 0) {
      return;
    }

    const sorted = geometry
      .map((entry, index) => ({ entry, index }))
      .sort((left, right) => {
        const leftZ = isFiniteNumber(left.entry.layer?.zIndex) ? left.entry.layer.zIndex : 0;
        const rightZ = isFiniteNumber(right.entry.layer?.zIndex) ? right.entry.layer.zIndex : 0;
        if (leftZ !== rightZ) {
          return leftZ - rightZ;
        }
        return left.index - right.index;
      });

    for (const { entry } of sorted) {
      const style = entry.style;
      const record = style && typeof style === "object" ? (style as Record<string, unknown>) : null;
      if (record?.kind === "world-grid") {
        this.drawWorldGrid(context, entry, record);
        continue;
      }
      this.drawPolygon(context, entry, record);
    }
  }

  private drawWorldGrid(
    context: CanvasRenderingContext2D,
    entry: StaticGeometry,
    style: Record<string, unknown>,
  ): void {
    if (entry.vertices.length < 2) {
      return;
    }
    const xs = entry.vertices.map(([x]) => x);
    const ys = entry.vertices.map(([, y]) => y);
    const minX = Math.min(...xs);
    const maxX = Math.max(...xs);
    const minY = Math.min(...ys);
    const maxY = Math.max(...ys);
    if (!Number.isFinite(minX) || !Number.isFinite(maxX) || !Number.isFinite(minY) || !Number.isFinite(maxY)) {
      return;
    }

    const rawColumns = style.columns;
    const rawRows = style.rows;
    const rawSpacing = style.spacing;
    const columns = isFiniteNumber(rawColumns) ? Math.max(0, Math.floor(rawColumns)) : 0;
    const rows = isFiniteNumber(rawRows) ? Math.max(0, Math.floor(rawRows)) : 0;
    const spacing = isFiniteNumber(rawSpacing) && rawSpacing > 0 ? rawSpacing : null;
    const width = maxX - minX;
    const height = maxY - minY;
    const stepX = columns > 0 ? width / columns : spacing ?? width;
    const stepY = rows > 0 ? height / rows : spacing ?? height;
    if (!Number.isFinite(stepX) || !Number.isFinite(stepY) || stepX <= 0 || stepY <= 0) {
      return;
    }

    const stroke = typeof style.stroke === "string" ? style.stroke : "rgba(255, 255, 255, 0.08)";
    const rawLineWidth = style.lineWidth;
    const lineWidth = isFiniteNumber(rawLineWidth) ? Math.max(0.5, rawLineWidth) : 1;

    context.save();
    context.beginPath();
    for (let column = 0; column <= Math.ceil(width / stepX); column += 1) {
      const x = minX + column * stepX;
      if (x < minX || x > maxX) {
        continue;
      }
      context.moveTo(x, minY);
      context.lineTo(x, maxY);
    }
    for (let row = 0; row <= Math.ceil(height / stepY); row += 1) {
      const y = minY + row * stepY;
      if (y < minY || y > maxY) {
        continue;
      }
      context.moveTo(minX, y);
      context.lineTo(maxX, y);
    }
    context.strokeStyle = stroke;
    context.lineWidth = lineWidth;
    context.stroke();
    context.restore();
  }

  private drawPolygon(
    context: CanvasRenderingContext2D,
    entry: StaticGeometry,
    style: Record<string, unknown> | null,
  ): void {
    const vertices = entry.vertices;
    if (!Array.isArray(vertices) || vertices.length === 0) {
      return;
    }

    const [firstX, firstY] = vertices[0];
    if (!isFiniteNumber(firstX) || !isFiniteNumber(firstY)) {
      return;
    }

    const fallback = this.resolveFallbackStyle(style);
    const fill = typeof style?.fill === "string" ? style.fill : fallback.fill;
    const stroke = typeof style?.stroke === "string" ? style.stroke : fallback.stroke;
    const rawLineWidth = style?.lineWidth;
    const lineWidth = isFiniteNumber(rawLineWidth) ? rawLineWidth : fallback.lineWidth ?? 1;

    context.save();
    context.beginPath();
    context.moveTo(firstX, firstY);
    for (let index = 1; index < vertices.length; index += 1) {
      const [x, y] = vertices[index];
      if (!isFiniteNumber(x) || !isFiniteNumber(y)) {
        continue;
      }
      context.lineTo(x, y);
    }
    context.closePath();

    if (fill) {
      context.fillStyle = fill;
      context.fill();
    }
    if (stroke) {
      context.strokeStyle = stroke;
      context.lineWidth = lineWidth > 0 ? lineWidth : 1;
      context.stroke();
    }
    context.restore();
  }

  private resolveFallbackStyle(
    style: Record<string, unknown> | null,
  ): { fill?: string; stroke?: string; lineWidth?: number } {
    if (!style) {
      return {};
    }
    if (typeof style.kind === "string" && style.kind.startsWith("world-")) {
      return {};
    }
    const managed = typeof style.managedByClient === "boolean" ? style.managedByClient : false;
    if (managed) {
      return {
        fill: "rgba(96, 204, 255, 0.2)",
        stroke: "rgba(96, 204, 255, 0.6)",
        lineWidth: 1.5,
      };
    }
    return {
      fill: "rgba(255, 196, 72, 0.18)",
      stroke: "rgba(255, 196, 72, 0.6)",
      lineWidth: 1.5,
    };
  }

  private syncEffects(batch: RenderBatch | null): void {
    if (!batch) {
      this.clearEffects();
      return;
    }

    const runtimeOverrides = batch.runtimeEffects
      ? new Map(batch.runtimeEffects.map((entry) => [entry.effectId, entry.intent]))
      : null;
    const seen = new Set<string>();

    for (const animation of batch.animations) {
      let intent: EffectSpawnIntent | null;
      if (runtimeOverrides && runtimeOverrides.has(animation.effectId)) {
        intent = runtimeOverrides.get(animation.effectId) ?? null;
      } else {
        intent = translateRenderAnimation(animation);
      }
      if (!intent) {
        this.removeEffect(animation.effectId);
        continue;
      }

      if (intent.state === "ended" && !intent.retained) {
        this.removeEffect(intent.effectId);
        continue;
      }

      seen.add(intent.effectId);
      const existing = this.activeEffects.get(intent.effectId);

      if (!existing) {
        if (intent.state !== "ended" || intent.retained) {
          this.spawnEffect(intent);
        }
        continue;
      }

      existing.retained = intent.retained;
      if (existing.signature !== intent.signature) {
        this.removeEffect(intent.effectId);
        if (intent.state !== "ended" || intent.retained) {
          this.spawnEffect(intent);
        }
      }
    }

    for (const [effectId, state] of Array.from(this.activeEffects.entries())) {
      if (!seen.has(effectId) && !state.retained) {
        this.removeEffect(effectId);
      }
    }
  }

  private spawnEffect(intent: EffectSpawnIntent): void {
    const options = { ...intent.options };
    const instance = this.effectManager.spawn(intent.definition, options);
    this.activeEffects.set(intent.effectId, {
      instance,
      signature: intent.signature,
      retained: intent.retained,
    });
  }

  private removeEffect(effectId: string): void {
    const existing = this.activeEffects.get(effectId);
    if (!existing) {
      return;
    }
    this.effectManager.removeInstance(existing.instance);
    this.activeEffects.delete(effectId);
  }

  private clearEffects(): void {
    for (const [effectId] of this.activeEffects) {
      this.removeEffect(effectId);
    }
    this.activeEffects.clear();
  }

  private drawPathTarget(context: CanvasRenderingContext2D, target: PathTarget): void {
    context.save();
    const radius = 10;
    const stroke = "rgba(64, 192, 255, 0.9)";
    const fill = "rgba(64, 192, 255, 0.15)";
    context.lineWidth = 2;
    context.strokeStyle = stroke;
    context.fillStyle = fill;
    context.beginPath();
    context.arc(target.x, target.y, radius, 0, Math.PI * 2);
    context.fill();
    context.stroke();
    context.beginPath();
    context.moveTo(target.x - radius, target.y);
    context.lineTo(target.x + radius, target.y);
    context.moveTo(target.x, target.y - radius);
    context.lineTo(target.x, target.y + radius);
    context.stroke();
    context.restore();
  }
}

