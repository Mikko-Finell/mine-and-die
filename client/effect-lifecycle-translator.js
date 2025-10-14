const COORD_SCALE = 16;
const DEFAULT_TICK_RATE = 15;
const DEFAULT_TILE_SIZE = 40;

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

function cloneEffect(fallbackEffect) {
  if (!isPlainObject(fallbackEffect)) {
    return {};
  }
  const clone = { ...fallbackEffect };
  if (isPlainObject(fallbackEffect.params)) {
    clone.params = { ...fallbackEffect.params };
  }
  return clone;
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
 *   fallbackEffect?: Record<string, any> | null,
 * }=} context
 * @returns {Record<string, any> | null}
 */
export function contractLifecycleToEffect(lifecycleEntry, context = {}) {
  const { store = null, renderState = null, fallbackEffect = null } = context;
  if (!isPlainObject(lifecycleEntry) || !isPlainObject(lifecycleEntry.instance)) {
    return cloneEffect(fallbackEffect) ?? null;
  }

  const instance = lifecycleEntry.instance;
  const tileSize = getTileSize(store);
  const tickRate = getTickRate(store);
  const tickDurationMs = 1000 / tickRate;

  const effect = cloneEffect(fallbackEffect);
  if (typeof instance.id === "string" && instance.id.length > 0) {
    effect.id = instance.id;
  }
  if (typeof instance.definitionId === "string" && instance.definitionId.length > 0) {
    effect.type = instance.definitionId;
  }

  const params = isPlainObject(effect.params) ? { ...effect.params } : {};
  if (isPlainObject(instance.params)) {
    copyParams(instance.params, params);
  }
  if (isPlainObject(instance.behaviorState?.extra)) {
    copyParams(instance.behaviorState.extra, params);
  }
  effect.params = Object.keys(params).length > 0 ? params : undefined;

  const ticksRemaining = Number(instance.behaviorState?.ticksRemaining);
  if (Number.isFinite(ticksRemaining) && ticksRemaining > 0) {
    effect.duration = ticksRemaining * tickDurationMs;
  }

  const geometry = instance.deliveryState?.geometry ?? null;
  const motion = instance.deliveryState?.motion ?? null;

  const quantToWorld = (value) => quantizedToWorld(value, tileSize);

  let width = quantToWorld(geometry?.width);
  let height = quantToWorld(geometry?.height);
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

  let centerX = quantToWorld(motion?.positionX);
  let centerY = quantToWorld(motion?.positionY);

  const offsetX = quantToWorld(geometry?.offsetX);
  const offsetY = quantToWorld(geometry?.offsetY);

  const anchor =
    findActorPosition(renderState, store, instance.followActorId) ??
    findActorPosition(renderState, store, instance.ownerActorId);

  if (anchor) {
    if (offsetX !== null) {
      centerX = anchor.x + offsetX;
    } else if (centerX === null || centerX === 0) {
      centerX = anchor.x;
    }
    if (offsetY !== null) {
      centerY = anchor.y + offsetY;
    } else if (centerY === null || centerY === 0) {
      centerY = anchor.y;
    }
  } else {
    if (centerX === null && offsetX !== null) {
      centerX = offsetX;
    }
    if (centerY === null && offsetY !== null) {
      centerY = offsetY;
    }
  }

  if (centerX !== null) {
    if (Number.isFinite(effect.width)) {
      effect.x = centerX - effect.width / 2;
    } else {
      effect.x = centerX;
    }
  }
  if (centerY !== null) {
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
