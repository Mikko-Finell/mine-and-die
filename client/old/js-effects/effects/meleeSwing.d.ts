import { type EffectDefinition } from "../types.js";
export interface MeleeSwingOptions {
    duration: number;
    width: number;
    height: number;
    fill: string;
    stroke: string;
    strokeWidth: number;
    innerFill: string;
    innerInset: number;
    fadeExponent: number;
    effectId?: string;
}
export declare const MeleeSwingEffectDefinition: EffectDefinition<MeleeSwingOptions>;
