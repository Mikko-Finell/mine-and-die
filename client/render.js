import { EffectManager } from "./js-effects/manager.js";
import { MeleeSwingEffectDefinition } from "./js-effects/effects/meleeSwing.js";
import { BloodSplatterDefinition } from "./js-effects/effects/bloodSplatter.js";
import { FireEffectDefinition } from "./js-effects/effects/fire.js";
import {
  makeRectZoneDefinition,
  updateRectZoneInstance,
} from "./js-effects/effects/rectZone.js";
import { EffectLayer } from "./js-effects/types.js";
import {
  ensureEffectRegistry,
  mirrorEffectInstances,
} from "./effect-manager-adapter.js";
import {
  contractLifecycleToEffect,
  contractLifecycleToUpdatePayload,
} from "./effect-lifecycle-translator.js";
import { peekEffectLifecycleState } from "./effect-lifecycle.js";
import {
  RENDER_MODE_PATCH,
  RENDER_MODE_SNAPSHOT,
} from "./render-modes.js";

export { RENDER_MODE_PATCH, RENDER_MODE_SNAPSHOT } from "./render-modes.js";

/**
 * render.js bridges authoritative simulation state to visuals: it lerps actors,
 * updates the shared EffectManager, and draws the scene every animation frame.
 */

const DEFAULT_FACING = "down";
const FACING_OFFSETS = {
  up: { x: 0, y: -1 },
  down: { x: 0, y: 1 },
  left: { x: -1, y: 0 },
  right: { x: 1, y: 0 },
};

const EFFECT_STYLES = {
  fireball: {
    fill: "rgba(251, 191, 36, 0.35)",
    stroke: "rgba(251, 146, 60, 0.95)",
  },
};

const FireballZoneEffectDefinition = makeRectZoneDefinition("fireball", {
  fill: EFFECT_STYLES.fireball.fill,
  stroke: EFFECT_STYLES.fireball.stroke,
  lineWidth: 2,
});

if (typeof EffectLayer !== "object" || typeof EffectLayer.ActorOverlay !== "number") {
  throw new Error("EffectLayer.ActorOverlay is not defined; rebuild js-effects to sync layers.");
}
const ACTOR_OVERLAY_LAYER = EffectLayer.ActorOverlay;
const GROUND_EFFECT_MAX_LAYER = ACTOR_OVERLAY_LAYER - 1;

function isPlainObject(value) {
  return value != null && typeof value === "object" && !Array.isArray(value);
}

function toEntriesMap(source) {
  if (isPlainObject(source)) {
    return source;
  }
  return Object.create(null);
}

function toEffectArray(source) {
  if (Array.isArray(source)) {
    return source;
  }
  if (isPlainObject(source)) {
    return Object.values(source);
  }
  return [];
}

function mapDefinitionToLegacyType(definitionId) {
  if (typeof definitionId !== "string" || definitionId.length === 0) {
    return null;
  }
  switch (definitionId) {
    case "melee-swing":
      return "attack";
    default:
      return definitionId;
  }
}

function markHiddenMetadata(target, key, value) {
  if (!target || typeof target !== "object" || typeof key !== "string") {
    return;
  }
  Object.defineProperty(target, key, {
    value,
    enumerable: false,
    configurable: true,
    writable: true,
  });
}

function addEffectToBucket(buckets, type, effect) {
  if (!buckets || typeof type !== "string" || type.length === 0 || !effect) {
    return;
  }
  const existing = buckets.get(type);
  if (existing) {
    existing.push(effect);
  } else {
    buckets.set(type, [effect]);
  }
}

function resolveLifecycleEffectType(entry, convertedEffect, fallbackEffect) {
  if (fallbackEffect && typeof fallbackEffect.type === "string") {
    return fallbackEffect.type;
  }
  if (convertedEffect && typeof convertedEffect.type === "string") {
    const definitionId =
      entry && typeof entry.instance?.definitionId === "string"
        ? entry.instance.definitionId
        : null;
    if (definitionId && convertedEffect.type !== definitionId) {
      return convertedEffect.type;
    }
    return mapDefinitionToLegacyType(convertedEffect.type);
  }
  const definitionId =
    entry && typeof entry.instance?.definitionId === "string"
      ? entry.instance.definitionId
      : null;
  return mapDefinitionToLegacyType(definitionId);
}

