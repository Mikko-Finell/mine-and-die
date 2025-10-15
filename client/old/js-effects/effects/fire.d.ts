import { type EffectDefinition } from "../types.js";
export interface FireOptions {
    spawnInterval: number;
    embersPerBurst: number;
    riseSpeed: number;
    windX: number;
    swirl: number;
    jitter: number;
    lifeScale: number;
    sizeScale: number;
    spawnRadius: number;
    concentration: number;
    emberPalette: string[];
    emberAlpha: number;
    additive: boolean;
}
export declare const FireEffectDefinition: EffectDefinition<FireOptions>;
