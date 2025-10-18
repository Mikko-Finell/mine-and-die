import type { AnimationFrame } from "./render";
import type { EffectCatalogEntryMetadata } from "./effect-catalog";
import type { EffectInstance as ContractEffectInstance } from "./generated/effect-contracts";
import {
  BloodSplatterDefinition,
  FireEffectDefinition,
  FireballEffectDefinition,
  ImpactBurstDefinition,
  MeleeSwingEffectDefinition,
  PlaceholderAuraDefinition,
  type EffectDefinition as RuntimeEffectDefinition,
} from "@js-effects/effects-lib";

export interface EffectSpawnIntent {
  readonly effectId: string;
  readonly definition: RuntimeEffectDefinition<any>;
  readonly options: Partial<any> & { x: number; y: number };
  readonly signature: string;
  readonly state: "active" | "ended";
  readonly retained: boolean;
}

interface TranslatorInput {
  readonly animation: AnimationFrame;
  readonly metadata: Record<string, unknown>;
  readonly instance: ContractEffectInstance;
  readonly catalog: EffectCatalogEntryMetadata | null;
  readonly parameters: Record<string, number>;
  readonly colors: readonly string[];
  readonly center: { x: number; y: number };
  readonly radius: number | null;
}

interface TranslatorResult {
  readonly definition: RuntimeEffectDefinition<any>;
  readonly options: Partial<any> & { x: number; y: number };
  readonly signatureProps: Record<string, unknown>;
}

type Translator = (input: TranslatorInput) => TranslatorResult | null;

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null && !Array.isArray(value);

const toNumber = (value: unknown): number | null => {
  const numeric = typeof value === "number" ? value : Number(value);
  if (!Number.isFinite(numeric)) {
    return null;
  }
  return numeric;
};

const positiveOr = (value: number | null, fallback: number): number => {
  if (value !== null && Number.isFinite(value) && value > 0) {
    return value;
  }
  return fallback;
};

const nonNegativeOr = (value: number | null, fallback: number): number => {
  if (value !== null && Number.isFinite(value) && value >= 0) {
    return value;
  }
  return fallback;
};

const roundForSignature = (value: number): number =>
  Math.round(value * 1000) / 1000;

const collectNumericProperties = (source: unknown): Record<string, number> => {
  if (!isRecord(source)) {
    return {};
  }
  const result: Record<string, number> = {};
  for (const [key, candidate] of Object.entries(source)) {
    const numeric = toNumber(candidate);
    if (numeric !== null) {
      result[key] = numeric;
    }
  }
  return result;
};

const mergeNumericMaps = (
  sources: readonly Record<string, number>[],
): Record<string, number> => {
  const merged: Record<string, number> = {};
  for (const source of sources) {
    for (const [key, value] of Object.entries(source)) {
      merged[key] = value;
    }
  }
  return merged;
};

const extractColors = (instance: ContractEffectInstance): readonly string[] => {
  const colors = Array.isArray(instance.colors) ? instance.colors : [];
  const filtered: string[] = [];
  for (const entry of colors) {
    if (typeof entry === "string" && entry.length > 0) {
      filtered.push(entry);
    }
  }
  return filtered;
};

const resolveCatalog = (metadata: Record<string, unknown>): EffectCatalogEntryMetadata | null => {
  const candidate = metadata.catalog;
  return isRecord(candidate) ? (candidate as EffectCatalogEntryMetadata) : null;
};

const resolveParameters = (
  metadata: Record<string, unknown>,
  instance: ContractEffectInstance,
): Record<string, number> => {
  const blocks = metadata.blocks;
  const blockParams = isRecord(blocks) ? collectNumericProperties(blocks.parameters) : {};
  const instanceParams = collectNumericProperties(instance.params);
  const behaviorExtra = collectNumericProperties(instance.behaviorState?.extra);
  return mergeNumericMaps([blockParams, instanceParams, behaviorExtra]);
};

