const EFFECT_MAP_KEY = "effectInstancesById";

function isEffectManagerInstance(value) {
  return value && typeof value === "object";
}

export function ensureEffectRegistry(store) {
  if (!store || typeof store !== "object") {
    return new Map();
  }
  const existing = store[EFFECT_MAP_KEY];
  if (existing instanceof Map) {
    return existing;
  }
  const created = new Map();
  store[EFFECT_MAP_KEY] = created;
  return created;
}

export function mirrorEffectInstances(store, manager) {
  const registry = ensureEffectRegistry(store);
  registry.clear();
  if (!isEffectManagerInstance(manager)) {
    return registry;
  }
  const metadata = manager?.instanceMetadata;
  if (metadata instanceof Map) {
    for (const [instance, meta] of metadata.entries()) {
      if (!meta || typeof meta !== "object") {
        continue;
      }
      const effectId = typeof meta.id === "string" ? meta.id : null;
      if (!effectId) {
        continue;
      }
      if (!isEffectManagerInstance(instance)) {
        continue;
      }
      registry.set(effectId, instance);
    }
  }
  return registry;
}

export function getEffectInstanceById(store, effectId) {
  if (typeof effectId !== "string" || effectId.length === 0) {
    return null;
  }
  const registry = store && store[EFFECT_MAP_KEY];
  if (!(registry instanceof Map)) {
    return null;
  }
  return registry.get(effectId) ?? null;
}

export function removeEffectInstance(store, effectId) {
  if (typeof effectId !== "string" || effectId.length === 0) {
    return false;
  }
  const registry = store && store[EFFECT_MAP_KEY];
  if (!(registry instanceof Map)) {
    return false;
  }
  return registry.delete(effectId);
}
