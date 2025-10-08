import { EffectManager } from "./js-effects/manager.js";
import { EffectLayer } from "./js-effects/types.js";
import { MeleeSwingEffectDefinition } from "./js-effects/effects/meleeSwing.js";
import { BloodSplatterDefinition } from "./js-effects/effects/bloodSplatter.js";
import { FireEffectDefinition } from "./js-effects/effects/fire.js";

/**
 * render.js bridges authoritative snapshots to the EffectManager runtime.
 * It mirrors simulation state into the shared manager, forwards triggers,
 * and relies on the runtime to own effect lifecycles end-to-end.
 */

const DEFAULT_FACING = "down";
const FACING_OFFSETS = {
  up: { x: 0, y: -1 },
  down: { x: 0, y: 1 },
  left: { x: -1, y: 0 },
  right: { x: 1, y: 0 },
};

function ensureEffectRuntime(store) {
  if (!store.effectManager) {
    store.effectManager = new EffectManager();
    store.effectTriggerHandlersRegistered = false;
  }
  if (!store.effectTriggerHandlersRegistered) {
    registerEffectTriggers(store, store.effectManager);
    store.effectTriggerHandlersRegistered = true;
  }
  return store.effectManager;
}

function registerEffectTriggers(store, manager) {
  manager.registerTrigger("blood-splatter", ({ manager: mgr, trigger }) => {
    spawnBloodSplatter(store, mgr, trigger);
  });
}

const SPAWN_OPTION_OVERRIDES = {
  attack: createMeleeSpawnOverrides,
  fire: createFireSpawnOverrides,
  fireball: createFireballSpawnOverrides,
};

function syncEffectsByType(store, manager, type, definition, onUpdate) {
  if (!store || !manager || typeof type !== "string" || !definition) {
    return;
  }

  const effects = Array.isArray(store.effects) ? store.effects : [];
  const seen = new Set();

  for (const effect of effects) {
    if (!effect || typeof effect !== "object") {
      continue;
    }
    if (effect.type !== type) {
      continue;
    }
    const id = typeof effect.id === "string" ? effect.id : null;
    if (!id) {
      continue;
    }

    seen.add(id);

    let instance = manager.getInstanceById(id);
    const overridesFactory = SPAWN_OPTION_OVERRIDES[type];
    const baseOptions = createBaseSpawnOptions(effect, store, id);
    const overrides = typeof overridesFactory === "function" ? overridesFactory(effect, store) : null;
    const spawnOptions = overrides ? { ...baseOptions, ...overrides } : baseOptions;
    const isNew = !instance;

    if (!instance) {
      instance = manager.spawn(definition, spawnOptions);
    }

    if (instance && onUpdate) {
      onUpdate(instance, effect, store, isNew);
    }
  }

  const active = manager.getInstancesByType(definition.type);
  for (const instance of active) {
    if (!seen.has(instance.id)) {
      manager.removeInstance(instance);
    }
  }
}

function createBaseSpawnOptions(effect, store, id) {
  const x = Number.isFinite(effect?.x) ? effect.x : 0;
  const y = Number.isFinite(effect?.y) ? effect.y : 0;
  const width = Number.isFinite(effect?.width) ? effect.width : store.TILE_SIZE || 40;
  const height = Number.isFinite(effect?.height) ? effect.height : store.TILE_SIZE || 40;
  const duration = Number.isFinite(effect?.duration) ? effect.duration : null;

  const options = { effectId: id, x, y };
  if (Number.isFinite(width)) {
    options.width = width;
  }
  if (Number.isFinite(height)) {
    options.height = height;
  }
  if (Number.isFinite(duration)) {
    options.duration = Math.max(0.05, duration / 1000);
  }
  return options;
}

function createMeleeSpawnOverrides(effect, store) {
  const width = Number.isFinite(effect?.width) ? effect.width : store.TILE_SIZE || 40;
  const height = Number.isFinite(effect?.height) ? effect.height : store.TILE_SIZE || 40;
  const duration = Number.isFinite(effect?.duration) ? effect.duration : 150;

  const safeSize = Math.max(1, Math.min(width, height));
  const strokeWidth = Math.max(2, Math.min(4, safeSize * 0.08));
  const innerInset = Math.max(3, safeSize * 0.22);

  return {
    width,
    height,
    duration: Math.max(0.05, duration / 1000 + 0.05),
    strokeWidth,
    innerInset,
  };
}