const resolveCenter = (instance: ContractEffectInstance): { x: number; y: number } => {
  const delivery = instance.deliveryState ?? ({} as ContractEffectInstance["deliveryState"]);
  const geometry = delivery?.geometry ?? {};
  const motion = delivery?.motion ?? {};
  const baseX = toNumber(motion?.positionX) ?? 0;
  const baseY = toNumber(motion?.positionY) ?? 0;
  const offsetX = toNumber(geometry?.offsetX) ?? 0;
  const offsetY = toNumber(geometry?.offsetY) ?? 0;
  return {
    x: baseX + offsetX,
    y: baseY + offsetY,
  };
};

const resolveRadius = (instance: ContractEffectInstance): number | null => {
  const geometry = instance.deliveryState?.geometry ?? {};
  const radius = toNumber(geometry?.radius);
  if (radius !== null && radius > 0) {
    return radius;
  }
  const width = toNumber(geometry?.width) ?? toNumber(geometry?.length);
  const height = toNumber(geometry?.height) ?? width;
  if (width !== null && height !== null) {
    return Math.max(width, height) / 2;
  }
  return null;
};

const translateMeleeSwing: Translator = ({ instance, colors, center, parameters }) => {
  const geometry = instance.deliveryState?.geometry ?? {};
  const defaultWidth = MeleeSwingEffectDefinition.defaults.width;
  const defaultHeight = MeleeSwingEffectDefinition.defaults.height;
  const width = positiveOr(toNumber(geometry?.width) ?? toNumber(geometry?.length), defaultWidth);
  const height = positiveOr(toNumber(geometry?.height) ?? toNumber(geometry?.width), defaultHeight);
  const x = center.x - width / 2;
  const y = center.y - height / 2;

  const options: Record<string, unknown> = {
    x,
    y,
    width,
    height,
    effectId: instance.id,
  };

  if (colors.length > 0) {
    options.fill = colors[0];
  }
  if (colors.length > 1) {
    options.stroke = colors[1];
  }
  if (colors.length > 2) {
    options.innerFill = colors[2];
  }

  if (Number.isFinite(parameters.durationMs)) {
    const durationSeconds = Math.max(0.05, parameters.durationMs / 1000);
    options.duration = durationSeconds;
  }

  const signatureProps = {
    x: roundForSignature(x),
    y: roundForSignature(y),
    width: roundForSignature(width),
    height: roundForSignature(height),
    fill: options.fill ?? null,
    stroke: options.stroke ?? null,
  };

  return {
    definition: MeleeSwingEffectDefinition,
    options: options as Partial<any> & { x: number; y: number },
    signatureProps,
  };
};

const translateBloodSplatter: Translator = ({ instance, colors, center, parameters }) => {
  const maxBursts = parameters.drops ? Math.max(1, Math.round(parameters.drops)) : undefined;
  const palette: [string, string] | undefined = colors.length >= 2
    ? [colors[0]!, colors[1]!]
    : colors.length === 1
      ? [colors[0]!, colors[0]!]
      : undefined;

  const options: Record<string, unknown> = {
    x: center.x,
    y: center.y,
    effectId: instance.id,
  };

  if (maxBursts !== undefined) {
    options.maxBursts = Math.max(1, Math.min(400, maxBursts));
  }
  if (palette) {
    options.colors = palette;
  }

  const signatureProps = {
    x: roundForSignature(center.x),
    y: roundForSignature(center.y),
    maxBursts: options.maxBursts ?? null,
    colors: palette ? palette.join("|") : null,
  };

  return {
    definition: BloodSplatterDefinition,
    options: options as Partial<any> & { x: number; y: number },
    signatureProps,
  };
};

