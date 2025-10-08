import { type DecalSpec, type EffectDefinition, type EffectFrameContext, type EffectInstance, type EffectPreset } from "./types.js";
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
/**
 * EffectManager owns the lifecycle of all visual EffectInstance objects.
 * Hosts feed it simulation state, triggers, and frame context; the manager
 * handles spawning, culling, updating, drawing, and decal ownership so
 * callers never track effect instances themselves.
 */
export declare class EffectManager {
    private effects;
    private finished;
    private pendingRemovals;
    private iterating;
    private creationCounter;
    private viewBounds;
    private stats;
    private effectIndex;
    private decals;
    private triggerHandlers;
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
    collectDecals(nowSeconds?: number | null): DecalSpec[];
    getLastFrameStats(): FrameStats;
    removeInstance<TOptions>(instance: EffectInstance<TOptions> | null | undefined): boolean;
    getInstanceById<TOptions>(id: string): EffectInstance<TOptions> | null;
    getInstancesByType(type: string): EffectInstance[];
    registerTrigger(type: string, handler: TriggerHandler): void;
    trigger(type: string, trigger: TriggerPayload, context?: Record<string, unknown> | null): void;
    triggerAll(triggers: Array<Record<string, unknown>> | null | undefined, context?: Record<string, unknown> | null): void;
    private track;
    private removeActiveInstance;
    private enqueueDecal;
}
export {};
