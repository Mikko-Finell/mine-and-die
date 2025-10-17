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

export class CanvasRenderer implements Renderer {
  private readonly layers: RenderLayer[];
  private currentDimensions: RenderDimensions;
  private provider: RenderContextProvider | null = null;
  private lastBatch: RenderBatch | null = null;

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
    this.redraw();
  }

  unmount(): void {
    this.provider = null;
    this.lastBatch = null;
  }

  renderBatch(batch: RenderBatch): void {
    this.lastBatch = batch;
    this.redraw();
  }

  resize(dimensions: RenderDimensions): void {
    this.currentDimensions = { ...dimensions };
    this.configureCanvas();
    this.redraw();
  }

  private configureCanvas(): void {
    const provider = this.provider;
    if (!provider) {
      return;
    }
    provider.canvas.width = Math.max(0, Math.floor(this.currentDimensions.width));
    provider.canvas.height = Math.max(0, Math.floor(this.currentDimensions.height));
  }

  private redraw(): void {
    const provider = this.provider;
    const batch = this.lastBatch;
    if (!provider || !batch) {
      return;
    }
    const { context } = provider;
    context.save();
    context.clearRect(0, 0, provider.canvas.width, provider.canvas.height);

    for (const geometry of batch.staticGeometry) {
      this.drawGeometry(context, geometry);
    }

    for (const animation of batch.animations) {
      this.drawAnimation(context, animation);
    }
    context.restore();
  }

  private drawGeometry(context: CanvasRenderingContext2D, geometry: StaticGeometry): void {
    const vertices = geometry.vertices ?? [];
    if (vertices.length === 0) {
      return;
    }

    context.save();
    const fill = (geometry.style.fill as string | undefined) ?? "rgba(255, 255, 255, 0.15)";
    const stroke = (geometry.style.stroke as string | undefined) ?? "rgba(255, 255, 255, 0.4)";
    context.beginPath();
    context.moveTo(vertices[0][0], vertices[0][1]);
    for (let index = 1; index < vertices.length; index += 1) {
      context.lineTo(vertices[index][0], vertices[index][1]);
    }
    context.closePath();
    context.fillStyle = fill;
    context.strokeStyle = stroke;
    context.fill();
    context.stroke();
    context.restore();
  }

  private drawAnimation(context: CanvasRenderingContext2D, animation: AnimationFrame): void {
    const metadata = animation.metadata ?? {};
    const position = metadata.position as { x: number; y: number } | undefined;
    if (!position) {
      return;
    }

    const radius = typeof metadata.radius === "number" ? metadata.radius : 6;
    const fill = (metadata.fill as string | undefined) ?? "rgba(255, 160, 64, 0.6)";
    const stroke = (metadata.stroke as string | undefined) ?? "rgba(255, 160, 64, 0.9)";

    context.save();
    context.beginPath();
    context.arc(position.x, position.y, radius, 0, Math.PI * 2);
    context.fillStyle = fill;
    context.strokeStyle = stroke;
    context.fill();
    context.stroke();
    context.restore();
  }
}