function createFireSpawnOverrides(effect, store) {
  const width = Number.isFinite(effect?.width) ? effect.width : store.TILE_SIZE || 40;
  const height = Number.isFinite(effect?.height) ? effect.height : store.TILE_SIZE || 40;
  const baseX = Number.isFinite(effect?.x) ? effect.x : 0;
  const baseY = Number.isFinite(effect?.y) ? effect.y : 0;
  const centerX = baseX + width / 2;
  const centerY = baseY + height / 2;

  return {
    x: centerX,
    y: centerY,
    additive: true,
    concentration: 0.25,
    emberAlpha: 1,
    emberPalette: [
      "rgba(255, 220, 150, 1.0)",
      "rgba(255, 180, 60, 1.0)",
      "rgba(255, 245, 200, 1.0)",
    ],
    embersPerBurst: 24,
    jitter: 22.5,
    lifeScale: 1.1,
    riseSpeed: 35,
    sizeScale: 1.3,
    spawnInterval: 0.06,
    spawnRadius: 15.5,
    swirl: 0.5,
    windX: 0,
  };
}

function createFireballSpawnOverrides(effect, store) {
  const width = Number.isFinite(effect?.width) ? effect.width : store.TILE_SIZE || 32;
  const height = Number.isFinite(effect?.height) ? effect.height : store.TILE_SIZE || 32;
  const baseX = Number.isFinite(effect?.x) ? effect.x : 0;
  const baseY = Number.isFinite(effect?.y) ? effect.y : 0;

  return {
    x: baseX + width / 2,
    y: baseY + height / 2,
    width,
    height,
    colors: [
      "rgba(253, 230, 138, 0.95)",
      "rgba(251, 191, 36, 0.85)",
      "rgba(194, 65, 12, 0.45)",
    ],
  };
}

function updateMeleeSwingInstance(instance, effect, store) {
  if (!instance || typeof instance !== "object") {
    return;
  }

  const width = Number.isFinite(effect?.width) ? effect.width : store.TILE_SIZE || 40;
  const height = Number.isFinite(effect?.height) ? effect.height : store.TILE_SIZE || 40;
  const x = Number.isFinite(effect?.x) ? effect.x : 0;
  const y = Number.isFinite(effect?.y) ? effect.y : 0;
  const duration = Number.isFinite(effect?.duration) ? effect.duration : 150;
  const safeSize = Math.max(1, Math.min(width, height));
  const strokeWidth = Math.max(2, Math.min(4, safeSize * 0.08));
  const innerInset = Math.max(3, safeSize * 0.22);

  if (instance.origin) {
    instance.origin.x = x;
    instance.origin.y = y;
  }
  if (instance.aabb) {
    instance.aabb.x = x;
    instance.aabb.y = y;
    instance.aabb.w = width;
    instance.aabb.h = height;
  }
  if (instance.opts && typeof instance.opts === "object") {
    instance.opts.width = width;
    instance.opts.height = height;
    instance.opts.duration = Math.max(0.05, duration / 1000 + 0.05);
    instance.opts.strokeWidth = strokeWidth;
    instance.opts.innerInset = innerInset;
  }
}

function updateFireInstanceTransform(instance, effect, store) {
  if (!instance || typeof instance !== "object") {
    return;
  }

  const width = Number.isFinite(effect?.width) ? effect.width : store.TILE_SIZE || 40;
  const height = Number.isFinite(effect?.height) ? effect.height : store.TILE_SIZE || 40;
  const baseX = Number.isFinite(effect?.x) ? effect.x : 0;
  const baseY = Number.isFinite(effect?.y) ? effect.y : 0;
  const centerX = baseX + width / 2;
  const centerY = baseY + height / 2;

  if (typeof instance.setCenter === "function") {
    instance.setCenter(centerX, centerY);
  } else if (instance.origin) {
    instance.origin.x = centerX;
    instance.origin.y = centerY;
  }

  const scale = Math.max(0.6, Math.max(width, height) / 64);
  if (typeof instance.setSizeScale === "function") {
    instance.setSizeScale(scale);
  } else if (instance.opts && typeof instance.opts === "object") {
    instance.opts.sizeScale = scale;
  }

  if (instance.aabb) {
    const radiusX = 56 * (instance.opts?.sizeScale ?? scale);
    const radiusY = 84 * (instance.opts?.sizeScale ?? scale);
    instance.aabb.x = centerX - radiusX;
    instance.aabb.y = centerY - radiusY;
    instance.aabb.w = radiusX * 2;
    instance.aabb.h = radiusY * 2;
  }
}

