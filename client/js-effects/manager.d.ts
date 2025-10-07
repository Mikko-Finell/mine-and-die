import { type DecalSpec, type EffectDefinition, type EffectFrameContext, type EffectInstance, type EffectPreset } from "./types";
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
export declare class EffectManager {
    private effects;
    private finished;
    private pendingRemovals;
    private iterating;
    private creationCounter;
    private viewBounds;
    private stats;
    spawn<TOptions>(definition: EffectDefinition<TOptions>, options: Partial<TOptions> & {
        x: number;
        y: number;
    }): EffectInstance<TOptions>;
    spawnFromPreset<TOptions>(definition: EffectDefinition<TOptions>, position: {
        x: number;
        y: number;
    }, preset?: EffectPreset | Partial<EffectPreset>, overrides?: Record<string, unknown>): EffectInstance<TOptions>;
    addInstance<TOptions>(instance: EffectInstance<TOptions>): EffectInstance<TOptions>;
    clear(): void;
    cullByAABB(view: ViewBounds): void;
    updateAll(frame: EffectFrameContext): void;
    drawAll(frame: EffectFrameContext): void;
    collectDecals(): DecalSpec[];
    getLastFrameStats(): FrameStats;
    removeInstance<TOptions>(instance: EffectInstance<TOptions> | null | undefined): boolean;
    private track;
    private removeActiveInstance;
}
export {};
