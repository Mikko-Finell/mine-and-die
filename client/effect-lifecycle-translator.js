const COORD_SCALE = 16;
const DEFAULT_TICK_RATE = 15;
const DEFAULT_TILE_SIZE = 40;

const CONTRACT_TYPE_REMAP = {
  attack: "melee-swing",
};

// Keep CONTRACT_PARAM_SCALE in sync with contractEffectParamScale in
// server/effects_manager.go.
const CONTRACT_PARAM_SCALE = 1024;

const PARAM_KEYS_BY_TYPE = new Map([
  [
    "blood-splatter",
    new Set([
      "drag",
      "dropletRadius",
      "maxBursts",
      "maxDroplets",
      "maxStainRadius",
      "maxStains",
      "minDroplets",
      "minStainRadius",
      "spawnInterval",
      "speed",
    ]),
  ],
]);

export function normalizeContractEffectType(type) {
  if (typeof type !== "string") {
    return type ?? null;
  }
  const trimmed = type.trim();
  if (trimmed.length === 0) {
    return trimmed;
  }
  return CONTRACT_TYPE_REMAP[trimmed] || trimmed;
}

function isPlainObject(value) {
  return value != null && typeof value === "object" && !Array.isArray(value);
}

function getTileSize(store) {
  const tileSize = Number(store?.TILE_SIZE);
  return Number.isFinite(tileSize) && tileSize > 0 ? tileSize : DEFAULT_TILE_SIZE;
}

function getTickRate(store) {
  const tickRate = Number(store?.tickRate ?? store?.TICK_RATE);
  if (Number.isFinite(tickRate) && tickRate > 0) {
    return tickRate;
  }
  return DEFAULT_TICK_RATE;
}

function quantizedToWorld(value, tileSize) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric)) {
    return null;
  }
  return (numeric / COORD_SCALE) * tileSize;
}

function contractMotionAppearsZeroed(motion) {
  if (!isPlainObject(motion)) {
    return true;
  }
  const positionX = Number(motion.positionX);
  const positionY = Number(motion.positionY);
  const velocityX = Number(motion.velocityX);
  const velocityY = Number(motion.velocityY);
  const positionZero =
    (!Number.isFinite(positionX) || positionX === 0) &&
    (!Number.isFinite(positionY) || positionY === 0);
  const velocityZero =
    (!Number.isFinite(velocityX) || velocityX === 0) &&
    (!Number.isFinite(velocityY) || velocityY === 0);
  return positionZero && velocityZero;
}

function copyParams(source, target) {
  if (!isPlainObject(source)) {
    return;
  }
  for (const [key, raw] of Object.entries(source)) {
    if (!key || typeof key !== "string") {
      continue;
    }
    const value = Number(raw);
    if (Number.isFinite(value)) {
      target[key] = value;
    }
  }
}

function collectTypeHints(instance, resolvedType) {
  const hints = new Set();
  const add = (value) => {
    if (typeof value !== "string") {
      return;
    }
    const trimmed = value.trim().toLowerCase();
    if (trimmed.length > 0) {
      hints.add(trimmed);
    }
  };

  add(resolvedType);
  if (instance && typeof instance === "object") {
    add(instance.definitionId);
    add(instance.type);
    add(instance.typeId);
    const definition = instance.definition && typeof instance.definition === "object"
      ? instance.definition
      : null;
    if (definition) {
      add(definition.type);
      add(definition.typeId);
    }
  }

  return hints;
}

function collectQuantizedParamKeys(typeHints) {
  if (!(typeHints instanceof Set) || typeHints.size === 0) {
    return null;
  }
  const collected = new Set();
  for (const hint of typeHints) {
    if (typeof hint !== "string" || hint.length === 0) {
      continue;
    }
    const keySet = PARAM_KEYS_BY_TYPE.get(hint);
    if (!keySet) {
      continue;
    }
    for (const key of keySet) {
      collected.add(key);
    }
  }
  return collected.size > 0 ? collected : null;
}