function updateFireballInstance(instance, effect) {
  if (!instance || typeof instance !== "object") {
    return;
  }
  if (typeof instance.syncFromEffect === "function") {
    instance.syncFromEffect(effect);
    return;
  }
  const width = Number.isFinite(effect?.width) ? effect.width : 24;
  const height = Number.isFinite(effect?.height) ? effect.height : 24;
  const baseX = Number.isFinite(effect?.x) ? effect.x : 0;
  const baseY = Number.isFinite(effect?.y) ? effect.y : 0;
  const centerX = baseX + width / 2;
  const centerY = baseY + height / 2;
  if (instance.position) {
    instance.position.x = centerX;
    instance.position.y = centerY;
  }
  if (instance.aabb) {
    instance.aabb.x = centerX - width / 2;
    instance.aabb.y = centerY - height / 2;
    instance.aabb.w = width;
    instance.aabb.h = height;
  }
}

class FireballProjectileInstance {
  constructor(opts) {
    this.id =
      typeof opts.effectId === "string" && opts.effectId
        ? opts.effectId
        : `fireball-${Math.random().toString(36).slice(2)}`;
    this.type = FireballProjectileEffectDefinition.type;
    this.layer = EffectLayer.ActorOverlay;
    this.sublayer = 1;
    this.kind = "loop";
    this.finished = false;
    this.elapsed = 0;
    this.glowPhase = 0;
    this.position = { x: opts.x, y: opts.y };
    this.radiusX = Math.max(6, (Number.isFinite(opts.width) ? opts.width : 24) / 2);
    this.radiusY = Math.max(6, (Number.isFinite(opts.height) ? opts.height : 24) / 2);
    this.colors = Array.isArray(opts.colors)
      ? opts.colors
      : ["rgba(253, 230, 138, 0.95)", "rgba(251, 191, 36, 0.85)", "rgba(194, 65, 12, 0.45)"];
    this.aabb = { x: 0, y: 0, w: 0, h: 0 };
    this.updateAABB();
  }

  isAlive() {
    return !this.finished;
  }

  getAABB() {
    return this.aabb;
  }

  update(frame) {
    const dt = Math.max(0, frame.dt || 0);
    this.elapsed += dt;
    this.glowPhase += dt * 6;
  }

  draw(frame) {
    const { ctx, camera } = frame;
    const screenX = camera.toScreenX(this.position.x);
    const screenY = camera.toScreenY(this.position.y);
    const radiusX = this.radiusX * camera.zoom;
    const radiusY = this.radiusY * camera.zoom;
    const outerRadius = Math.max(radiusX, radiusY) * (1.05 + 0.1 * Math.sin(this.glowPhase));

    ctx.save();
    ctx.translate(screenX, screenY);
    const gradient = ctx.createRadialGradient(0, 0, outerRadius * 0.1, 0, 0, outerRadius);
    gradient.addColorStop(0, this.colors[0] ?? "rgba(253, 230, 138, 0.95)");
    gradient.addColorStop(0.6, this.colors[1] ?? "rgba(251, 191, 36, 0.85)");
    gradient.addColorStop(1, this.colors[2] ?? "rgba(194, 65, 12, 0.45)");
    ctx.fillStyle = gradient;
    ctx.beginPath();
    ctx.ellipse(0, 0, outerRadius, outerRadius * (radiusY / Math.max(1, radiusX)), 0, 0, Math.PI * 2);
    ctx.fill();
    ctx.restore();
  }

  dispose() {}

  syncFromEffect(effect) {
    const width = Number.isFinite(effect?.width) ? effect.width : this.radiusX * 2;
    const height = Number.isFinite(effect?.height) ? effect.height : this.radiusY * 2;
    const baseX = Number.isFinite(effect?.x) ? effect.x : this.position.x - width / 2;
    const baseY = Number.isFinite(effect?.y) ? effect.y : this.position.y - height / 2;
    this.position.x = baseX + width / 2;
    this.position.y = baseY + height / 2;
    this.radiusX = Math.max(6, width / 2);
    this.radiusY = Math.max(6, height / 2);
    this.updateAABB();
  }

