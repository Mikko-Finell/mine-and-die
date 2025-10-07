import { type EffectDefinition } from "../types.js";
export interface FireOptions {
    spawnInterval: number;
    embersPerBurst: number;
    flamesPerBurst: number;
    riseSpeed: number;
    windX: number;
    swirl: number;
    jitter: number;
    sizeScale: number;
    lifeScale: number;
    baseColor: string;
    midColor: string;
    hotColor: string;
    emberColor: string;
    emberAlpha: number;
    gradientBias: number;
    additive: boolean;
}
export declare const FireEffectDefinition: EffectDefinition<FireOptions>;