function collectEffectRenderBuckets(store, renderState) {
  const buckets = new Map();
  const lifecycle = renderState?.lifecycle ?? null;
  const lifecycleEntries =
    lifecycle?.entries instanceof Map
      ? lifecycle.entries
      : null;
  const legacyEffects = Array.isArray(renderState?.effects)
    ? renderState.effects
    : null;

  const legacyById = new Map();
  if (legacyEffects) {
    for (const effect of legacyEffects) {
      if (!effect || typeof effect !== "object") {
        continue;
      }
      const id = typeof effect.id === "string" ? effect.id : null;
      if (!id) {
        continue;
      }
      if (!legacyById.has(id)) {
        legacyById.set(id, effect);
      }
    }
  }

  const consumedIds = new Set();
  if (lifecycleEntries) {
    for (const [effectId, entry] of lifecycleEntries.entries()) {
      if (!entry || typeof entry !== "object") {
        continue;
      }
      const fallback = legacyById.get(effectId) ?? null;
      const converted = contractLifecycleToEffect(entry, {
        store,
        renderState,
        fallbackEffect: fallback,
      });
      if (!converted || typeof converted !== "object") {
        continue;
      }
      if (typeof converted.id !== "string" || converted.id.length === 0) {
        converted.id = effectId;
      }
      const legacyType = resolveLifecycleEffectType(entry, converted, fallback);
      if (!legacyType) {
        continue;
      }
      converted.type = legacyType;
      markHiddenMetadata(converted, "__contractDerived", true);
      const definitionId =
        typeof entry.instance?.definitionId === "string"
          ? entry.instance.definitionId
          : null;
      if (definitionId) {
        markHiddenMetadata(converted, "__contractDefinitionId", definitionId);
      }
      addEffectToBucket(buckets, legacyType, converted);
      consumedIds.add(effectId);
    }
  }

  const endedEntries =
    lifecycle && lifecycle.endedEntries instanceof Map
      ? lifecycle.endedEntries
      : null;
  const endedVersion =
    lifecycle && typeof lifecycle.endedVersion === "number"
      ? lifecycle.endedVersion
      : null;
  const lastConsumedEndedVersion =
    store && typeof store.__consumedEndedVersion === "number"
      ? store.__consumedEndedVersion
      : null;
  if (
    endedEntries &&
    endedEntries.size > 0 &&
    endedVersion !== null &&
    endedVersion !== lastConsumedEndedVersion
  ) {
    for (const [effectId, entry] of endedEntries.entries()) {
      if (!entry || typeof entry !== "object") {
        continue;
      }
      if (consumedIds.has(effectId)) {
        continue;
      }
      const fallback = legacyById.get(effectId) ?? null;
      const converted = contractLifecycleToEffect(entry, {
        store,
        renderState,
        fallbackEffect: fallback,
      });
      if (!converted || typeof converted !== "object") {
        continue;
      }
      if (typeof converted.id !== "string" || converted.id.length === 0) {
        converted.id = effectId;
      }
      const legacyType = resolveLifecycleEffectType(entry, converted, fallback);
      if (!legacyType) {
        continue;
      }
      converted.type = legacyType;
      markHiddenMetadata(converted, "__contractDerived", true);
      const definitionId =
        typeof entry.instance?.definitionId === "string"
          ? entry.instance.definitionId
          : null;
      if (definitionId) {
        markHiddenMetadata(converted, "__contractDefinitionId", definitionId);
      }
      addEffectToBucket(buckets, legacyType, converted);
      consumedIds.add(effectId);
    }
    if (store && typeof store === "object") {
      store.__consumedEndedVersion = endedVersion;
    }
  }

  if (legacyEffects) {
    for (const effect of legacyEffects) {
      if (!effect || typeof effect !== "object") {
        continue;
      }
      const id = typeof effect.id === "string" ? effect.id : null;
      if (id && consumedIds.has(id)) {
        continue;
      }
      const type = typeof effect.type === "string" ? effect.type : null;
      if (!type) {
        continue;
      }
      addEffectToBucket(buckets, type, effect);
    }
  }

  return buckets;
}

const EMPTY_LIFECYCLE_VIEW = {
  entries: new Map(),
  lastSeqById: new Map(),
  lastBatchTick: null,
  endedEntries: new Map(),
  endedVersion: 0,
  getEntry: () => null,
};

function ensureLifecycleRenderView(store) {
  const state = peekEffectLifecycleState(store);
  if (!state) {
    store.__effectLifecycleView = EMPTY_LIFECYCLE_VIEW;
    store.__effectLifecycleViewVersion = null;
    store.__effectLifecycleViewState = null;
    return EMPTY_LIFECYCLE_VIEW;
  }

  if (
    store.__effectLifecycleView &&
    store.__effectLifecycleViewVersion === state.version &&
    store.__effectLifecycleViewState === state
  ) {
    return store.__effectLifecycleView;
  }

  const view = {
    entries: state.instances,
    lastSeqById: state.lastSeqById,
    lastBatchTick: state.lastBatchTick,
    endedEntries: state.recentlyEnded,
    endedVersion: state.recentlyEndedVersion,
    getEntry(effectId) {
      if (typeof effectId !== "string" || effectId.length === 0) {
        return null;
      }
      return state.instances.get(effectId) ?? null;
    },
  };

  store.__effectLifecycleView = view;
  store.__effectLifecycleViewVersion = state.version;
  store.__effectLifecycleViewState = state;
  return view;
}

function resolveRenderState(store) {
  const lifecycle = ensureLifecycleRenderView(store);
  const wantsPatchMode = store?.renderMode === RENDER_MODE_PATCH;
  const patchedState = wantsPatchMode ? store?.patchState?.patched : null;
  if (wantsPatchMode && isPlainObject(patchedState)) {
    return {
      mode: RENDER_MODE_PATCH,
      players: toEntriesMap(patchedState.players),
      npcs: toEntriesMap(patchedState.npcs),
      effects: toEffectArray(patchedState.effects),
      groundItems: toEntriesMap(patchedState.groundItems),
      tick: patchedState.tick,
      sequence: patchedState.sequence,
      lifecycle,
    };
  }

  return {
    mode: RENDER_MODE_SNAPSHOT,
    players: toEntriesMap(store?.players),
    npcs: toEntriesMap(store?.npcs),
    effects: Array.isArray(store?.effects) ? store.effects : [],
    groundItems: toEntriesMap(store?.groundItems),
    tick: store?.lastTick ?? null,
    sequence: null,
    lifecycle,
  };
}