  updateAABB() {
    this.aabb.x = this.position.x - this.radiusX;
    this.aabb.y = this.position.y - this.radiusY;
    this.aabb.w = this.radiusX * 2;
    this.aabb.h = this.radiusY * 2;
  }
}

const FireballProjectileEffectDefinition = {
  type: "fireball",
  defaults: {
    width: 24,
    height: 24,
    colors: [
      "rgba(253, 230, 138, 0.95)",
      "rgba(251, 191, 36, 0.85)",
      "rgba(194, 65, 12, 0.45)",
    ],
  },
  create: (opts) => new FireballProjectileInstance(opts),
};

function spawnBloodSplatter(store, manager, trigger) {
  if (!trigger || typeof trigger !== "object") {
    return;
  }

  const width = Number.isFinite(trigger.width) ? trigger.width : store.TILE_SIZE || 40;
  const height = Number.isFinite(trigger.height) ? trigger.height : store.TILE_SIZE || 40;
  const baseX = Number.isFinite(trigger.x) ? trigger.x : 0;
  const baseY = Number.isFinite(trigger.y) ? trigger.y : 0;
  const centerX = baseX + width / 2;
  const centerY = baseY + height / 2;

  const params = trigger && typeof trigger.params === "object" ? trigger.params : null;
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

  manager.spawn(BloodSplatterDefinition, {
    x: centerX,
    y: centerY,
    colors: [readColor("midColor", "#7a0e12"), readColor("darkColor", "#4a090b")],
    drag: readNumber("drag", 0.92),
    dropletRadius: readNumber("dropletRadius", 3),
    maxBursts: readNumber("maxBursts", 0),
    maxDroplets: readNumber("maxDroplets", 33),
    maxStainRadius: readNumber("maxStainRadius", 6),
    maxStains: readNumber("maxStains", 140),
    minDroplets: readNumber("minDroplets", 4),
    minStainRadius: readNumber("minStainRadius", 4),
    spawnInterval: readNumber("spawnInterval", 1.1),
    speed: readNumber("speed", 3),
  });
}

const EFFECT_BINDINGS = [
  { type: "attack", definition: MeleeSwingEffectDefinition, onUpdate: updateMeleeSwingInstance },
  { type: "fire", definition: FireEffectDefinition, onUpdate: updateFireInstanceTransform },
  { type: "fireball", definition: FireballProjectileEffectDefinition, onUpdate: updateFireballInstance },
];


function getWorldDimensions(store) {
  const fallbackWidth = store.canvas?.width || store.GRID_WIDTH * store.TILE_SIZE;
  const fallbackHeight = store.canvas?.height || store.GRID_HEIGHT * store.TILE_SIZE;
  const width =
    typeof store.WORLD_WIDTH === "number" ? store.WORLD_WIDTH : fallbackWidth;
  const height =
    typeof store.WORLD_HEIGHT === "number" ? store.WORLD_HEIGHT : fallbackHeight;
  return { width, height };
}

function updateCamera(store) {
  if (!store.camera) {
    store.camera = { x: 0, y: 0, lockOnPlayer: true };
  }
  const camera = store.camera;
  const { width: worldWidth, height: worldHeight } = getWorldDimensions(store);
  const viewportWidth = store.canvas?.width || worldWidth;
  const viewportHeight = store.canvas?.height || worldHeight;

  if (camera.lockOnPlayer && store.playerId) {
    const target =
      store.displayPlayers[store.playerId] || store.players[store.playerId];
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

    const lerpAmount = Math.min(1, dt * store.LERP_RATE);
    Object.entries(store.players).forEach(([id, player]) => {
      if (!store.displayPlayers[id]) {
        store.displayPlayers[id] = { x: player.x, y: player.y };
      }
      const display = store.displayPlayers[id];
      display.x += (player.x - display.x) * lerpAmount;
      display.y += (player.y - display.y) * lerpAmount;
    });

    Object.keys(store.displayPlayers).forEach((id) => {
      if (!store.players[id]) {
        delete store.displayPlayers[id];
      }
    });

    Object.entries(store.npcs).forEach(([id, npc]) => {
      if (!store.displayNPCs[id]) {
        store.displayNPCs[id] = { x: npc.x, y: npc.y };
      }
      const display = store.displayNPCs[id];
      display.x += (npc.x - display.x) * lerpAmount;
      display.y += (npc.y - display.y) * lerpAmount;
    });

    Object.keys(store.displayNPCs).forEach((id) => {
      if (!store.npcs[id]) {
        delete store.displayNPCs[id];
      }
    });

    updateCamera(store);

    drawScene(store, dt, now);
    requestAnimationFrame(gameLoop);
  }

  requestAnimationFrame(gameLoop);
}