function dequantizeContractParams(params, keySet) {
  if (!isPlainObject(params) || !(keySet instanceof Set) || keySet.size === 0) {
    return;
  }
  for (const key of keySet) {
    if (typeof key !== "string" || key.length === 0) {
      continue;
    }
    const raw = params[key];
    if (!Number.isFinite(raw)) {
      continue;
    }
    params[key] = raw / CONTRACT_PARAM_SCALE;
  }
}

function normalizeColorList(source) {
  if (!Array.isArray(source)) {
    return [];
  }
  const colors = [];
  for (const entry of source) {
    if (typeof entry !== "string") {
      continue;
    }
    const trimmed = entry.trim();
    if (trimmed.length > 0) {
      colors.push(trimmed);
    }
  }
  return colors;
}

function findActorPosition(renderState, store, actorId) {
  if (typeof actorId !== "string" || actorId.length === 0) {
    return null;
  }
  const sources = [
    renderState?.players?.[actorId],
    store?.displayPlayers?.[actorId],
    store?.players?.[actorId],
    renderState?.npcs?.[actorId],
    store?.displayNPCs?.[actorId],
    store?.npcs?.[actorId],
  ];
  for (const source of sources) {
    if (!source || typeof source !== "object") {
      continue;
    }
    const x = Number(source.x);
    const y = Number(source.y);
    if (Number.isFinite(x) && Number.isFinite(y)) {
      return { x, y };
    }
  }
  return null;
}

/**
 * Convert a contract lifecycle entry into an effect-shaped payload compatible
 * with legacy render definitions.
 *
 * @param {{ instance?: Record<string, any> } | null | undefined} lifecycleEntry
 * @param {{
 *   store?: Record<string, any> | null,
 *   renderState?: Record<string, any> | null,
 * }=} context
 * @returns {Record<string, any> | null}
 */