function ensureEffectManager(store) {
  if (!(store.effectManager instanceof EffectManager)) {
    store.effectManager = new EffectManager();
    store.__effectTriggersRegistered = false;
  }
  ensureEffectRegistry(store);
  if (!store.__effectTriggersRegistered) {
    registerDefaultEffectTriggers(store.effectManager);
    store.__effectTriggersRegistered = true;
  }
  return store.effectManager;
}

function registerDefaultEffectTriggers(manager) {
  manager.registerTrigger("blood-splatter", handleBloodSplatterTrigger);
}

function syncEffectsByType(
  store,
  manager,
  type,
  definition,
  onUpdate,
  effectsOverride,
  options = {}
) {
  if (!manager || typeof type !== "string" || type.length === 0) {
    return;
  }
  const lifecycleView =
    options && typeof options === "object" ? options.lifecycle ?? null : null;
  const lifecycleEntries =
    lifecycleView && lifecycleView.entries instanceof Map
      ? lifecycleView.entries
      : null;
  const getLifecycleEntry =
    lifecycleView && typeof lifecycleView.getEntry === "function"
      ? lifecycleView.getEntry
      : null;
  const renderState =
    options && typeof options === "object" ? options.renderState ?? null : null;
  const effects = Array.isArray(effectsOverride)
    ? effectsOverride
    : Array.isArray(store.effects)
      ? store.effects
      : [];
  const definitionType =
    definition && typeof definition.type === "string" && definition.type.length > 0
      ? definition.type
      : null;
  const trackedType = definitionType || type;
  const tracked = manager.getTrackedInstances(trackedType);
  const crossType = trackedType !== type;
  const seen = new Set();

  for (const effect of effects) {
    if (!effect || typeof effect !== "object" || effect.type !== type) {
      continue;
    }
    const id = typeof effect.id === "string" ? effect.id : null;
    if (!id) {
      continue;
    }
    const lifecycleEntry = getLifecycleEntry
      ? getLifecycleEntry(id)
      : lifecycleEntries instanceof Map
        ? lifecycleEntries.get(id) ?? null
        : null;
    const derivedFromContract =
      lifecycleEntry && effect && effect.__contractDerived === true;
    const sourceEffect = derivedFromContract
      ? effect
      : contractLifecycleToEffect(lifecycleEntry, {
          store,
          renderState,
          fallbackEffect: effect,
        });
    if (!sourceEffect || typeof sourceEffect !== "object") {
      continue;
    }
    seen.add(id);
    let instance = tracked.get(id);
    if (!instance) {
      const spawnOptions =
        typeof definition.fromEffect === "function"
          ? definition.fromEffect(sourceEffect, store, lifecycleEntry)
          : { ...sourceEffect };
      if (!spawnOptions || typeof spawnOptions !== "object") {
        continue;
      }
      if (!spawnOptions.effectId) {
        spawnOptions.effectId = id;
      }
      instance = manager.spawn(definition, spawnOptions);
    }
    if (instance && instance.__hostEffectId !== id) {
      instance.__hostEffectId = id;
    }
    if (instance && crossType) {
      if (typeof instance.__hostEffectType !== "string") {
        instance.__hostEffectType = type;
      }
      if (instance.__hostEffectType !== type) {
        continue;
      }
    }
    if (instance) {
      if (lifecycleEntry) {
        instance.__effectLifecycleEntry = lifecycleEntry;
      } else if (instance.__effectLifecycleEntry) {
        delete instance.__effectLifecycleEntry;
      }
    }
    if (instance && typeof onUpdate === "function") {
      const updatePayload = contractLifecycleToUpdatePayload(lifecycleEntry, {
        store,
        renderState,
        fallbackEffect: sourceEffect,
      });
      onUpdate(instance, updatePayload, store, lifecycleEntry);
    }
  }

  for (const [trackedId, instance] of Array.from(tracked.entries())) {
    if (crossType) {
      if (!instance || instance.__hostEffectType !== type) {
        continue;
      }
    }
    const hostEffectId =
      instance && typeof instance.__hostEffectId === "string"
        ? instance.__hostEffectId
        : trackedId;
    if (!seen.has(hostEffectId)) {
      if (instance && instance.__effectLifecycleEntry) {
        delete instance.__effectLifecycleEntry;
      }
      manager.removeInstance(instance);
    }
  }
}

function updateFireInstanceTransform(instance, effect, store) {
  if (!instance || typeof instance !== "object" || !effect) {
    return;
  }
  const tileSize = Number.isFinite(store?.TILE_SIZE) ? store.TILE_SIZE : 40;
  const width = Number.isFinite(effect.width) ? effect.width : tileSize;
  const height = Number.isFinite(effect.height) ? effect.height : tileSize;
  const baseX = Number.isFinite(effect.x) ? effect.x : 0;
  const baseY = Number.isFinite(effect.y) ? effect.y : 0;
  const centerX = baseX + width / 2;
  const centerY = baseY + height / 2;
  const origin = instance.origin;
  if (origin && typeof origin === "object") {
    origin.x = centerX;
    origin.y = centerY;
  }
  const opts = instance.opts || {};
  const sizeScale = Number.isFinite(opts.sizeScale) ? opts.sizeScale : 1;
  const radiusX = 56 * sizeScale;
  const radiusY = 84 * sizeScale;
  const aabb = instance.aabb;
  if (aabb && typeof aabb === "object") {
    aabb.x = centerX - radiusX;
    aabb.y = centerY - radiusY;
    aabb.w = radiusX * 2;
    aabb.h = radiusY * 2;
  }
}

