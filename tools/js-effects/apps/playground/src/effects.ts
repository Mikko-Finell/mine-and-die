import {
  BloodSplatterDefinition,
  ImpactBurstDefinition,
  MeleeSwingEffectDefinition,
  PlaceholderAuraDefinition,
  type EffectDefinition,
} from "@js-effects/effects-lib";

export interface EffectCatalogEntry<TOptions> {
  id: string;
  name: string;
  description: string;
  definition: EffectDefinition<TOptions>;
}

export type AnyEffectCatalogEntry = EffectCatalogEntry<any>;

export const availableEffects: AnyEffectCatalogEntry[] = [
  {
    id: PlaceholderAuraDefinition.type,
    name: "Placeholder Aura",
    description: "A pulsing glow that orbits the selected origin.",
    definition: PlaceholderAuraDefinition,
  },
  {
    id: MeleeSwingEffectDefinition.type,
    name: "Melee Swing",
    description:
      "The red melee hitbox used in-game; fades quickly after spawning.",
    definition: MeleeSwingEffectDefinition,
  },
  {
    id: ImpactBurstDefinition.type,
    name: "Impact Burst",
    description: "A one-shot burst that leaves a glowing decal behind.",
    definition: ImpactBurstDefinition,
  },
  {
    id: BloodSplatterDefinition.type,
    name: "Blood Splatter",
    description: "Sprays droplets that settle into dark stains on the ground.",
    definition: BloodSplatterDefinition,
  },
];

