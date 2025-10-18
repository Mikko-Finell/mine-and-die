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
    this.applyWorldDimensionsFromBatch(batch);
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

  private applyWorldDimensionsFromBatch(batch: RenderBatch | null): void {
    if (!batch) {
      return;
    }

    const dimensions = this.extractWorldDimensions(batch.staticGeometry);
    if (!dimensions) {
      return;
    }

    const widthChanged = dimensions.width !== this.currentDimensions.width;
    const heightChanged = dimensions.height !== this.currentDimensions.height;
    if (!widthChanged && !heightChanged) {
      return;
    }

    this.currentDimensions = { ...dimensions };
    this.configureCanvas();
  }

  private extractWorldDimensions(geometry: readonly StaticGeometry[]): RenderDimensions | null {
    for (const entry of geometry) {
      const dimensions = this.extractDimensionsFromEntry(entry);
      if (dimensions) {
        return dimensions;
      }
    }
    return null;
  }

  private extractDimensionsFromEntry(entry: StaticGeometry): RenderDimensions | null {
    const style = this.asRecord(entry.style);
    const kind = typeof style.kind === "string" ? style.kind : null;
    if (kind !== "world-background" && kind !== "world-grid") {
      return null;
    }

    const bounds = this.computeBounds(entry.vertices);
    if (!bounds) {
      return null;
    }

    const width = this.resolveDimension(style.width, bounds.maxX - bounds.minX);
    const height = this.resolveDimension(style.height, bounds.maxY - bounds.minY);
    if (width <= 0 || height <= 0) {
      return null;
    }

    return { width, height };
  }

  private drawStaticGeometry(
    context: CanvasRenderingContext2D,
    geometry: readonly StaticGeometry[],
  ): void {
    if (!Array.isArray(geometry) || geometry.length === 0) {
      return;
    }

    const ranked = geometry
      .map((entry, index) => ({
        entry,
        index,
        zIndex: this.resolveLayerZIndex(entry.layer),
      }))
      .sort((left, right) => {
        const diff = left.zIndex - right.zIndex;
        if (diff !== 0) {
          return diff;
        }
        return left.index - right.index;
      });

    for (const { entry } of ranked) {
      this.drawStaticGeometryEntry(context, entry);
    }
  }

  private drawStaticGeometryEntry(
    context: CanvasRenderingContext2D,
    entry: StaticGeometry,
  ): void {
    const style = this.asRecord(entry.style);
    const kind = typeof style.kind === "string" ? style.kind : null;

    switch (kind) {
      case "world-background":
        this.drawWorldBackground(context, entry, style);
        return;
      case "world-grid":
        this.drawWorldGrid(context, entry, style);
        return;
      case "player":
      case "npc":
        this.drawCircleGeometry(context, entry, style);
        return;
      default:
        this.drawPolygonGeometry(context, entry, style);
        return;
    }
  }

  private drawWorldBackground(
    context: CanvasRenderingContext2D,
    entry: StaticGeometry,
    style: Record<string, unknown>,
  ): void {
    const bounds = this.computeBounds(entry.vertices);
    if (!bounds) {
      return;
    }

    const origin = this.resolvePoint(style.origin) ?? [bounds.minX, bounds.minY];
    const width = this.resolveDimension(style.width, bounds.maxX - bounds.minX);
    const height = this.resolveDimension(style.height, bounds.maxY - bounds.minY);
    if (width <= 0 || height <= 0) {
      return;
    }

    const fill = this.resolveFill(style);
    const stroke = this.resolveStroke(style);
    const lineWidth = this.resolveLineWidth(style, 1);

    context.save();
    if (fill) {
      context.fillStyle = fill;
      context.fillRect(origin[0], origin[1], width, height);
    }
    if (stroke && lineWidth > 0) {
      context.lineWidth = lineWidth;
      context.strokeStyle = stroke;
      context.strokeRect(origin[0], origin[1], width, height);
    }
    context.restore();
  }

  private drawWorldGrid(
    context: CanvasRenderingContext2D,
    entry: StaticGeometry,
    style: Record<string, unknown>,
  ): void {
    const bounds = this.computeBounds(entry.vertices);
    if (!bounds) {
      return;
    }

    const origin = this.resolvePoint(style.origin) ?? [bounds.minX, bounds.minY];
    const width = this.resolveDimension(style.width, bounds.maxX - bounds.minX);
    const height = this.resolveDimension(style.height, bounds.maxY - bounds.minY);
    if (width <= 0 || height <= 0) {
      return;
    }

    const fill = this.resolveFill(style);
    const stroke = this.resolveStroke(style) ?? "rgba(255, 255, 255, 0.06)";
    const lineWidth = this.resolveLineWidth(style, 1);
    const columns = this.resolveCount(style.columns, Math.max(1, Math.round(width)));
    const rows = this.resolveCount(style.rows, Math.max(1, Math.round(height)));

    context.save();
    if (fill) {
      context.fillStyle = fill;
      context.fillRect(origin[0], origin[1], width, height);
    }
    if (stroke && lineWidth > 0) {
      context.strokeStyle = stroke;
      context.lineWidth = lineWidth;
      context.beginPath();

      const drawVerticalLines = Math.max(1, columns);
      for (let column = 0; column <= drawVerticalLines; column += 1) {
        const x = origin[0] + (width * column) / drawVerticalLines;
        context.moveTo(x, origin[1]);
        context.lineTo(x, origin[1] + height);
      }

      const drawHorizontalLines = Math.max(1, rows);
      for (let row = 0; row <= drawHorizontalLines; row += 1) {
        const y = origin[1] + (height * row) / drawHorizontalLines;
        context.moveTo(origin[0], y);
        context.lineTo(origin[0] + width, y);
      }

      context.stroke();
    }
    context.restore();
  }

  private drawCircleGeometry(
    context: CanvasRenderingContext2D,
    entry: StaticGeometry,
    style: Record<string, unknown>,
  ): void {
    const bounds = this.computeBounds(entry.vertices);
    if (!bounds) {
      return;
    }

    const centerX = (bounds.minX + bounds.maxX) / 2;
    const centerY = (bounds.minY + bounds.maxY) / 2;
    const radius = this.resolveDimension(style.radius, Math.min(bounds.maxX - bounds.minX, bounds.maxY - bounds.minY) / 2);
    if (radius <= 0) {
      return;
    }

    const fill = this.resolveFill(style);
    const stroke = this.resolveStroke(style);
    const lineWidth = this.resolveLineWidth(style, 1);

    context.save();
    context.beginPath();
    context.arc(centerX, centerY, radius, 0, Math.PI * 2);
    if (fill) {
      context.fillStyle = fill;
      context.fill();
    }
    if (stroke && lineWidth > 0) {
      context.lineWidth = lineWidth;
      context.strokeStyle = stroke;
      context.stroke();
    }
    context.restore();
  }

  private drawPolygonGeometry(
    context: CanvasRenderingContext2D,
    entry: StaticGeometry,
    style: Record<string, unknown>,
  ): void {
    if (!Array.isArray(entry.vertices) || entry.vertices.length === 0) {
      return;
    }

    const fill = this.resolveFill(style);
    const stroke = this.resolveStroke(style);
    const lineWidth = this.resolveLineWidth(style, 1);
    if (!fill && (!stroke || lineWidth <= 0)) {
      return;
    }

    context.save();
    context.beginPath();
    const [firstX, firstY] = entry.vertices[0];
    context.moveTo(firstX, firstY);
    for (let index = 1; index < entry.vertices.length; index += 1) {
      const [x, y] = entry.vertices[index];
      context.lineTo(x, y);
    }
    context.closePath();
    if (fill) {
      context.fillStyle = fill;
      context.fill();
    }
    if (stroke && lineWidth > 0) {
      context.lineWidth = lineWidth;
      context.strokeStyle = stroke;
      context.stroke();
    }
    context.restore();
  }

  private resolveLayerZIndex(layer: RenderLayer | undefined): number {
    if (layer && Number.isFinite(layer.zIndex)) {
      return layer.zIndex;
    }
    if (!layer) {
      return 0;
    }
    const match = this.layers.find((candidate) => candidate.id === layer.id);
    if (match && Number.isFinite(match.zIndex)) {
      return match.zIndex;
    }
    return 0;
  }

  private asRecord(value: unknown): Record<string, unknown> {
    if (!value || typeof value !== "object") {
      return {};
    }
    return value as Record<string, unknown>;
  }

  private computeBounds(
    vertices: readonly [number, number][],
  ): { minX: number; minY: number; maxX: number; maxY: number } | null {
    if (!Array.isArray(vertices) || vertices.length === 0) {
      return null;
    }
    let minX = Number.POSITIVE_INFINITY;
    let maxX = Number.NEGATIVE_INFINITY;
    let minY = Number.POSITIVE_INFINITY;
    let maxY = Number.NEGATIVE_INFINITY;
    for (const [x, y] of vertices) {
      if (!Number.isFinite(x) || !Number.isFinite(y)) {
        continue;
      }
      if (x < minX) {
        minX = x;
      }
      if (x > maxX) {
        maxX = x;
      }
      if (y < minY) {
        minY = y;
      }
      if (y > maxY) {
        maxY = y;
      }
    }
    if (!Number.isFinite(minX) || !Number.isFinite(maxX) || !Number.isFinite(minY) || !Number.isFinite(maxY)) {
      return null;
    }
    return { minX, minY, maxX, maxY };
  }

  private resolveDimension(value: unknown, fallback: number): number {
    if (typeof value === "number" && Number.isFinite(value) && value > 0) {
      return value;
    }
    return fallback;
  }

  private resolveLineWidth(style: Record<string, unknown>, fallback: number): number {
    const candidate = style.lineWidth;
    if (typeof candidate === "number" && Number.isFinite(candidate) && candidate >= 0) {
      return candidate;
    }
    return fallback;
  }

  private resolveFill(style: Record<string, unknown>): string | null {
    const { fill } = style;
    return typeof fill === "string" && fill.length > 0 ? fill : null;
  }

  private resolveStroke(style: Record<string, unknown>): string | null {
    const { stroke } = style;
    return typeof stroke === "string" && stroke.length > 0 ? stroke : null;
  }

  private resolvePoint(value: unknown): [number, number] | null {
    if (!Array.isArray(value) || value.length < 2) {
      return null;
    }
    const [x, y] = value;
    if (typeof x !== "number" || !Number.isFinite(x) || typeof y !== "number" || !Number.isFinite(y)) {
      return null;
    }
    return [x, y];
  }

  private resolveCount(value: unknown, fallback: number): number {
    if (typeof value === "number" && Number.isFinite(value) && value > 0) {
      return Math.floor(value);
    }
    return Math.max(1, Math.floor(fallback));
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

