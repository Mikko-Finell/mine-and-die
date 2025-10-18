import { EffectManager, type EffectInstance as RuntimeEffectInstance } from "@js-effects/effects-lib";
import { translateRenderAnimation, type EffectSpawnIntent } from "./effect-runtime-adapter";
import type { PathTarget } from "./input";

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
}

export interface RenderContextProvider {
  readonly canvas: HTMLCanvasElement;
  readonly context: CanvasRenderingContext2D;
}

export interface RendererConfiguration {
  readonly dimensions: RenderDimensions;
  readonly layers: readonly RenderLayer[];
}

export interface Renderer {
  readonly configuration: RendererConfiguration;
  readonly mount: (provider: RenderContextProvider) => void;
  readonly unmount: () => void;
  readonly renderBatch: (batch: RenderBatch) => void;
  readonly resize: (dimensions: RenderDimensions) => void;
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
    this.clearEffects();
    this.effectManager.clear();
    this.provider = null;
    this.lastBatch = null;
  }

  renderBatch(batch: RenderBatch): void {
    this.lastBatch = batch;
    this.syncEffects(batch);
  }

  resize(dimensions: RenderDimensions): void {
    this.currentDimensions = { ...dimensions };
    this.configureCanvas();
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

    const batch = this.lastBatch;
    if (batch?.pathTarget) {
      this.drawPathTarget(context, batch.pathTarget);
    }
  }

  private syncEffects(batch: RenderBatch | null): void {
    if (!batch) {
      this.clearEffects();
      return;
    }

    const seen = new Set<string>();

    for (const animation of batch.animations) {
      const intent = translateRenderAnimation(animation);
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