function getWorldDimensions(store) {
  const fallbackWidth = store.canvas?.width || store.GRID_WIDTH * store.TILE_SIZE;
  const fallbackHeight = store.canvas?.height || store.GRID_HEIGHT * store.TILE_SIZE;
  const width =
    typeof store.WORLD_WIDTH === "number" ? store.WORLD_WIDTH : fallbackWidth;
  const height =
    typeof store.WORLD_HEIGHT === "number" ? store.WORLD_HEIGHT : fallbackHeight;
  return { width, height };
}

function updateCamera(store, renderState) {
  if (!store.camera) {
    store.camera = { x: 0, y: 0, lockOnPlayer: true };
  }
  const camera = store.camera;
  const { width: worldWidth, height: worldHeight } = getWorldDimensions(store);
  const viewportWidth = store.canvas?.width || worldWidth;
  const viewportHeight = store.canvas?.height || worldHeight;

  if (camera.lockOnPlayer && store.playerId) {
    const target =
      store.displayPlayers[store.playerId] ||
      renderState.players?.[store.playerId] ||
      store.players?.[store.playerId];
    if (target) {
      camera.x = target.x - viewportWidth / 2;
      camera.y = target.y - viewportHeight / 2;
      return;
    }
  }

  camera.x = typeof camera.x === "number" ? camera.x : 0;
  camera.y = typeof camera.y === "number" ? camera.y : 0;
}

// startRenderLoop animates interpolation and draws the scene each frame.
export function startRenderLoop(store) {
  store.lastTimestamp = performance.now();

  function gameLoop(now) {
    const dt = Math.min((now - store.lastTimestamp) / 1000, 0.2);
    store.lastTimestamp = now;

    const renderState = resolveRenderState(store);
    const lerpAmount = Math.min(1, dt * store.LERP_RATE);

    const renderPlayers = renderState.players;
    Object.entries(renderPlayers).forEach(([id, player]) => {
      if (!player) {
        return;
      }
      if (!store.displayPlayers[id]) {
        store.displayPlayers[id] = { x: player.x, y: player.y };
      }
      const display = store.displayPlayers[id];
      display.x += (player.x - display.x) * lerpAmount;
      display.y += (player.y - display.y) * lerpAmount;
    });

    Object.keys(store.displayPlayers).forEach((id) => {
      if (!renderPlayers[id]) {
        delete store.displayPlayers[id];
      }
    });

    const renderNPCs = renderState.npcs;
    Object.entries(renderNPCs).forEach(([id, npc]) => {
      if (!npc) {
        return;
      }
      if (!store.displayNPCs[id]) {
        store.displayNPCs[id] = { x: npc.x, y: npc.y };
      }
      const display = store.displayNPCs[id];
      display.x += (npc.x - display.x) * lerpAmount;
      display.y += (npc.y - display.y) * lerpAmount;
    });

    Object.keys(store.displayNPCs).forEach((id) => {
      if (!renderNPCs[id]) {
        delete store.displayNPCs[id];
      }
    });

    updateCamera(store, renderState);

    drawScene(store, renderState, dt, now);
    requestAnimationFrame(gameLoop);
  }

  requestAnimationFrame(gameLoop);
}

// drawScene paints the background, obstacles, effects, and players.
function drawScene(store, renderState, frameDt, frameNow) {
  const { ctx, canvas } = store;
  ctx.fillStyle = "#020617";
  ctx.fillRect(0, 0, canvas.width, canvas.height);

  const camera = store.camera || { x: 0, y: 0 };
  const tileSize = store.TILE_SIZE || 40;
  const { width: worldWidth, height: worldHeight } = getWorldDimensions(store);
  const viewportWidth = canvas?.width || worldWidth;
  const viewportHeight = canvas?.height || worldHeight;

  ctx.save();
  ctx.translate(-camera.x, -camera.y);

  ctx.fillStyle = "#0f172a";
  ctx.fillRect(0, 0, worldWidth, worldHeight);

  const viewportLeft = camera.x;
  const viewportTop = camera.y;
  const viewportRight = viewportLeft + viewportWidth;
  const viewportBottom = viewportTop + viewportHeight;
  const startColumn = Math.floor(viewportLeft / tileSize);
  const endColumn = Math.ceil(viewportRight / tileSize);
  const startRow = Math.floor(viewportTop / tileSize);
  const endRow = Math.ceil(viewportBottom / tileSize);
  const gridLeft = startColumn * tileSize;
  const gridTop = startRow * tileSize;
  const gridRight = endColumn * tileSize;
  const gridBottom = endRow * tileSize;

  ctx.strokeStyle = "#1e293b";
  ctx.lineWidth = 1;
  for (let column = startColumn; column <= endColumn; column++) {
    const x = column * tileSize;
    ctx.beginPath();
    ctx.moveTo(x, gridTop);
    ctx.lineTo(x, gridBottom);
    ctx.stroke();
  }
  for (let row = startRow; row <= endRow; row++) {
    const y = row * tileSize;
    ctx.beginPath();
    ctx.moveTo(gridLeft, y);
    ctx.lineTo(gridRight, y);
    ctx.stroke();
  }

  store.obstacles.forEach((obstacle) => {
    drawObstacle(ctx, obstacle);
  });

  const effectPass = prepareEffectPass(
    store,
    renderState,
    frameDt,
    frameNow,
    viewportWidth,
    viewportHeight
  );

  drawEffectLayerRange(effectPass, {
    maxLayer: GROUND_EFFECT_MAX_LAYER,
    resetDrawn: true,
  });

  drawGroundItems(store, renderState);
  drawNPCs(store, renderState);

  Object.entries(store.displayPlayers).forEach(([id, position]) => {
    ctx.fillStyle = id === store.playerId ? "#38bdf8" : "#f97316";
    ctx.fillRect(
      position.x - store.PLAYER_HALF,
      position.y - store.PLAYER_HALF,
      store.PLAYER_SIZE,
      store.PLAYER_SIZE
    );

    const player = renderState.players?.[id] || store.players?.[id];
    if (player && typeof player.maxHealth === "number" && player.maxHealth > 0 && typeof player.health === "number") {
      drawHealthBar(ctx, store, position, player, id);
    }
    const facing = player && typeof player.facing === "string" ? player.facing : DEFAULT_FACING;
    const offset = FACING_OFFSETS[facing] || FACING_OFFSETS[DEFAULT_FACING];
    const indicatorLength = store.PLAYER_HALF + 6;

    ctx.save();
    ctx.strokeStyle = id === store.playerId ? "#e0f2fe" : "#ffedd5";
    ctx.lineWidth = 3;
    ctx.lineCap = "round";
    ctx.beginPath();
    ctx.moveTo(position.x, position.y);
    ctx.lineTo(
      position.x + offset.x * indicatorLength,
      position.y + offset.y * indicatorLength
    );
    ctx.stroke();
    ctx.restore();
  });

  drawEffectLayerRange(effectPass, {
    minLayer: ACTOR_OVERLAY_LAYER,
    resetDrawn: false,
  });

  ctx.restore();
}

