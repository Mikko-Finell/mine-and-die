import { EffectDefinition } from "../types.js";
export interface BloodSplatterOptions {
    spawnInterval: number;
    minDroplets: number;
    maxDroplets: number;
    dropletRadius: number;
    minStainRadius: number;
    maxStainRadius: number;
    drag: number;
    speed: number;
    colors: [string, string];
    maxStains: number;
    maxBursts: number;
}
export declare const BloodSplatterDefinition: EffectDefinition<BloodSplatterOptions>;
