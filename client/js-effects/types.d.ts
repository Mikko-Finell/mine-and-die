export interface RandomGenerator {
    next(): number;
    seedFrom?(id: string): void;
}
export interface EffectFrameContext {
    ctx: CanvasRenderingContext2D;
    dt: number;
    now: number;
    camera: {
        toScreenX(x: number): number;
        toScreenY(y: number): number;
        zoom: number;
    };
    rng?: RandomGenerator;
}
export interface DecalSpec {
    x: number;
    y: number;
    rotation?: number;
    shape?: {
        type: "oval";
        rx: number;
        ry: number;
    } | {
        type: "rect";
        w: number;
        h: number;
    } | {
        type: "poly";
        points: number[];
    };
    texture?: HTMLCanvasElement | ImageBitmap | string;
    averageColor?: string;
    ttl?: number;
    layerHint?: string;
}
export declare enum EffectLayer {
    GroundDecal = 100,
    ActorOverlay = 200
}
export interface EffectInstance<TOptions = unknown> {
    readonly id: string;
    readonly type: string;
    layer: EffectLayer;
    sublayer?: number;
    kind?: "once" | "loop";
    isAlive(): boolean;
    dispose?(): void;
    handoffToDecal?(): DecalSpec | null;
    getAABB(): {
        x: number;
        y: number;
        w: number;
        h: number;
    };
    update(frame: EffectFrameContext): void;
    draw(frame: EffectFrameContext): void;
}
export interface EffectDefinition<TOptions> {
    type: string;
    defaults: TOptions;
    create(opts: Partial<TOptions> & {
        x: number;
        y: number;
    }): EffectInstance<TOptions>;
    createFromPreset?(position: {
        x: number;
        y: number;
    }, preset?: EffectPreset | Partial<EffectPreset>, overrides?: Record<string, unknown>): EffectInstance<TOptions>;
}
export interface EffectPreset {
    name?: string;
    version?: string;
    options: Record<string, unknown>;
    meta?: {
        author?: string;
        notes?: string;
    };
}