function drawGroundItems(store, renderState) {
  const { ctx } = store;
  if (!ctx || !store || typeof store !== "object") {
    return;
  }
  const source = renderState?.groundItems;
  let items = [];
  if (Array.isArray(source)) {
    items = source;
  } else if (isPlainObject(source)) {
    items = Object.values(source);
  } else if (isPlainObject(store.groundItems)) {
    items = Object.values(store.groundItems);
  }
  if (!items || items.length === 0) {
    return;
  }
  const coinRadius = Math.max(6, Math.min(12, (store.TILE_SIZE || 40) * 0.2));
  for (const item of items) {
    if (!item || typeof item !== "object") {
      continue;
    }
    const qty = Number(item.qty);
    if (!Number.isFinite(qty) || qty <= 0) {
      continue;
    }
    const x = Number(item.x);
    const y = Number(item.y);
    if (!Number.isFinite(x) || !Number.isFinite(y)) {
      continue;
    }
    const type = typeof item.type === "string" ? item.type : "gold";
    const isGold = type === "gold";
    ctx.save();
    ctx.lineWidth = 2;
    if (isGold) {
      ctx.fillStyle = "#fbbf24";
      ctx.strokeStyle = "#f59e0b";
      ctx.beginPath();
      ctx.arc(x, y, coinRadius, 0, Math.PI * 2);
      ctx.fill();
      ctx.stroke();
    } else {
      ctx.fillStyle = "#9ca3af";
      ctx.strokeStyle = "#4b5563";
      ctx.beginPath();
      ctx.ellipse(
        x,
        y,
        coinRadius * 0.9,
        coinRadius * 0.45,
        Math.PI / 4,
        0,
        Math.PI * 2
      );
      ctx.fill();
      ctx.stroke();
      ctx.beginPath();
      ctx.moveTo(x + coinRadius * 0.6, y + coinRadius * 0.6);
      ctx.quadraticCurveTo(
        x + coinRadius * 1.1,
        y + coinRadius * 1.1,
        x + coinRadius * 1.4,
        y + coinRadius * 0.2
      );
      ctx.stroke();
    }
    ctx.fillStyle = isGold ? "#78350f" : "#111827";
    ctx.font = "10px sans-serif";
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";
    ctx.fillText(String(qty), x, y);
    ctx.restore();
  }
}

function drawNPCs(store, renderState) {
  const { ctx } = store;
  Object.entries(store.displayNPCs).forEach(([id, position]) => {
    ctx.fillStyle = "#a855f7";
    ctx.fillRect(
      position.x - store.PLAYER_HALF,
      position.y - store.PLAYER_HALF,
      store.PLAYER_SIZE,
      store.PLAYER_SIZE
    );

    const npc = renderState.npcs?.[id] || store.npcs?.[id];
    if (
      npc &&
      typeof npc.maxHealth === "number" &&
      npc.maxHealth > 0 &&
      typeof npc.health === "number"
    ) {
      drawHealthBar(ctx, store, position, npc, id);
    }

    const facing = npc && typeof npc.facing === "string" ? npc.facing : DEFAULT_FACING;
    const offset = FACING_OFFSETS[facing] || FACING_OFFSETS[DEFAULT_FACING];
    const indicatorLength = store.PLAYER_HALF + 6;

    ctx.save();
    ctx.strokeStyle = "#f5d0fe";
    ctx.lineWidth = 3;
    ctx.lineCap = "round";
    ctx.beginPath();
    ctx.moveTo(position.x, position.y);
    ctx.lineTo(
      position.x + offset.x * indicatorLength,
      position.y + offset.y * indicatorLength
    );
    ctx.stroke();
    ctx.restore();

    if (npc && typeof npc.type === "string" && npc.type.length > 0) {
      ctx.save();
      ctx.fillStyle = "#f8fafc";
      ctx.font = "10px sans-serif";
      ctx.textAlign = "center";
      ctx.textBaseline = "bottom";
      ctx.fillText(npc.type, position.x, position.y - store.PLAYER_HALF - 10);
      ctx.restore();
    }
  });
}

