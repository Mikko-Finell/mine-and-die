import { EffectDefinition } from "../types";
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
}
export declare const BloodSplatterDefinition: EffectDefinition<BloodSplatterOptions>;