export function contractLifecycleToEffect(lifecycleEntry, context = {}) {
  const { store = null, renderState = null } = context;
  if (!isPlainObject(lifecycleEntry) || !isPlainObject(lifecycleEntry.instance)) {
    return null;
  }

  const instance = lifecycleEntry.instance;
  const tileSize = getTileSize(store);
  const tickRate = getTickRate(store);
  const tickDurationMs = 1000 / tickRate;
  const quantToWorld = (value) => quantizedToWorld(value, tileSize);

  const effect = {};
  if (typeof instance.id === "string" && instance.id.length > 0) {
    effect.id = instance.id;
  }
  const rawDefinitionId =
    typeof instance.definitionId === "string" && instance.definitionId.length > 0
      ? instance.definitionId
      : null;
  const rawTypeId =
    typeof instance.definition?.typeId === "string" &&
    instance.definition.typeId.length > 0
      ? instance.definition.typeId
      : null;
  const resolvedType = normalizeContractEffectType(rawTypeId || rawDefinitionId);
  if (resolvedType) {
    effect.type = resolvedType;
  }

  const params = {};
  if (isPlainObject(instance.params)) {
    copyParams(instance.params, params);
  }
  const typeHints = collectTypeHints(instance, resolvedType ?? rawDefinitionId ?? rawTypeId);
  const quantizedKeys = collectQuantizedParamKeys(typeHints);
  dequantizeContractParams(params, quantizedKeys);
  const behaviorExtra = instance.behaviorState?.extra;
  if (isPlainObject(behaviorExtra)) {
    copyParams(behaviorExtra, params);
  }

  const centerFromExtraX = quantToWorld(behaviorExtra?.centerX);
  const centerFromExtraY = quantToWorld(behaviorExtra?.centerY);
  if (centerFromExtraX !== null) {
    params.centerX = centerFromExtraX;
  }
  if (centerFromExtraY !== null) {
    params.centerY = centerFromExtraY;
  }
  effect.params = Object.keys(params).length > 0 ? params : undefined;
  if (effect.params) {
    for (const [key, value] of Object.entries(effect.params)) {
      if (typeof key !== "string" || key.length === 0) {
        continue;
      }
      if (!(key in effect) && Number.isFinite(value)) {
        effect[key] = value;
      }
    }
  }

  const instanceColors = normalizeColorList(instance.colors);
  const colors = instanceColors;
  if (colors.length > 0) {
    effect.colors = colors;
  }

  const ticksRemaining = Number(instance.behaviorState?.ticksRemaining);
  if (Number.isFinite(ticksRemaining) && ticksRemaining > 0) {
    effect.duration = ticksRemaining * tickDurationMs;
  }

  const geometry = instance.deliveryState?.geometry ?? null;
  const motion = instance.deliveryState?.motion ?? null;

  let width = quantToWorld(geometry?.width);
  let height = quantToWorld(geometry?.height);
  if ((width === null || width <= 0) && Number.isFinite(params.width)) {
    const paramWidth = Number(params.width);
    if (Number.isFinite(paramWidth) && paramWidth > 0) {
      width = paramWidth;
    }
  }
  if ((height === null || height <= 0) && Number.isFinite(params.height)) {
    const paramHeight = Number(params.height);
    if (Number.isFinite(paramHeight) && paramHeight > 0) {
      height = paramHeight;
    }
  }
  const radius = quantToWorld(geometry?.radius);
  if (radius !== null) {
    const diameter = radius * 2;
    if (width === null) {
      width = diameter;
    }
    if (height === null) {
      height = diameter;
    }
  }
  if (width !== null) {
    effect.width = width;
  }
  if (height !== null) {
    effect.height = height;
  }

  const motionCenterX = quantToWorld(motion?.positionX);
  const motionCenterY = quantToWorld(motion?.positionY);
  const motionLooksZeroed = contractMotionAppearsZeroed(motion);

  let centerX = centerFromExtraX;
  let centerY = centerFromExtraY;
  const derivedFromMotionX = centerX == null && motionCenterX !== null;
  const derivedFromMotionY = centerY == null && motionCenterY !== null;
  if (derivedFromMotionX) {
    centerX = motionCenterX;
  }
  if (derivedFromMotionY) {
    centerY = motionCenterY;
  }

  const offsetX = quantToWorld(geometry?.offsetX);
  const offsetY = quantToWorld(geometry?.offsetY);

  const anchor =
    findActorPosition(renderState, store, instance.followActorId) ??
    findActorPosition(renderState, store, instance.ownerActorId);

  if (anchor) {
    if (offsetX !== null) {
      centerX = anchor.x + offsetX;
    } else if (
      centerX == null ||
      (derivedFromMotionX && motionLooksZeroed)
    ) {
      centerX = anchor.x;
    }
    if (offsetY !== null) {
      centerY = anchor.y + offsetY;
    } else if (
      centerY == null ||
      (derivedFromMotionY && motionLooksZeroed)
    ) {
      centerY = anchor.y;
    }
  } else {
    if (centerX == null && offsetX !== null) {
      centerX = offsetX;
    }
    if (centerY == null && offsetY !== null) {
      centerY = offsetY;
    }
  }

  if (centerX != null) {
    if (Number.isFinite(effect.width)) {
      effect.x = centerX - effect.width / 2;
    } else {
      effect.x = centerX;
    }
  }
  if (centerY != null) {
    if (Number.isFinite(effect.height)) {
      effect.y = centerY - effect.height / 2;
    } else {
      effect.y = centerY;
    }
  }

  if (typeof instance.ownerActorId === "string" && instance.ownerActorId.length > 0) {
    effect.ownerActorId = instance.ownerActorId;
  }
  if (typeof instance.followActorId === "string" && instance.followActorId.length > 0) {
    effect.followActorId = instance.followActorId;
  }

  if (
    isPlainObject(instance.behaviorState) &&
    Number.isFinite(instance.behaviorState.cooldownTicks)
  ) {
    effect.cooldownTicks = Number(instance.behaviorState.cooldownTicks);
  }

  return effect;
}

export function contractLifecycleToUpdatePayload(lifecycleEntry, context = {}) {
  return contractLifecycleToEffect(lifecycleEntry, context);
}