const translateFire: Translator = ({ instance, colors, center, parameters }) => {
  const palette = colors.length > 0 ? colors : undefined;
  const spawnRadius = positiveOr(parameters.spawnRadius ?? null, FireEffectDefinition.defaults.spawnRadius);
  const additive = parameters.additive === 1 || parameters.additive === true;

  const options: Record<string, unknown> = {
    x: center.x,
    y: center.y,
    effectId: instance.id,
    spawnRadius,
  };

  if (palette) {
    options.emberPalette = palette;
  }
  if (additive) {
    options.additive = true;
  }
  if (Number.isFinite(parameters.sizeScale)) {
    options.sizeScale = Math.max(0.1, parameters.sizeScale);
  }

  const signatureProps = {
    x: roundForSignature(center.x),
    y: roundForSignature(center.y),
    palette: palette ? palette.join("|") : null,
    spawnRadius: roundForSignature(spawnRadius),
    sizeScale: options.sizeScale ?? null,
    additive: options.additive === true,
  };

  return {
    definition: FireEffectDefinition,
    options: options as Partial<any> & { x: number; y: number },
    signatureProps,
  };
};

const translateImpactBurst: Translator = ({ instance, colors, center, radius, parameters }) => {
  const primary = colors[0] ?? ImpactBurstDefinition.defaults.color;
  const secondary = colors[1] ?? ImpactBurstDefinition.defaults.secondaryColor;
  const ringRadius = positiveOr(radius, ImpactBurstDefinition.defaults.ringRadius);
  const decalRadius = positiveOr(parameters.decalRadius ?? null, ImpactBurstDefinition.defaults.decalRadius);

  const options: Record<string, unknown> = {
    x: center.x,
    y: center.y,
    effectId: instance.id,
    color: primary,
    secondaryColor: secondary,
    ringRadius,
    decalRadius,
  };

  if (Number.isFinite(parameters.durationMs)) {
    const seconds = Math.max(0.05, parameters.durationMs / 1000);
    options.duration = seconds;
  }

  const signatureProps = {
    x: roundForSignature(center.x),
    y: roundForSignature(center.y),
    color: primary,
    secondaryColor: secondary,
    ringRadius: roundForSignature(ringRadius),
    decalRadius: roundForSignature(decalRadius),
    duration: options.duration ?? null,
  };

  return {
    definition: ImpactBurstDefinition,
    options: options as Partial<any> & { x: number; y: number },
    signatureProps,
  };
};

const translateFireball: Translator = ({ instance, colors, center, parameters, radius }) => {
  const motion = instance.deliveryState?.motion ?? {};
  const velocityX = toNumber(motion?.velocityX) ?? 0;
  const velocityY = toNumber(motion?.velocityY) ?? 0;
  const heading = Math.atan2(velocityY, velocityX);
  const speedFromParams = parameters.speed ?? null;
  const speed = positiveOr(speedFromParams ?? Math.hypot(velocityX, velocityY), FireballEffectDefinition.defaults.speed);
  const range = positiveOr(parameters.range ?? null, FireballEffectDefinition.defaults.range);
  const effectRadius = positiveOr(radius, FireballEffectDefinition.defaults.radius);

  const palette = colors.length > 0 ? colors : undefined;
  const options: Record<string, unknown> = {
    x: center.x,
    y: center.y,
    effectId: instance.id,
    speed,
    range,
    radius: effectRadius,
    heading,
  };

  if (palette) {
    options.coreColor = palette[0];
    if (palette[1]) {
      options.midColor = palette[1];
    }
    if (palette[2]) {
      options.rimColor = palette[2];
    }
  }

  const signatureProps = {
    x: roundForSignature(center.x),
    y: roundForSignature(center.y),
    speed: roundForSignature(speed),
    range: roundForSignature(range),
    radius: roundForSignature(effectRadius),
    heading: roundForSignature(heading),
    palette: palette ? palette.join("|") : null,
  };

  return {
    definition: FireballEffectDefinition,
    options: options as Partial<any> & { x: number; y: number },
    signatureProps,
  };
};