function drawHealthBar(ctx, store, position, player, id) {
  const maxHealth = player.maxHealth;
  const health = Math.max(0, Math.min(player.health, maxHealth));
  const ratio = maxHealth > 0 ? health / maxHealth : 0;
  const barWidth = store.PLAYER_SIZE;
  const barHeight = 4;
  const barX = position.x - store.PLAYER_HALF;
  const barY = position.y - store.PLAYER_HALF - 8;

  ctx.save();
  ctx.fillStyle = "rgba(15, 23, 42, 0.65)";
  ctx.fillRect(barX, barY, barWidth, barHeight);

  const fillWidth = Math.max(0, Math.min(barWidth, barWidth * ratio));
  if (fillWidth > 0) {
    ctx.fillStyle = id === store.playerId ? "#4ade80" : "#f87171";
    ctx.fillRect(barX, barY, fillWidth, barHeight);
  }

  ctx.strokeStyle = "rgba(15, 23, 42, 0.9)";
  ctx.strokeRect(barX - 0.5, barY - 0.5, barWidth + 1, barHeight + 1);
  ctx.restore();
}

// prepareEffectPass syncs effect instances and returns the frame context.
function prepareEffectPass(
  store,
  renderState,
  frameDt,
  frameNow,
  viewportWidth,
  viewportHeight
) {
  if (!store || !store.ctx) {
    return null;
  }

  const manager = ensureEffectManager(store);
  const lifecycle = renderState?.lifecycle || ensureLifecycleRenderView(store);
  const normalizedFrameNow = Number.isFinite(frameNow) ? frameNow : null;
  const lastPass = store.__lastEffectPass;
  if (
    normalizedFrameNow !== null &&
    typeof store.__lastEffectFrameNow === "number" &&
    store.__lastEffectFrameNow === normalizedFrameNow &&
    lastPass &&
    lastPass.manager === manager &&
    lastPass.lifecycle === lifecycle
  ) {
    return lastPass;
  }

  const triggers = drainPendingEffectTriggers(store);
  if (triggers.length > 0) {
    manager.triggerAll(triggers, { store });
  }

  const effectBuckets = collectEffectRenderBuckets(store, renderState);

  syncEffectsByType(
    store,
    manager,
    "attack",
    MeleeSwingEffectDefinition,
    undefined,
    effectBuckets.get("attack") ?? [],
    { lifecycle, renderState },
  );
  syncEffectsByType(
    store,
    manager,
    "fire",
    FireEffectDefinition,
    updateFireInstanceTransform,
    effectBuckets.get("fire") ?? [],
    { lifecycle, renderState },
  );
  syncEffectsByType(
    store,
    manager,
    "fireball",
    FireballZoneEffectDefinition,
    (instance, effect, state, lifecycleEntry) =>
      updateRectZoneInstance(instance, effect, state, lifecycleEntry),
    effectBuckets.get("fireball") ?? [],
    { lifecycle, renderState },
  );
  syncEffectsByType(
    store,
    manager,
    "blood-splatter",
    BloodSplatterDefinition,
    undefined,
    effectBuckets.get("blood-splatter") ?? [],
    { lifecycle, renderState },
  );

  const camera = store.camera || { x: 0, y: 0 };
  const safeWidth = Number.isFinite(viewportWidth)
    ? viewportWidth
    : store.canvas?.width || 0;
  const safeHeight = Number.isFinite(viewportHeight)
    ? viewportHeight
    : store.canvas?.height || 0;
  const frameContext = {
    ctx: store.ctx,
    dt: Math.max(0, frameDt || 0),
    now: Number.isFinite(frameNow) ? frameNow / 1000 : Date.now() / 1000,
    camera: {
      toScreenX: (value) => value,
      toScreenY: (value) => value,
      zoom: 1,
    },
  };

  manager.cullByAABB({
    x: camera.x,
    y: camera.y,
    w: safeWidth,
    h: safeHeight,
  });
  manager.updateAll(frameContext);
  manager.collectDecals(frameContext.now);
  mirrorEffectInstances(store, manager);

  const effectPass = { manager, frameContext, lifecycle };
  if (normalizedFrameNow !== null) {
    store.__lastEffectFrameNow = normalizedFrameNow;
  } else {
    delete store.__lastEffectFrameNow;
  }
  store.__lastEffectPass = effectPass;
  return effectPass;
}

function drawEffectLayerRange(
  effectPass,
  {
    minLayer = Number.NEGATIVE_INFINITY,
    maxLayer = Number.POSITIVE_INFINITY,
    resetDrawn = true,
  } = {}
) {
  if (!effectPass || !effectPass.manager || !effectPass.frameContext) {
    return;
  }

  const { manager, frameContext } = effectPass;
  if (typeof manager.drawLayerRange === "function") {
    manager.drawLayerRange(frameContext, minLayer, maxLayer, { resetDrawn });
    return;
  }

  if (resetDrawn) {
    manager.drawAll(frameContext);
  }
}

