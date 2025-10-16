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
  constructor(public readonly configuration: RendererConfiguration) {}

  mount(_provider: RenderContextProvider): void {
    throw new Error("Renderer mount is not implemented.");
  }

  unmount(): void {
    throw new Error("Renderer unmount is not implemented.");
  }

  renderBatch(_batch: RenderBatch): void {
    throw new Error("Renderer renderBatch is not implemented.");
  }

  resize(_dimensions: RenderDimensions): void {
    throw new Error("Renderer resize is not implemented.");
  }
}