const translatePlaceholder: Translator = ({ instance, center, radius, colors }) => {
  const effectRadius = positiveOr(radius, PlaceholderAuraDefinition.defaults.radius);
  const palette = colors.length > 0 ? colors : PlaceholderAuraDefinition.defaults.colors;

  const options: Record<string, unknown> = {
    x: center.x,
    y: center.y,
    effectId: instance.id,
    radius: effectRadius,
    colors: palette,
  };

  const signatureProps = {
    x: roundForSignature(center.x),
    y: roundForSignature(center.y),
    radius: roundForSignature(effectRadius),
    palette: palette.join("|"),
  };

  return {
    definition: PlaceholderAuraDefinition,
    options: options as Partial<any> & { x: number; y: number },
    signatureProps,
  };
};

const TRANSLATORS: Record<string, Translator> = {
  "melee/swing": translateMeleeSwing,
  "visual/blood-splatter": translateBloodSplatter,
  "status/burning-visual": translateFire,
  "status/burning-tick": translateImpactBurst,
  "projectile/fireball": translateFireball,
};

const CONTRACT_FALLBACKS: Record<string, string> = {
  attack: "melee/swing",
  "blood-splatter": "visual/blood-splatter",
  "burning-tick": "status/burning-tick",
  fire: "status/burning-visual",
  fireball: "projectile/fireball",
};

const resolveJsEffectId = (
  metadata: Record<string, unknown>,
  instance: ContractEffectInstance,
): string | null => {
  const blocks = metadata.blocks;
  if (isRecord(blocks)) {
    const id = blocks.jsEffect;
    if (typeof id === "string" && id.length > 0) {
      return id;
    }
  }
  const catalog = resolveCatalog(metadata);
  if (catalog) {
    const id = (catalog.blocks as Record<string, unknown>).jsEffect;
    if (typeof id === "string" && id.length > 0) {
      return id;
    }
  }
  const contractId = typeof metadata.contractId === "string" && metadata.contractId.length > 0
    ? metadata.contractId
    : typeof instance.definitionId === "string"
      ? instance.definitionId
      : null;
  if (contractId) {
    const fallback = CONTRACT_FALLBACKS[contractId];
    if (fallback) {
      return fallback;
    }
  }
  return null;
};

const resolveState = (metadata: Record<string, unknown>): "active" | "ended" => {
  const state = metadata.state;
  if (state === "ended") {
    return "ended";
  }
  const lastEvent = typeof metadata.lastEventKind === "string" ? metadata.lastEventKind : null;
  return lastEvent === "end" ? "ended" : "active";
};

export const translateRenderAnimation = (animation: AnimationFrame): EffectSpawnIntent | null => {
  const metadata = isRecord(animation.metadata) ? animation.metadata : null;
  if (!metadata) {
    return null;
  }

  const instance = isRecord(metadata.instance)
    ? (metadata.instance as ContractEffectInstance)
    : null;
  if (!instance) {
    return null;
  }

  const effectId = typeof animation.effectId === "string" && animation.effectId.length > 0
    ? animation.effectId
    : typeof instance.id === "string" && instance.id.length > 0
      ? instance.id
      : null;
  if (!effectId) {
    return null;
  }

  const catalog = resolveCatalog(metadata);
  const parameters = resolveParameters(metadata, instance);
  const colors = extractColors(instance);
  const center = resolveCenter(instance);
  const radius = resolveRadius(instance);

  const translatorKey = resolveJsEffectId(metadata, instance);
  const translator = translatorKey ? TRANSLATORS[translatorKey] ?? null : null;
  const result = (translator ?? translatePlaceholder)({
    animation,
    metadata,
    instance,
    catalog,
    parameters,
    colors,
    center,
    radius,
  });

  if (!result) {
    return null;
  }

  const state = resolveState(metadata);
  const retained = metadata.retained === true;
  const signature = JSON.stringify(result.signatureProps);

  return {
    effectId,
    definition: result.definition,
    options: result.options,
    signature,
    state,
    retained,
  };
};