function drainPendingEffectTriggers(store) {
  if (!Array.isArray(store.pendingEffectTriggers) || store.pendingEffectTriggers.length === 0) {
    return [];
  }
  const drained = store.pendingEffectTriggers.slice();
  store.pendingEffectTriggers.length = 0;
  return drained;
}

function handleBloodSplatterTrigger({ manager, trigger, context }) {
  const store = context?.store;
  if (!manager || !trigger || !store) {
    return;
  }

  const width = Number.isFinite(trigger.width) ? trigger.width : store.TILE_SIZE || 40;
  const height = Number.isFinite(trigger.height) ? trigger.height : store.TILE_SIZE || 40;
  const baseX = Number.isFinite(trigger.x) ? trigger.x : 0;
  const baseY = Number.isFinite(trigger.y) ? trigger.y : 0;
  const centerX = baseX + width / 2;
  const centerY = baseY + height / 2;

  const params =
    trigger && typeof trigger === "object" && typeof trigger.params === "object"
      ? trigger.params
      : null;
  const readNumber = (key, fallback) => {
    if (!params || typeof params !== "object") {
      return fallback;
    }
    const value = params[key];
    return Number.isFinite(value) ? value : fallback;
  };
  const readColor = (key, fallback) => {
    if (!params || typeof params !== "object") {
      return fallback;
    }
    const value = params[key];
    return typeof value === "string" && value ? value : fallback;
  };

  const dropletRadius = readNumber("dropletRadius", 3);
  const drag = readNumber("drag", 0.92);
  const speed = readNumber("speed", 3);
  const spawnInterval = readNumber("spawnInterval", 1.1);
  const minDroplets = readNumber("minDroplets", 4);
  const maxDroplets = readNumber("maxDroplets", 33);
  const minStainRadius = readNumber("minStainRadius", 4);
  const maxStainRadius = readNumber("maxStainRadius", 6);
  const maxStains = readNumber("maxStains", 140);
  const maxBursts = readNumber("maxBursts", 0);
  const midColor = readColor("midColor", "#7a0e12");
  const darkColor = readColor("darkColor", "#4a090b");

  manager.spawn(BloodSplatterDefinition, {
    x: centerX,
    y: centerY,
    colors: [midColor, darkColor],
    drag,
    dropletRadius,
    maxBursts,
    maxDroplets,
    maxStainRadius,
    maxStains,
    minDroplets,
    minStainRadius,
    spawnInterval,
    speed,
  });
}

// drawObstacle picks the correct renderer for each obstacle type.
function drawObstacle(ctx, obstacle) {
  if (!obstacle || typeof obstacle !== "object") {
    return;
  }

  const normalizedType = normalizeObstacleType(obstacle.type);
  if (normalizedType === "gold-ore") {
    drawGoldOreObstacle(ctx, obstacle);
    return;
  }
  if (normalizedType === "lava") {
    drawLavaObstacle(ctx, obstacle);
    return;
  }

  drawDefaultObstacle(ctx, obstacle);
}

// drawDefaultObstacle paints a simple stone block.
function drawDefaultObstacle(ctx, obstacle) {
  const { x, y, width, height } = obstacle;
  ctx.save();
  ctx.fillStyle = "#334155";
  ctx.fillRect(x, y, width, height);
  ctx.strokeStyle = "#475569";
  ctx.strokeRect(x, y, width, height);
  ctx.restore();
}

