import type { EffectPreset } from "./types";

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null && !Array.isArray(value);

const normalizePreset = (input: EffectPreset): EffectPreset => {
  return {
    name: input.name,
    version: input.version,
    options: { ...(input.options ?? {}) },
    meta: input.meta ? { ...input.meta } : undefined,
  };
};

const validatePresetShape = (candidate: unknown): EffectPreset => {
  if (!isRecord(candidate)) {
    throw new Error("Preset must be an object");
  }

  const { options, name, version, meta } = candidate;

  if (!isRecord(options)) {
    throw new Error("Preset.options must be an object");
  }

  if (name !== undefined && typeof name !== "string") {
    throw new Error("Preset.name must be a string if provided");
  }

  if (version !== undefined && typeof version !== "string") {
    throw new Error("Preset.version must be a string if provided");
  }

  if (meta !== undefined) {
    if (!isRecord(meta)) {
      throw new Error("Preset.meta must be an object if provided");
    }
    const { author, notes } = meta;
    if (author !== undefined && typeof author !== "string") {
      throw new Error("Preset.meta.author must be a string if provided");
    }
    if (notes !== undefined && typeof notes !== "string") {
      throw new Error("Preset.meta.notes must be a string if provided");
    }
  }

  return normalizePreset({
    name: name as string | undefined,
    version: version as string | undefined,
    options: options as Record<string, unknown>,
    meta: meta as EffectPreset["meta"],
  });
};

export const loadPreset = async (
  source: string | object
): Promise<EffectPreset> => {
  if (typeof source === "string") {
    const response = await fetch(source);
    if (!response.ok) {
      throw new Error(`Failed to load preset from ${source}: ${response.status}`);
    }
    const data = await response.json();
    return validatePresetShape(data);
  }

  if (!isRecord(source)) {
    throw new Error("Preset source must be a string or object");
  }

  return validatePresetShape(source);
};