// drawScene paints the background, obstacles, effects, and players.
function drawScene(store, frameDt, frameNow) {
  const { ctx, canvas } = store;
  ctx.fillStyle = "#0f172a";
  ctx.fillRect(0, 0, canvas.width, canvas.height);

  const camera = store.camera || { x: 0, y: 0 };
  const tileSize = store.TILE_SIZE || 40;
  const { width: worldWidth, height: worldHeight } = getWorldDimensions(store);
  const viewportWidth = canvas?.width || worldWidth;
  const viewportHeight = canvas?.height || worldHeight;

  ctx.save();
  ctx.translate(-camera.x, -camera.y);

  ctx.strokeStyle = "#1e293b";
  ctx.lineWidth = 1;
  const columnCount = Math.ceil(worldWidth / tileSize);
  for (let column = 0; column <= columnCount; column++) {
    const x = Math.min(column * tileSize, worldWidth);
    ctx.beginPath();
    ctx.moveTo(x, 0);
    ctx.lineTo(x, worldHeight);
    ctx.stroke();
  }
  const rowCount = Math.ceil(worldHeight / tileSize);
  for (let row = 0; row <= rowCount; row++) {
    const y = Math.min(row * tileSize, worldHeight);
    ctx.beginPath();
    ctx.moveTo(0, y);
    ctx.lineTo(worldWidth, y);
    ctx.stroke();
  }

  store.obstacles.forEach((obstacle) => {
    drawObstacle(ctx, obstacle);
  });

  drawEffects(store, frameDt, frameNow, viewportWidth, viewportHeight);

  drawNPCs(store);

  Object.entries(store.displayPlayers).forEach(([id, position]) => {
    ctx.fillStyle = id === store.playerId ? "#38bdf8" : "#f97316";
    ctx.fillRect(
      position.x - store.PLAYER_HALF,
      position.y - store.PLAYER_HALF,
      store.PLAYER_SIZE,
      store.PLAYER_SIZE
    );

    const player = store.players[id];
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

  ctx.restore();
}

function drawNPCs(store) {
  const { ctx } = store;
  Object.entries(store.displayNPCs).forEach(([id, position]) => {
    ctx.fillStyle = "#a855f7";
    ctx.fillRect(
      position.x - store.PLAYER_HALF,
      position.y - store.PLAYER_HALF,
      store.PLAYER_SIZE,
      store.PLAYER_SIZE
    );

    const npc = store.npcs[id];
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

// drawEffects mirrors snapshot state into the shared EffectManager and draws active instances.
function drawEffects(store, frameDt, frameNow, viewportWidth, viewportHeight) {
  const { ctx } = store;
  const manager = ensureEffectRuntime(store);
  if (!manager || !ctx) {
    return;
  }

  const nowSeconds = Number.isFinite(frameNow) ? frameNow / 1000 : Date.now() / 1000;
  const triggers = drainPendingEffectTriggers(store);
  manager.triggerAll(triggers, { store });

  for (const binding of EFFECT_BINDINGS) {
    syncEffectsByType(store, manager, binding.type, binding.definition, binding.onUpdate);
  }

  const camera = store.camera || { x: 0, y: 0 };
  const safeWidth = Number.isFinite(viewportWidth)
    ? viewportWidth
    : store.canvas?.width || 0;
  const safeHeight = Number.isFinite(viewportHeight)
    ? viewportHeight
    : store.canvas?.height || 0;

  const frameContext = {
    ctx,
    dt: Math.max(0, frameDt || 0),
    now: nowSeconds,
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
  manager.drawAll(frameContext);
  manager.collectDecals(nowSeconds);
}

function drainPendingEffectTriggers(store) {
  if (!Array.isArray(store.pendingEffectTriggers) || store.pendingEffectTriggers.length === 0) {
    return [];
  }
  const drained = store.pendingEffectTriggers.slice();
  store.pendingEffectTriggers.length = 0;
  return drained;
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