// drawGoldOreObstacle renders a gold ore node with deterministic nuggets.
function drawGoldOreObstacle(ctx, obstacle) {
  const { x, y, width, height } = obstacle;
  ctx.save();

  const gradient = ctx.createLinearGradient(x, y, x + width, y + height);
  gradient.addColorStop(0, "#3f3a2d");
  gradient.addColorStop(1, "#2f2a22");
  ctx.fillStyle = gradient;
  ctx.fillRect(x, y, width, height);

  ctx.strokeStyle = "#b09155";
  ctx.lineWidth = 2;
  ctx.strokeRect(x + 0.5, y + 0.5, width - 1, height - 1);

  const rng = createObstacleRng(obstacle);
  const veinCount = Math.max(2, Math.round((width + height) / 70));
  const veinThickness = Math.max(1.2, Math.min(width, height) * 0.05);
  const nuggetColors = ["#fde047", "#facc15", "#fef08a"];

  for (let veinIndex = 0; veinIndex < veinCount; veinIndex++) {
    const startX = clampValue(x + rng() * width, x, x + width);
    const startY = clampValue(y + rng() * height, y, y + height);
    const segmentCount = 2 + Math.floor(rng() * 2);
    const segments = [];

    ctx.beginPath();
    ctx.moveTo(startX, startY);

    let lastX = startX;
    let lastY = startY;
    for (let segmentIndex = 0; segmentIndex < segmentCount; segmentIndex++) {
      const controlX = clampValue(
        lastX + (rng() - 0.5) * width * 0.6,
        x,
        x + width
      );
      const controlY = clampValue(
        lastY + (rng() - 0.5) * height * 0.6,
        y,
        y + height
      );
      const endX = clampValue(
        controlX + (rng() - 0.5) * width * 0.4,
        x,
        x + width
      );
      const endY = clampValue(
        controlY + (rng() - 0.5) * height * 0.4,
        y,
        y + height
      );

      ctx.quadraticCurveTo(controlX, controlY, endX, endY);
      segments.push({
        startX: lastX,
        startY: lastY,
        controlX,
        controlY,
        endX,
        endY,
      });
      lastX = endX;
      lastY = endY;
    }

    ctx.save();
    ctx.strokeStyle = "rgba(250, 204, 21, 0.75)";
    ctx.lineWidth = veinThickness;
    ctx.lineCap = "round";
    ctx.lineJoin = "round";
    ctx.stroke();
    ctx.restore();

    segments.forEach((segment) => {
      const nuggetAlongSegment = 1 + Math.floor(rng() * 2);
      for (let nugget = 0; nugget < nuggetAlongSegment; nugget++) {
        const t = 0.15 + rng() * 0.7;
        const invT = 1 - t;
        const pointX =
          invT * invT * segment.startX +
          2 * invT * t * segment.controlX +
          t * t * segment.endX;
        const pointY =
          invT * invT * segment.startY +
          2 * invT * t * segment.controlY +
          t * t * segment.endY;

        const offsetRadius = Math.min(width, height) * 0.05 * rng();
        const angle = rng() * Math.PI * 2;
        const nuggetX = clampValue(
          pointX + Math.cos(angle) * offsetRadius,
          x,
          x + width
        );
        const nuggetY = clampValue(
          pointY + Math.sin(angle) * offsetRadius,
          y,
          y + height
        );

        const radius = Math.max(1.2, Math.min(width, height) * (0.015 + rng() * 0.025));
        ctx.beginPath();
        ctx.arc(nuggetX, nuggetY, radius, 0, Math.PI * 2);
        const color = nuggetColors[(veinIndex + nugget) % nuggetColors.length];
        ctx.fillStyle = color;
        ctx.fill();

        ctx.lineWidth = 0.8;
        ctx.strokeStyle = "rgba(253, 224, 71, 0.5)";
        ctx.stroke();
      }
    });
  }

  ctx.restore();
}

// drawLavaObstacle renders a glowing lava pool that still allows traversal.
function drawLavaObstacle(ctx, obstacle) {
  const { x, y, width, height } = obstacle;
  ctx.save();

  const centerX = x + width / 2;
  const centerY = y + height / 2;
  const innerRadius = Math.min(width, height) * 0.2;
  const outerRadius = Math.max(width, height) * 0.8;
  const gradient = ctx.createRadialGradient(
    centerX,
    centerY,
    innerRadius,
    centerX,
    centerY,
    outerRadius
  );
  gradient.addColorStop(0, "#fde68a");
  gradient.addColorStop(0.4, "#fb923c");
  gradient.addColorStop(0.75, "#ea580c");
  gradient.addColorStop(1, "#7f1d1d");
  ctx.fillStyle = gradient;
  ctx.fillRect(x, y, width, height);

  const rng = createObstacleRng(obstacle);
  const bubbleCount = Math.max(4, Math.round((width + height) / 30));
  for (let i = 0; i < bubbleCount; i++) {
    const bubbleX = clampValue(x + rng() * width, x + 6, x + width - 6);
    const bubbleY = clampValue(y + rng() * height, y + 6, y + height - 6);
    const bubbleRadius = Math.max(2.5, Math.min(width, height) * (0.04 + rng() * 0.05));

    ctx.beginPath();
    ctx.arc(bubbleX, bubbleY, bubbleRadius, 0, Math.PI * 2);
    ctx.fillStyle = "rgba(254, 215, 170, 0.9)";
    ctx.fill();

    ctx.lineWidth = 1.2;
    ctx.strokeStyle = "rgba(249, 115, 22, 0.7)";
    ctx.stroke();
  }

  ctx.strokeStyle = "rgba(127, 29, 29, 0.95)";
  ctx.lineWidth = 2;
  ctx.strokeRect(x + 0.5, y + 0.5, width - 1, height - 1);

  ctx.restore();
}

// createObstacleRng returns a deterministic RNG per obstacle for visuals.
function createObstacleRng(obstacle) {
  const seedSource =
    (typeof obstacle.id === "string" && obstacle.id) ||
    (typeof obstacle.type === "string" && obstacle.type) ||
    `${obstacle.x},${obstacle.y},${obstacle.width},${obstacle.height}`;

  let seed = 0;
  for (let i = 0; i < seedSource.length; i++) {
    seed = (seed * 31 + seedSource.charCodeAt(i)) | 0;
  }
  seed ^= seed >>> 16;
  if (seed === 0) {
    seed = 0x9e3779b9;
  }

  return function rng() {
    seed = Math.imul(seed ^ (seed >>> 15), 1 | seed);
    seed ^= seed + Math.imul(seed ^ (seed >>> 7), 61 | seed);
    return ((seed ^ (seed >>> 14)) >>> 0) / 4294967296;
  };
}

// clampValue bounds a number inside the provided range.
function clampValue(value, min, max) {
  if (value < min) return min;
  if (value > max) return max;
  return value;
}

// normalizeObstacleType standardizes obstacle type strings used for styling.
function normalizeObstacleType(rawType) {
  if (typeof rawType !== "string") {
    return "";
  }
  const trimmed = rawType.trim().toLowerCase();
  if (trimmed === "goldore" || trimmed === "gold_ore" || trimmed === "gold ore") {
    return "gold-ore";
  }
  return trimmed;
}
