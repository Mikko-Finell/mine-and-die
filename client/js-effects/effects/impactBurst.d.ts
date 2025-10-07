import { EffectDefinition } from "../types";
export interface ImpactBurstOptions {
    duration: number;
    ringRadius: number;
    particleCount: number;
    color: string;
    secondaryColor: string;
    decalRadius: number;
    decalTtl: number;
}
export declare const ImpactBurstDefinition: EffectDefinition<ImpactBurstOptions>;
