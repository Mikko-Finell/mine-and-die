import { computeRtt, createHeartbeat } from "./heartbeat.js";
import { createPatchState, updatePatchState } from "./patches.js";
import {
  applyEffectLifecycleBatch,
  resetEffectLifecycleState,
  updateEphemeralEffectLifecycleEntries,
} from "./effect-lifecycle.js";

const HEARTBEAT_INTERVAL = 2000;
const DEFAULT_FACING = "down";
export const PROTOCOL_VERSION = 1;
export const DEFAULT_WORLD_SEED = "prototype";
const DEFAULT_OBSTACLE_COUNT = 2;
const DEFAULT_GOLD_MINE_COUNT = 1;
const DEFAULT_GOBLIN_COUNT = 2;
const DEFAULT_RAT_COUNT = 1;
const DEFAULT_NPC_COUNT = DEFAULT_GOBLIN_COUNT + DEFAULT_RAT_COUNT;
const DEFAULT_LAVA_COUNT = 3;
export const DEFAULT_WORLD_WIDTH = 2400;
export const DEFAULT_WORLD_HEIGHT = 1800;
const VALID_FACINGS = new Set(["up", "down", "left", "right"]);
const KEYFRAME_RETRY_MAX_ATTEMPTS = 3;
const KEYFRAME_RETRY_BASE_DELAY = 200;
const KEYFRAME_RETRY_MAX_STEP = 2000;
const KEYFRAME_RETRY_TICK_INTERVAL = 100;
const heartbeatControllers = new WeakMap();
const unknownEffectUpdateHandlers = new WeakMap();

function isBloodSplatterIdentifier(value) {
  if (typeof value !== "string" || value.length === 0) {
    return false;
  }
  return value.toLowerCase() === "blood-splatter";
}

function isFireIdentifier(value) {
  if (typeof value !== "string" || value.length === 0) {
    return false;
  }
  return value.toLowerCase() === "fire";
}

function isAttackIdentifier(value) {
  if (typeof value !== "string" || value.length === 0) {
    return false;
  }
  return value.toLowerCase() === "attack";
}

function isMeleeSwingIdentifier(value) {
  if (typeof value !== "string" || value.length === 0) {
    return false;
  }
  return value.toLowerCase() === "melee-swing";
}

function collectEffectSpawnEvents(payload) {
  if (!payload || typeof payload !== "object") {
    return [];
  }
  if (Array.isArray(payload.effect_spawned)) {
    return payload.effect_spawned;
  }
  if (Array.isArray(payload.effectSpawned)) {
    return payload.effectSpawned;
  }
  return [];
}

function isBloodSplatterSpawn(spawn) {
  if (!spawn || typeof spawn !== "object") {
    return false;
  }
  const candidates = [];
  if (typeof spawn.definitionId === "string") {
    candidates.push(spawn.definitionId);
  }
  if (typeof spawn.type === "string") {
    candidates.push(spawn.type);
  }
  if (typeof spawn.typeId === "string") {
    candidates.push(spawn.typeId);
  }
  const spawnDefinition =
    spawn.definition && typeof spawn.definition === "object" ? spawn.definition : null;
  if (spawnDefinition) {
    if (typeof spawnDefinition.type === "string") {
      candidates.push(spawnDefinition.type);
    }
    if (typeof spawnDefinition.typeId === "string") {
      candidates.push(spawnDefinition.typeId);
    }
  }
  const instance = spawn.instance && typeof spawn.instance === "object" ? spawn.instance : null;
  if (instance) {
    if (typeof instance.definitionId === "string") {
      candidates.push(instance.definitionId);
    }
    if (typeof instance.type === "string") {
      candidates.push(instance.type);
    }
    if (typeof instance.typeId === "string") {
      candidates.push(instance.typeId);
    }
    const instanceDefinition =
      instance.definition && typeof instance.definition === "object"
        ? instance.definition
        : null;
    if (instanceDefinition) {
      if (typeof instanceDefinition.type === "string") {
        candidates.push(instanceDefinition.type);
      }
      if (typeof instanceDefinition.typeId === "string") {
        candidates.push(instanceDefinition.typeId);
      }
    }
  }
  return candidates.some((candidate) => isBloodSplatterIdentifier(candidate));
}

function isFireSpawn(spawn) {
  if (!spawn || typeof spawn !== "object") {
    return false;
  }
  const candidates = [];
  if (typeof spawn.definitionId === "string") {
    candidates.push(spawn.definitionId);
  }
  if (typeof spawn.type === "string") {
    candidates.push(spawn.type);
  }
  if (typeof spawn.typeId === "string") {
    candidates.push(spawn.typeId);
  }
  const spawnDefinition =
    spawn.definition && typeof spawn.definition === "object" ? spawn.definition : null;
  if (spawnDefinition) {
    if (typeof spawnDefinition.type === "string") {
      candidates.push(spawnDefinition.type);
    }
    if (typeof spawnDefinition.typeId === "string") {
      candidates.push(spawnDefinition.typeId);
    }
  }
  const instance = spawn.instance && typeof spawn.instance === "object" ? spawn.instance : null;
  if (instance) {
    if (typeof instance.definitionId === "string") {
      candidates.push(instance.definitionId);
    }
    if (typeof instance.type === "string") {
      candidates.push(instance.type);
    }
    if (typeof instance.typeId === "string") {
      candidates.push(instance.typeId);
    }
    const instanceDefinition =
      instance.definition && typeof instance.definition === "object"
        ? instance.definition
        : null;
    if (instanceDefinition) {
      if (typeof instanceDefinition.type === "string") {
        candidates.push(instanceDefinition.type);
      }
      if (typeof instanceDefinition.typeId === "string") {
        candidates.push(instanceDefinition.typeId);
      }
    }
  }
  return candidates.some((candidate) => isFireIdentifier(candidate));
}

function isAttackSpawn(spawn) {
  if (!spawn || typeof spawn !== "object") {
    return false;
  }
  const candidates = [];
  if (typeof spawn.definitionId === "string") {
    candidates.push(spawn.definitionId);
  }
  if (typeof spawn.type === "string") {
    candidates.push(spawn.type);
  }
  if (typeof spawn.typeId === "string") {
    candidates.push(spawn.typeId);
  }
  const spawnDefinition =
    spawn.definition && typeof spawn.definition === "object" ? spawn.definition : null;
  if (spawnDefinition) {
    if (typeof spawnDefinition.type === "string") {
      candidates.push(spawnDefinition.type);
    }
    if (typeof spawnDefinition.typeId === "string") {
      candidates.push(spawnDefinition.typeId);
    }
  }
  const instance = spawn.instance && typeof spawn.instance === "object" ? spawn.instance : null;
  if (instance) {
    if (typeof instance.definitionId === "string") {
      candidates.push(instance.definitionId);
    }
    if (typeof instance.type === "string") {
      candidates.push(instance.type);
    }
    if (typeof instance.typeId === "string") {
      candidates.push(instance.typeId);
    }
    const instanceDefinition =
      instance.definition && typeof instance.definition === "object"
        ? instance.definition
        : null;
    if (instanceDefinition) {
      if (typeof instanceDefinition.type === "string") {
        candidates.push(instanceDefinition.type);
      }
      if (typeof instanceDefinition.typeId === "string") {
        candidates.push(instanceDefinition.typeId);
      }
    }
  }
  return candidates.some((candidate) => isAttackIdentifier(candidate));
}

function isMeleeSwingSpawn(spawn) {
  if (!spawn || typeof spawn !== "object") {
    return false;
  }
  const candidates = [];
  if (typeof spawn.definitionId === "string") {
    candidates.push(spawn.definitionId);
  }
  if (typeof spawn.type === "string") {
    candidates.push(spawn.type);
  }
  if (typeof spawn.typeId === "string") {
    candidates.push(spawn.typeId);
  }
  const spawnDefinition =
    spawn.definition && typeof spawn.definition === "object" ? spawn.definition : null;
  if (spawnDefinition) {
    if (typeof spawnDefinition.type === "string") {
      candidates.push(spawnDefinition.type);
    }
    if (typeof spawnDefinition.typeId === "string") {
      candidates.push(spawnDefinition.typeId);
    }
  }
  const instance = spawn.instance && typeof spawn.instance === "object" ? spawn.instance : null;
  if (instance) {
    if (typeof instance.definitionId === "string") {
      candidates.push(instance.definitionId);
    }
    if (typeof instance.type === "string") {
      candidates.push(instance.type);
    }
    if (typeof instance.typeId === "string") {
      candidates.push(instance.typeId);
    }
    const instanceDefinition =
      instance.definition && typeof instance.definition === "object"
        ? instance.definition
        : null;
    if (instanceDefinition) {
      if (typeof instanceDefinition.type === "string") {
        candidates.push(instanceDefinition.type);
      }
      if (typeof instanceDefinition.typeId === "string") {
        candidates.push(instanceDefinition.typeId);
      }
    }
  }
  return candidates.some((candidate) => isMeleeSwingIdentifier(candidate));
}

function shouldLogBloodSplatterSpawn(messageType, payload) {
  const normalizedType = typeof messageType === "string" ? messageType : null;
  if (normalizedType !== "state" && normalizedType !== "keyframe" && normalizedType !== "patch") {
    return false;
  }
  const spawns = collectEffectSpawnEvents(payload);
  if (spawns.length === 0) {
    return false;
  }
  return spawns.some((spawn) => isBloodSplatterSpawn(spawn));
}

function shouldLogFireSpawn(messageType, payload) {
  const normalizedType = typeof messageType === "string" ? messageType : null;
  if (normalizedType !== "state" && normalizedType !== "keyframe" && normalizedType !== "patch") {
    return false;
  }
  const spawns = collectEffectSpawnEvents(payload);
  if (spawns.length === 0) {
    return false;
  }
  return spawns.some((spawn) => isFireSpawn(spawn));
}

function shouldLogAttackSpawn(messageType, payload) {
  const normalizedType = typeof messageType === "string" ? messageType : null;
  if (normalizedType !== "state" && normalizedType !== "keyframe" && normalizedType !== "patch") {
    return false;
  }
  const spawns = collectEffectSpawnEvents(payload);
  if (spawns.length === 0) {
    return false;
  }
  return spawns.some((spawn) => isAttackSpawn(spawn));
}

function shouldLogMeleeSwingSpawn(messageType, payload) {
  const normalizedType = typeof messageType === "string" ? messageType : null;
  if (normalizedType !== "state" && normalizedType !== "keyframe" && normalizedType !== "patch") {
    return false;
  }
  const spawns = collectEffectSpawnEvents(payload);
  if (spawns.length === 0) {
    return false;
  }
  return spawns.some((spawn) => isMeleeSwingSpawn(spawn));
}

function logBloodSplatterSpawnIfNeeded(messageType, payload, rawMessage) {
  if (typeof rawMessage !== "string" || rawMessage.length === 0) {
    return;
  }
  if (!shouldLogBloodSplatterSpawn(messageType, payload)) {
    return;
  }
  const context = typeof messageType === "string" && messageType.length > 0 ? messageType : "message";
  console.log(`[effects] ${context} blood-splatter spawn detected:`, rawMessage);
}

function logFireSpawnIfNeeded(messageType, payload, rawMessage) {
  if (typeof rawMessage !== "string" || rawMessage.length === 0) {
    return;
  }
  if (!shouldLogFireSpawn(messageType, payload)) {
    return;
  }
  const context = typeof messageType === "string" && messageType.length > 0 ? messageType : "message";
  console.log(`[effects] ${context} fire spawn detected:`, rawMessage);
}

function logAttackSpawnIfNeeded(messageType, payload, rawMessage) {
  if (typeof rawMessage !== "string" || rawMessage.length === 0) {
    return;
  }
  if (!shouldLogAttackSpawn(messageType, payload)) {
    return;
  }
  const context = typeof messageType === "string" && messageType.length > 0 ? messageType : "message";
  console.log(`[effects] ${context} attack spawn detected:`, rawMessage);
}

function logMeleeSwingSpawnIfNeeded(messageType, payload, rawMessage) {
  if (typeof rawMessage !== "string" || rawMessage.length === 0) {
    return;
  }
  if (!shouldLogMeleeSwingSpawn(messageType, payload)) {
    return;
  }
  const context = typeof messageType === "string" && messageType.length > 0 ? messageType : "message";
  console.log(`[effects] ${context} melee-swing spawn detected:`, rawMessage);
}

function computeKeyframeRetryDelay(attempts) {
  const normalized = Math.max(1, Math.floor(attempts));
  const exponential = KEYFRAME_RETRY_BASE_DELAY * Math.pow(2, normalized - 1);
  const bounded = Math.min(
    KEYFRAME_RETRY_MAX_STEP,
    Math.max(KEYFRAME_RETRY_BASE_DELAY, exponential),
  );
  return bounded;
}

function createUnknownEffectUpdateHandler(store) {
  return function handleUnknownEffectUpdate(event) {
    const id = typeof event?.id === "string" && event.id.length > 0 ? event.id : "<unknown>";
    let seqValue = "unknown";
    if (typeof event?.seq === "number" && Number.isFinite(event.seq)) {
      seqValue = Math.floor(event.seq);
    } else if (typeof event?.seq === "string" && event.seq.trim().length > 0) {
      const parsed = Number(event.seq);
      if (Number.isFinite(parsed)) {
        seqValue = Math.floor(parsed);
      }
    }
    console.warn(
      `[effects] Received effect_update for unknown effect ${id} (seq ${seqValue}).`,
    );
    if (typeof store?.recordEffectLifecycleDiagnostic === "function") {
      store.recordEffectLifecycleDiagnostic(event);
    }
  };
}

function getUnknownEffectUpdateHandler(store) {
  if (!store || typeof store !== "object") {
    return createUnknownEffectUpdateHandler(store);
  }
  let handler = unknownEffectUpdateHandlers.get(store);
  if (!handler) {
    handler = createUnknownEffectUpdateHandler(store);
    unknownEffectUpdateHandlers.set(store, handler);
  }
  return handler;
}

function normalizeProtocolVersionValue(value) {
  if (typeof value === "number") {
    return Number.isFinite(value) ? value : null;
  }
  if (typeof value === "string" && value.trim().length > 0) {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : null;
  }
  return null;
}

export function readProtocolVersion(payload) {
  if (!payload || typeof payload !== "object") {
    return null;
  }

  const candidates = ["ver", "protocol"];
  for (const key of candidates) {
    if (!Object.prototype.hasOwnProperty.call(payload, key)) {
      continue;
    }
    const version = normalizeProtocolVersionValue(payload[key]);
    if (version !== null) {
      return version;
    }
  }

  return null;
}

export function handleProtocolVersion(payload, context, options = {}) {
  const version = readProtocolVersion(payload);
  const normalizedContext =
    typeof context === "string" && context.length > 0 ? context : "message";

  if (version !== null && version !== PROTOCOL_VERSION) {
    const message =
      `Protocol version mismatch (${normalizedContext}): expected ${PROTOCOL_VERSION}, received ${version}`;
    if (typeof options.onMismatch === "function") {
      options.onMismatch({
        expected: PROTOCOL_VERSION,
        received: version,
        context: normalizedContext,
        message,
      });
    } else {
      console.warn(message);
    }
  }

  return version;
}

/**
 * Build the payload describing the player's movement intent.
 *
 * @param {{ dx?: number, dy?: number } | null | undefined} intent
 * @param {string | null | undefined} facing
 * @returns {{ type: "input", dx: number, dy: number, facing: string }}
 */
export function buildInputPayload(intent, facing) {
  const movement = intent && typeof intent === "object" ? intent : {};
  const dxValue = Number(movement.dx);
  const dyValue = Number(movement.dy);
  const dx = Number.isFinite(dxValue) ? dxValue : 0;
  const dy = Number.isFinite(dyValue) ? dyValue : 0;
  const normalizedFacing =
    typeof facing === "string" && VALID_FACINGS.has(facing)
      ? facing
      : DEFAULT_FACING;

  return {
    type: "input",
    dx,
    dy,
    facing: normalizedFacing,
  };
}

/**
 * Build the payload describing a navigation target in world space.
 *
 * @param {number} x
 * @param {number} y
 * @returns {{ type: "path", x: number, y: number }}
 */
export function buildPathPayload(x, y) {
  return {
    type: "path",
    x,
    y,
  };
}

/**
 * Build the payload instructing the server to cancel an active path.
 *
 * @returns {{ type: "cancelPath" }}
 */
export function buildCancelPathPayload() {
  return { type: "cancelPath" };
}

/**
 * Build the payload for a one-off action invocation.
 *
 * @param {string} action
 * @param {Record<string, unknown> | null | undefined} params
 * @returns {{ type: "action", action: string } & (Record<"params", Record<string, unknown>> | {})}
 */
export function buildActionPayload(action, params) {
  const payload = { type: "action", action };
  if (params && typeof params === "object" && Object.keys(params).length > 0) {
    payload.params = params;
  }
  return payload;
}

/**
 * Build the payload for a heartbeat ping.
 *
 * @param {number} sentAt
 * @returns {{ type: "heartbeat", sentAt: number }}
 */
export function buildHeartbeatPayload(sentAt) {
  return { type: "heartbeat", sentAt };
}

/**
 * Build the payload for a debug console command.
 *
 * Contract: `{ type: "console", cmd, qty? }` where `qty` is included only
 * when the provided parameter can be coerced to a finite integer quantity.
 *
 * @param {string} cmd
 * @param {{ qty?: unknown } | null | undefined} params
 * @returns {{ type: "console", cmd: string } & (Record<"qty", number> | {})}
 */
export function buildConsolePayload(cmd, params) {
  const payload = { type: "console", cmd };

  if (
    params &&
    typeof params === "object" &&
    Object.prototype.hasOwnProperty.call(params, "qty")
  ) {
    const qtyRaw = params.qty;
    if (qtyRaw !== null && qtyRaw !== undefined) {
      const qtyValue = Number(qtyRaw);
      if (Number.isFinite(qtyValue)) {
        payload.qty = Math.trunc(qtyValue);
      }
    }
  }

  return payload;
}

/**
 * Parse a JSON-encoded server event into a normalized envelope.
 *
 * Arrays and primitive values are rejected to keep the envelope consistent.
 *
 * @param {string | null | undefined} jsonString
 * @returns {{ type: string, data: any } | null}
 */
export function parseServerEvent(jsonString) {
  if (typeof jsonString !== "string") {
    return null;
  }

  try {
    const parsed = JSON.parse(jsonString);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      return null;
    }

    const { type } = parsed;
    if (typeof type !== "string" || type.length === 0) {
      return null;
    }

    return { type, data: parsed };
  } catch (error) {
    return null;
  }
}

/**
 * Resolve the world width/height for movement and rendering calculations.
 *
 * Preference order:
 * 1. Explicit WORLD_* values when finite and > 0.
 * 2. Canvas dimensions when present and finite.
 * 3. GRID_* multiplied by TILE_SIZE when both are finite and positive.
 * 4. DEFAULT_WORLD_* constants as the final fallback.
 *
 * Non-object inputs and non-finite values fall through to later fallbacks.
 *
 * @param {object} storeLike
 * @returns {{ width: number, height: number }}
 */
export function getWorldDims(storeLike) {
  const source = storeLike && typeof storeLike === "object" ? storeLike : {};
  const canvas =
    source.canvas && typeof source.canvas === "object" ? source.canvas : null;

  const toPositive = (value) => {
    const num = Number(value);
    return Number.isFinite(num) && num > 0 ? num : null;
  };

  const productPositive = (a, b) => {
    const left = Number(a);
    const right = Number(b);
    if (!Number.isFinite(left) || !Number.isFinite(right)) {
      return null;
    }
    const product = left * right;
    return Number.isFinite(product) && product > 0 ? product : null;
  };

  const width =
    toPositive(source.WORLD_WIDTH) ??
    (canvas ? toPositive(canvas.width) : null) ??
    productPositive(source.GRID_WIDTH, source.TILE_SIZE) ??
    DEFAULT_WORLD_WIDTH;

  const height =
    toPositive(source.WORLD_HEIGHT) ??
    (canvas ? toPositive(canvas.height) : null) ??
    productPositive(source.GRID_HEIGHT, source.TILE_SIZE) ??
    DEFAULT_WORLD_HEIGHT;

  return { width, height };
}

/**
 * Clamp a target coordinate to the world bounds, respecting the player's size.
 *
 * Coordinates are clamped to [playerHalf, dims - playerHalf] when possible,
 * collapsing to the nearest valid edge when the world is smaller than the
 * player's diameter. Non-finite inputs are treated as zero.
 *
 * @param {number} x
 * @param {number} y
 * @param {{ width?: number, height?: number } | null | undefined} dims
 * @param {number} playerHalf
 * @returns {{ x: number, y: number }}
 */
export function clampToWorld(x, y, dims, playerHalf) {
  const size = dims && typeof dims === "object" ? dims : {};
  const rawWidth = Number(size.width);
  const rawHeight = Number(size.height);
  const width = Number.isFinite(rawWidth) ? Math.max(0, rawWidth) : 0;
  const height = Number.isFinite(rawHeight) ? Math.max(0, rawHeight) : 0;

  const halfValue = Number(playerHalf);
  const half = Number.isFinite(halfValue) && halfValue > 0 ? halfValue : 0;

  const minX = half;
  const minY = half;
  const maxX = width > 0 ? Math.max(half, width - half) : half;
  const maxY = height > 0 ? Math.max(half, height - half) : half;

  const normalizedX = Number.isFinite(Number(x)) ? Number(x) : 0;
  const normalizedY = Number.isFinite(Number(y)) ? Number(y) : 0;

  const clampedX = Math.min(Math.max(normalizedX, minX), maxX);
  const clampedY = Math.min(Math.max(normalizedY, minY), maxY);

  return { x: clampedX, y: clampedY };
}

export function normalizeCount(value, fallback) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) {
    return fallback;
  }
  return Math.max(0, Math.floor(parsed));
}

export function splitNpcCounts(countConfig, defaults = {}) {
  const source =
    countConfig && typeof countConfig === "object" ? countConfig : Object.create(null);
  const defaultGoblin = normalizeCount(
    defaults.goblinCount,
    DEFAULT_GOBLIN_COUNT,
  );
  const defaultRat = normalizeCount(defaults.ratCount, DEFAULT_RAT_COUNT);
  const defaultTotal = normalizeCount(
    defaults.npcCount,
    defaultGoblin + defaultRat,
  );

  const goblinProvided = Object.prototype.hasOwnProperty.call(
    source,
    "goblinCount",
  );
  const ratProvided = Object.prototype.hasOwnProperty.call(source, "ratCount");

  let goblinCount = goblinProvided
    ? normalizeCount(source.goblinCount, defaultGoblin)
    : defaultGoblin;
  let ratCount = ratProvided
    ? normalizeCount(source.ratCount, defaultRat)
    : defaultRat;

  if (
    Object.prototype.hasOwnProperty.call(source, "npcCount") &&
    !goblinProvided &&
    !ratProvided
  ) {
    const total = normalizeCount(source.npcCount, defaultTotal);
    const goblins = Math.min(2, total);
    goblinCount = goblins;
    ratCount = Math.max(0, total - goblins);
  }

  const npcCount = goblinCount + ratCount;

  return { goblinCount, ratCount, npcCount };
}

function normalizeWorldConfig(config) {
  const normalized = {
    obstacles: true,
    obstaclesCount: DEFAULT_OBSTACLE_COUNT,
    goldMines: true,
    goldMineCount: DEFAULT_GOLD_MINE_COUNT,
    npcs: true,
    goblinCount: DEFAULT_GOBLIN_COUNT,
    ratCount: DEFAULT_RAT_COUNT,
    npcCount: DEFAULT_NPC_COUNT,
    lava: true,
    lavaCount: DEFAULT_LAVA_COUNT,
    seed: DEFAULT_WORLD_SEED,
    width: DEFAULT_WORLD_WIDTH,
    height: DEFAULT_WORLD_HEIGHT,
  };

  if (!config || typeof config !== "object") {
    return normalized;
  }

  if (config.obstacles === false) {
    normalized.obstacles = false;
  }
  if (Object.prototype.hasOwnProperty.call(config, "obstaclesCount")) {
    normalized.obstaclesCount = normalizeCount(
      config.obstaclesCount,
      normalized.obstaclesCount,
    );
  }
  if (config.goldMines === false) {
    normalized.goldMines = false;
  }
  if (config.goldMines === true) {
    normalized.goldMines = true;
  }
  if (Object.prototype.hasOwnProperty.call(config, "goldMineCount")) {
    normalized.goldMineCount = normalizeCount(
      config.goldMineCount,
      normalized.goldMineCount,
    );
  }
  if (config.npcs === false) {
    normalized.npcs = false;
  }
  if (config.npcs === true) {
    normalized.npcs = true;
  }
  const counts = splitNpcCounts(config, normalized);
  normalized.goblinCount = counts.goblinCount;
  normalized.ratCount = counts.ratCount;
  normalized.npcCount = counts.npcCount;
  if (config.lava === false) {
    normalized.lava = false;
  }
  if (config.lava === true) {
    normalized.lava = true;
  }
  if (Object.prototype.hasOwnProperty.call(config, "lavaCount")) {
    normalized.lavaCount = normalizeCount(config.lavaCount, normalized.lavaCount);
  }

  if (Object.prototype.hasOwnProperty.call(config, "seed")) {
    const rawSeed = config.seed;
    if (typeof rawSeed === "string") {
      normalized.seed = rawSeed.trim() || DEFAULT_WORLD_SEED;
    } else if (rawSeed != null) {
      normalized.seed = String(rawSeed).trim() || DEFAULT_WORLD_SEED;
    }
  }

  if (Object.prototype.hasOwnProperty.call(config, "width")) {
    const widthValue = Number(config.width);
    if (Number.isFinite(widthValue) && widthValue > 0) {
      normalized.width = widthValue;
    }
  }
  if (Object.prototype.hasOwnProperty.call(config, "height")) {
    const heightValue = Number(config.height);
    if (Number.isFinite(heightValue) && heightValue > 0) {
      normalized.height = heightValue;
    }
  }

  return normalized;
}

// normalizeFacing guards against invalid facing values from the network.
export function normalizeFacing(facing) {
  return typeof facing === "string" && VALID_FACINGS.has(facing)
    ? facing
    : DEFAULT_FACING;
}

function normalizeGroundItems(items) {
  if (!Array.isArray(items) || items.length === 0) {
    return {};
  }
  const entries = [];
  for (const item of items) {
    if (!item || typeof item !== "object") {
      continue;
    }
    const id = typeof item.id === "string" ? item.id : null;
    if (!id) {
      continue;
    }
    const x = Number(item.x);
    const y = Number(item.y);
    const qty = Number(item.qty);
    const type = typeof item.type === "string" ? item.type : "gold";
    entries.push([
      id,
      {
        id,
        type,
        x: Number.isFinite(x) ? x : 0,
        y: Number.isFinite(y) ? y : 0,
        qty: Number.isFinite(qty) ? qty : 0,
      },
    ]);
  }
  return Object.fromEntries(entries);
}

export { normalizeWorldConfig, normalizeGroundItems };

function clonePlayersFromMap(map) {
  if (!map || typeof map !== "object") {
    return {};
  }
  const players = {};
  for (const [id, entry] of Object.entries(map)) {
    if (!entry || typeof entry !== "object" || typeof id !== "string") {
      continue;
    }
    players[id] = {
      ...entry,
      id,
      facing: normalizeFacing(entry.facing),
    };
  }
  return players;
}

function cloneNPCsFromMap(map) {
  if (!map || typeof map !== "object") {
    return {};
  }
  const npcs = {};
  for (const [id, entry] of Object.entries(map)) {
    if (!entry || typeof entry !== "object" || typeof id !== "string") {
      continue;
    }
    npcs[id] = {
      ...entry,
      id,
      facing: normalizeFacing(entry.facing),
    };
  }
  return npcs;
}

function collectEffectsFromMap(map) {
  if (!map || typeof map !== "object") {
    return [];
  }
  const effects = [];
  for (const entry of Object.values(map)) {
    if (!entry || typeof entry !== "object") {
      continue;
    }
    effects.push(entry);
  }
  return effects;
}

function cloneGroundItemsFromMap(map) {
  if (!map || typeof map !== "object") {
    return {};
  }
  const items = {};
  for (const [id, entry] of Object.entries(map)) {
    if (!entry || typeof entry !== "object" || typeof id !== "string") {
      continue;
    }
    const x = Number(entry.x);
    const y = Number(entry.y);
    const qty = Number(entry.qty);
    items[id] = {
      ...entry,
      id,
      x: Number.isFinite(x) ? x : 0,
      y: Number.isFinite(y) ? y : 0,
      qty: Number.isFinite(qty) ? qty : 0,
    };
  }
  return items;
}

export function applyStateSnapshot(prev, payload, patchedState) {
  const previousState = prev && typeof prev === "object" ? prev : {};
  const snapshot = payload && typeof payload === "object" ? payload : {};
  const patched = patchedState && typeof patchedState === "object" ? patchedState : null;

  let players = {};
  if (Array.isArray(snapshot.players)) {
    for (const entry of snapshot.players) {
      if (!entry || typeof entry !== "object" || typeof entry.id !== "string") {
        continue;
      }
      players[entry.id] = {
        ...entry,
        facing: normalizeFacing(entry.facing),
      };
    }
  } else if (patched?.players) {
    players = clonePlayersFromMap(patched.players);
  } else if (previousState.players && typeof previousState.players === "object") {
    players = clonePlayersFromMap(previousState.players);
  }

  let npcs = {};
  if (Array.isArray(snapshot.npcs)) {
    for (const entry of snapshot.npcs) {
      if (!entry || typeof entry !== "object" || typeof entry.id !== "string") {
        continue;
      }
      npcs[entry.id] = {
        ...entry,
        facing: normalizeFacing(entry.facing),
      };
    }
  } else if (patched?.npcs) {
    npcs = cloneNPCsFromMap(patched.npcs);
  } else if (previousState.npcs && typeof previousState.npcs === "object") {
    npcs = cloneNPCsFromMap(previousState.npcs);
  }

  const obstacles = Array.isArray(snapshot.obstacles)
    ? snapshot.obstacles.slice()
    : Array.isArray(previousState.obstacles)
      ? previousState.obstacles.slice()
      : [];
  let effects = null;
  if (Array.isArray(snapshot.effects)) {
    effects = snapshot.effects;
  } else if (Array.isArray(patched?.effects)) {
    effects = patched.effects;
  } else if (patched?.effects) {
    effects = collectEffectsFromMap(patched.effects);
  } else if (Array.isArray(previousState.effects)) {
    effects = previousState.effects;
  } else {
    effects = [];
  }

  const result = {
    players,
    npcs,
    obstacles,
    effects,
    hasLocalPlayer: false,
    keyframeInterval: null,
  };

  let groundItemsState = {};
  if (Array.isArray(snapshot.groundItems)) {
    groundItemsState = normalizeGroundItems(snapshot.groundItems);
  } else if (patched?.groundItems) {
    groundItemsState = cloneGroundItemsFromMap(patched.groundItems);
  } else if (previousState.groundItems && typeof previousState.groundItems === "object") {
    groundItemsState = cloneGroundItemsFromMap(previousState.groundItems);
  }
  result.groundItems = groundItemsState;

  let lastTick = null;
  if (Object.prototype.hasOwnProperty.call(snapshot, "t")) {
    const tickValue = snapshot.t;
    if (typeof tickValue === "number" && Number.isFinite(tickValue) && tickValue >= 0) {
      lastTick = Math.floor(tickValue);
    }
  }
  if (lastTick === null && patched && typeof patched.tick === "number" && Number.isFinite(patched.tick)) {
    lastTick = Math.floor(patched.tick);
  }
  if (lastTick === null && previousState && typeof previousState === "object") {
    const priorTick = previousState.lastTick;
    if (typeof priorTick === "number" && Number.isFinite(priorTick) && priorTick >= 0) {
      lastTick = Math.floor(priorTick);
    }
  }
  result.lastTick = lastTick;

  if (snapshot.config) {
    result.worldConfig = normalizeWorldConfig(snapshot.config);
  } else if (previousState.worldConfig) {
    result.worldConfig = { ...previousState.worldConfig };
  }

  if (Object.prototype.hasOwnProperty.call(snapshot, "keyframeInterval")) {
    const intervalCandidate = Number(snapshot.keyframeInterval);
    if (Number.isFinite(intervalCandidate) && intervalCandidate >= 1) {
      result.keyframeInterval = Math.floor(intervalCandidate);
    }
  }
  if (
    result.keyframeInterval === null &&
    previousState &&
    typeof previousState === "object" &&
    Number.isFinite(previousState.keyframeInterval) &&
    previousState.keyframeInterval >= 1
  ) {
    result.keyframeInterval = Math.floor(previousState.keyframeInterval);
  }

  const localId = typeof previousState.playerId === "string" ? previousState.playerId : null;
  if (localId && players[localId]) {
    result.hasLocalPlayer = true;
    result.currentFacing = players[localId].facing;
  }

  return result;
}

export function deriveDisplayMaps(
  players,
  npcs,
  previousDisplayPlayers = {},
  previousDisplayNpcs = {},
) {
  const normalizedPlayers = players && typeof players === "object" ? players : {};
  const normalizedNpcs = npcs && typeof npcs === "object" ? npcs : {};
  const prevPlayers =
    previousDisplayPlayers && typeof previousDisplayPlayers === "object"
      ? previousDisplayPlayers
      : {};
  const prevNpcs =
    previousDisplayNpcs && typeof previousDisplayNpcs === "object"
      ? previousDisplayNpcs
      : {};

  const displayPlayers = {};
  for (const player of Object.values(normalizedPlayers)) {
    if (!player || typeof player.id !== "string") {
      continue;
    }
    const prev = prevPlayers[player.id];
    if (
      prev &&
      typeof prev === "object" &&
      Number.isFinite(prev.x) &&
      Number.isFinite(prev.y)
    ) {
      displayPlayers[player.id] = prev;
    } else {
      displayPlayers[player.id] = { x: player.x, y: player.y };
    }
  }

  const displayNPCs = {};
  for (const npc of Object.values(normalizedNpcs)) {
    if (!npc || typeof npc.id !== "string") {
      continue;
    }
    const prev = prevNpcs[npc.id];
    if (
      prev &&
      typeof prev === "object" &&
      Number.isFinite(prev.x) &&
      Number.isFinite(prev.y)
    ) {
      displayNPCs[npc.id] = prev;
    } else {
      displayNPCs[npc.id] = { x: npc.x, y: npc.y };
    }
  }

  return { displayPlayers, displayNPCs };
}

function handleConsoleAck(store, payload) {
  if (!payload || typeof payload !== "object") {
    return;
  }
  const cmd = typeof payload.cmd === "string" ? payload.cmd : "";
  const status = typeof payload.status === "string" ? payload.status : "ok";
  const qtyValue = Number(payload.qty);
  const qty = Number.isFinite(qtyValue) ? qtyValue : 0;
  const reason = typeof payload.reason === "string" ? payload.reason : null;
  const stackId = typeof payload.stackId === "string" ? payload.stackId : null;
  store.lastConsoleAck = {
    cmd,
    status,
    qty,
    reason,
    stackId,
    receivedAt: Date.now(),
  };
  const messageParts = ["[console]", cmd || "<unknown>", status];
  if (qty) {
    messageParts.push(`qty=${qty}`);
  }
  if (reason) {
    messageParts.push(`reason=${reason}`);
  }
  if (stackId) {
    messageParts.push(`stack=${stackId}`);
  }
  const message = messageParts.join(" ");
  if (status === "ok") {
    console.info(message);
  } else {
    console.warn(message);
  }
}

function ensureEffectTriggerState(store) {
  if (!Array.isArray(store.pendingEffectTriggers)) {
    store.pendingEffectTriggers = [];
  }
  if (!(store.processedEffectTriggerIds instanceof Set)) {
    store.processedEffectTriggerIds = new Set();
  }
}

/**
 * Returns an updated effect trigger queue without mutating the provided state.
 *
 * - Ignores entries that are not objects.
 * - Deduplicates triggers that provide a string `id`, keeping the first occurrence.
 * - Triggers without an `id` are always queued and preserve their relative order.
 */
export function enqueueEffectTriggers(prev, triggers) {
  const sourcePending = Array.isArray(prev && prev.pending) ? prev.pending : [];
  const sourceProcessed =
    prev && prev.processedIds instanceof Set ? prev.processedIds : new Set();

  const pending = sourcePending.slice();
  const processedIds = new Set(sourceProcessed);

  if (!Array.isArray(triggers) || triggers.length === 0) {
    return { pending, processedIds };
  }

  for (const trigger of triggers) {
    if (!trigger || typeof trigger !== "object") {
      continue;
    }
    const id = typeof trigger.id === "string" ? trigger.id : null;
    if (id && processedIds.has(id)) {
      continue;
    }
    pending.push(trigger);
    if (id) {
      processedIds.add(id);
    }
  }

  return { pending, processedIds };
}

function queueEffectTriggers(store, triggers) {
  if (!Array.isArray(triggers) || triggers.length === 0) {
    return;
  }
  ensureEffectTriggerState(store);
  const nextState = enqueueEffectTriggers(
    {
      pending: store.pendingEffectTriggers,
      processedIds: store.processedEffectTriggerIds,
    },
    triggers,
  );
  store.pendingEffectTriggers = nextState.pending;
  store.processedEffectTriggerIds = nextState.processedIds;
}

function getPendingKeyframeRequests(store) {
  if (!store || !store.patchState) {
    return null;
  }
  const { pendingKeyframeRequests } = store.patchState;
  return pendingKeyframeRequests instanceof Map ? pendingKeyframeRequests : null;
}

function getPendingReplayForSequence(store, sequence) {
  if (!store || !store.patchState) {
    return null;
  }
  const { pendingReplays } = store.patchState;
  if (!Array.isArray(pendingReplays) || pendingReplays.length === 0) {
    return null;
  }
  const normalizedSeq = Math.floor(sequence);
  return (
    pendingReplays.find((entry) => entry && entry.sequence === normalizedSeq) || null
  );
}

function stopKeyframeRetryLoop(store) {
  if (!store) {
    return;
  }
  if (store.keyframeRetryTimer !== null) {
    clearInterval(store.keyframeRetryTimer);
    store.keyframeRetryTimer = null;
  }
}

function startKeyframeRetryLoop(store) {
  if (!store) {
    return;
  }
  if (store.keyframeRetryTimer !== null) {
    return;
  }
  store.keyframeRetryTimer = setInterval(() => {
    processKeyframeRetryLoop(store);
  }, KEYFRAME_RETRY_TICK_INTERVAL);
}

function updateKeyframeRetryLoop(store) {
  const requests = getPendingKeyframeRequests(store);
  if (requests && requests.size > 0) {
    startKeyframeRetryLoop(store);
  } else {
    stopKeyframeRetryLoop(store);
  }
}

function processKeyframeRetryLoop(store) {
  const requests = getPendingKeyframeRequests(store);
  if (!requests || requests.size === 0) {
    stopKeyframeRetryLoop(store);
    return;
  }
  const now = Date.now();
  for (const [sequence, entry] of requests.entries()) {
    if (!entry || typeof entry !== "object") {
      continue;
    }
    const nextRetryAt = Number(entry.nextRetryAt);
    if (!Number.isFinite(nextRetryAt)) {
      continue;
    }
    const attempts = Math.max(0, Math.floor(entry.attempts ?? 0));
    if (attempts >= KEYFRAME_RETRY_MAX_ATTEMPTS) {
      if (nextRetryAt <= now) {
        requests.delete(sequence);
        handleConnectionLoss(store);
        return;
      }
      continue;
    }
    if (nextRetryAt > now) {
      continue;
    }
    const pendingReplay = getPendingReplayForSequence(store, sequence);
    const hintTick =
      pendingReplay && typeof pendingReplay.keyframeTick === "number"
        ? pendingReplay.keyframeTick
        : null;
    requestKeyframeSnapshot(store, sequence, hintTick, { incrementAttempt: true });
  }
}

function requestKeyframeSnapshot(store, sequence, tick, options = {}) {
  if (!store || typeof sequence !== "number" || !Number.isFinite(sequence)) {
    return;
  }
  const normalizedSeq = Math.floor(sequence);
  if (normalizedSeq <= 0) {
    return;
  }
  const pendingRequests = getPendingKeyframeRequests(store);
  if (pendingRequests) {
    const existing = pendingRequests.get(normalizedSeq) || {};
    const baseAttempts = Math.max(0, Math.floor(existing.attempts ?? 0));
    const shouldIncrement = options.incrementAttempt !== false;
    const nextAttempts = shouldIncrement
      ? baseAttempts + 1
      : baseAttempts > 0
        ? baseAttempts
        : 1;
    const now = Date.now();
    const nextRetryDelay = computeKeyframeRetryDelay(nextAttempts);
    const nextRetryAt = now + nextRetryDelay;
    const firstRequestedAtOption = options.firstRequestedAt;
    const firstRequestedAt = Number.isFinite(firstRequestedAtOption)
      ? Math.floor(firstRequestedAtOption)
      : Number.isFinite(existing.firstRequestedAt)
        ? Math.floor(existing.firstRequestedAt)
        : now;
    pendingRequests.set(normalizedSeq, {
      attempts: nextAttempts,
      nextRetryAt,
      firstRequestedAt,
    });
  }
  const message = {
    type: "keyframeRequest",
    keyframeSeq: normalizedSeq,
  };
  if (typeof tick === "number" && Number.isFinite(tick) && tick >= 0) {
    message.keyframeTick = Math.floor(tick);
  }
  sendMessage(store, message);
  updateKeyframeRetryLoop(store);
}

function syncPatchTestingState(store, payload, source) {
  if (!store || !store.patchState) {
    return null;
  }
  const previous = store.patchState;
  const resetHistory =
    source === "join" ||
    (payload && typeof payload === "object" && (
      payload.resync === true ||
      payload.patchReset === true ||
      payload.full === true ||
      payload.reset === true
    ));
  const next = updatePatchState(previous, payload, {
    source,
    resetHistory,
    requestKeyframe: (sequence, tick, options = {}) =>
      requestKeyframeSnapshot(store, sequence, tick, options),
  });
  const prevError = previous && typeof previous === "object" ? previous.lastError : null;
  const prevCount = Array.isArray(previous?.errors) ? previous.errors.length : 0;
  const nextCount = Array.isArray(next.errors) ? next.errors.length : 0;
  const nextError = next.lastError;
  const hasNewError =
    !!nextError && (!prevError || prevCount !== nextCount || prevError.message !== nextError.message);
  if (hasNewError) {
    const parts = ["[patches]", source || "snapshot"];
    if (nextError.kind) {
      parts.push(nextError.kind);
    }
    if (nextError.entityId) {
      parts.push(nextError.entityId);
    }
    parts.push(nextError.message || "error applying patch");
    console.warn(parts.join(" "), nextError);
  }
  let finalState = next;
  if (next && next.resyncRequested) {
    finalState = { ...next, resyncRequested: false };
    stopKeyframeRetryLoop(store);
    handleConnectionLoss(store);
  }
  store.patchState = finalState;
  return finalState;
}

function handleKeyframeMessage(store, payload) {
  const sequence = typeof payload?.sequence === "number" && Number.isFinite(payload.sequence)
    ? Math.floor(payload.sequence)
    : null;
  const messageType = typeof payload?.type === "string" ? payload.type : null;
  const nextState = syncPatchTestingState(store, payload, messageType || "keyframe");
  if (
    (messageType === "keyframe" || messageType === "keyframeNack") &&
    typeof store?.patchState?.pendingKeyframeRequests !== "undefined"
  ) {
    updateKeyframeRetryLoop(store);
  }
  if (store && typeof store.updateDiagnostics === "function") {
    store.updateDiagnostics();
  }
  return nextState;
}

// sendMessage serializes payloads, applies simulated latency, and tracks stats.
export function sendMessage(store, payload, { onSent } = {}) {
  if (!store.socket || store.socket.readyState !== WebSocket.OPEN) {
    return;
  }
  const basePayload =
    payload && typeof payload === "object" ? { ...payload } : {};
  const message = {
    ...basePayload,
    ver: PROTOCOL_VERSION,
  };

  const lastTick = store.lastTick;
  if (typeof lastTick === "number" && Number.isFinite(lastTick) && lastTick >= 0) {
    message.ack = Math.floor(lastTick);
  }
  const messageText = JSON.stringify(message);
  const dispatch = () => {
    if (!store.socket || store.socket.readyState !== WebSocket.OPEN) {
      return;
    }
    if (onSent) {
      onSent();
    }
    store.socket.send(messageText);
    store.messagesSent += 1;
    store.bytesSent += messageText.length;
    store.lastMessageSentAt = Date.now();
    store.updateDiagnostics();
  };

  if (store.simulatedLatencyMs > 0) {
    setTimeout(dispatch, store.simulatedLatencyMs);
  } else {
    dispatch();
  }
}

export function setKeyframeCadence(store, interval) {
  if (!Number.isFinite(interval) || interval < 1) {
    return;
  }
  const normalized = Math.floor(interval);
  const message = {
    type: "keyframeCadence",
    keyframeInterval: normalized,
  };
  sendMessage(store, message);
}

// joinGame performs the `/join` handshake and seeds the local store.
export async function joinGame(store) {
  if (store.isJoining) return;
  store.isJoining = true;
  if (store.reconnectTimeout !== null) {
    clearTimeout(store.reconnectTimeout);
    store.reconnectTimeout = null;
  }
  store.setStatusBase("Joining game...");
  try {
    const response = await fetch("/join", { method: "POST" });
    if (!response.ok) {
      throw new Error(`join failed: ${response.status}`);
    }
    const payload = await response.json();
    handleProtocolVersion(payload, "/join response");
    store.playerId = payload.id;
    store.players = Object.fromEntries(
      payload.players.map((p) => [p.id, { ...p, facing: normalizeFacing(p.facing) }])
    );
    store.npcs = Object.fromEntries(
      Array.isArray(payload.npcs)
        ? payload.npcs.map((npc) => [npc.id, { ...npc, facing: normalizeFacing(npc.facing) }])
        : []
    );
    store.obstacles = Array.isArray(payload.obstacles) ? payload.obstacles : [];
    store.effects = Array.isArray(payload.effects) ? payload.effects : [];
    store.groundItems = normalizeGroundItems(payload.groundItems);
    store.pendingEffectTriggers = [];
    store.processedEffectTriggerIds = new Set();
    resetEffectLifecycleState(store);
    if (typeof store.resetEffectLifecycleDiagnostics === "function") {
      store.resetEffectLifecycleDiagnostics();
    }
    store.lastEffectLifecycleSummary = null;
    queueEffectTriggers(store, payload.effectTriggers);
    const joinLifecycleSummary = applyEffectLifecycleBatch(store, payload, {
      onUnknownUpdate: getUnknownEffectUpdateHandler(store),
    });
    store.lastEffectLifecycleSummary = joinLifecycleSummary;
    updateEphemeralEffectLifecycleEntries(store, joinLifecycleSummary);
    syncPatchTestingState(store, payload, "join");
    store.worldConfig = normalizeWorldConfig(payload.config);
    store.WORLD_WIDTH = store.worldConfig.width;
    store.WORLD_HEIGHT = store.worldConfig.height;
    if (store.effectManager && typeof store.effectManager.clear === "function") {
      store.effectManager.clear();
    }
    if (typeof store.updateWorldConfigUI === "function") {
      store.updateWorldConfigUI();
    }
    const joinCadenceValue = Number(payload.keyframeInterval);
    if (Number.isFinite(joinCadenceValue) && joinCadenceValue >= 1) {
      const normalizedCadence = Math.floor(joinCadenceValue);
      store.keyframeInterval = normalizedCadence;
      if (!Number.isFinite(store.defaultKeyframeInterval)) {
        store.defaultKeyframeInterval = normalizedCadence;
      }
    } else {
      store.keyframeInterval = null;
    }
    if (typeof store.updateRenderModeUI === "function") {
      store.updateRenderModeUI();
    }
    const fallbackSpawnX =
      typeof store.WORLD_WIDTH === "number"
        ? store.WORLD_WIDTH / 2
        : DEFAULT_WORLD_WIDTH / 2;
    const fallbackSpawnY =
      typeof store.WORLD_HEIGHT === "number"
        ? store.WORLD_HEIGHT / 2
        : DEFAULT_WORLD_HEIGHT / 2;
    if (!store.players[store.playerId]) {
      store.players[store.playerId] = {
        id: store.playerId,
        x: fallbackSpawnX,
        y: fallbackSpawnY,
        facing: DEFAULT_FACING,
      };
    }
    store.displayPlayers = {};
    Object.values(store.players).forEach((p) => {
      store.displayPlayers[p.id] = { x: p.x, y: p.y };
    });
    store.displayNPCs = {};
    Object.values(store.npcs).forEach((npc) => {
      store.displayNPCs[npc.id] = { x: npc.x, y: npc.y };
    });
    store.currentIntent = { dx: 0, dy: 0 };
    store.currentFacing = normalizeFacing(store.players[store.playerId].facing);
    store.directionOrder = [];
    store.isPathActive = false;
    store.activePathTarget = null;
    store.lastPathRequestAt = null;
    store.setLatency(null);
    store.setStatusBase(`Connected as ${store.playerId}. Use WASD or click to move.`);
    connectEvents(store);
    store.updateDiagnostics();
    if (store.renderInventory) {
      store.renderInventory();
    }
  } catch (err) {
    store.setLatency(null);
    store.setStatusBase(`Unable to join: ${err.message}`);
    setTimeout(() => joinGame(store), 1500);
  } finally {
    store.isJoining = false;
  }
}

// connectEvents opens the WebSocket and sets up connection handlers.
export function connectEvents(store) {
  if (!store.playerId) return;
  closeSocketSilently(store);

  const protocol = window.location.protocol === "https:" ? "wss" : "ws";
  const wsUrl = `${protocol}://${window.location.host}/ws?id=${encodeURIComponent(
    store.playerId
  )}`;
  store.socket = new WebSocket(wsUrl);
  store.messagesSent = 0;
  store.bytesSent = 0;
  store.lastMessageSentAt = null;
  store.lastHeartbeatSentAt = null;
  store.lastHeartbeatAckAt = null;
  store.lastHeartbeatRoundTrip = null;
  store.lastIntentSentAt = null;
  store.lastStateReceivedAt = null;
  store.lastTick = null;
  store.updateDiagnostics();
  if (typeof store.updateRenderModeUI === "function") {
    store.updateRenderModeUI();
  }

  store.socket.onopen = () => {
    store.setStatusBase(`Connected as ${store.playerId}. Use WASD or click to move.`);
    store.setLatency(null);
    sendCurrentIntent(store);
    startHeartbeat(store);
    store.updateDiagnostics();
    if (typeof store.updateRenderModeUI === "function") {
      store.updateRenderModeUI();
    }
  };

  store.socket.onmessage = (event) => {
    const parsed = parseServerEvent(event.data);
    if (!parsed) {
      return;
    }

    const payload = parsed.data;
    logBloodSplatterSpawnIfNeeded(parsed.type, payload, event.data);
    logFireSpawnIfNeeded(parsed.type, payload, event.data);
    logAttackSpawnIfNeeded(parsed.type, payload, event.data);
    logMeleeSwingSpawnIfNeeded(parsed.type, payload, event.data);
    handleProtocolVersion(payload, `${parsed.type} message`);
    if (parsed.type === "state") {
        const patchState = syncPatchTestingState(store, payload, "state");
        const patchedView =
          patchState && typeof patchState === "object" && patchState.patched
            ? patchState.patched
            : null;
        const snapshot = applyStateSnapshot(store, payload, patchedView);

        store.players = snapshot.players;
        store.npcs = snapshot.npcs;
        store.obstacles = snapshot.obstacles;
        store.effects = snapshot.effects;
        store.groundItems = snapshot.groundItems || {};
        queueEffectTriggers(store, payload.effectTriggers);
        const lifecycleSummary = applyEffectLifecycleBatch(store, payload, {
          onUnknownUpdate: getUnknownEffectUpdateHandler(store),
        });
        store.lastEffectLifecycleSummary = lifecycleSummary;
        updateEphemeralEffectLifecycleEntries(store, lifecycleSummary);

        if (snapshot.worldConfig) {
          store.worldConfig = snapshot.worldConfig;
          store.WORLD_WIDTH = store.worldConfig.width;
          store.WORLD_HEIGHT = store.worldConfig.height;
          if (typeof store.updateWorldConfigUI === "function") {
            store.updateWorldConfigUI();
          }
        }

        if (!snapshot.hasLocalPlayer) {
          store.setStatusBase("Server no longer recognizes this player. Rejoining...");
          handleConnectionLoss(store);
          return;
        }

        store.currentFacing = snapshot.currentFacing;

        const { displayPlayers, displayNPCs } = deriveDisplayMaps(
          store.players,
          store.npcs,
          store.displayPlayers,
          store.displayNPCs,
        );
        store.displayPlayers = displayPlayers;
        store.displayNPCs = displayNPCs;
        if (
          store.isPathActive &&
          store.activePathTarget &&
          store.players[store.playerId]
        ) {
          const target = store.activePathTarget;
          const playerState = store.players[store.playerId];
          const dx = playerState.x - target.x;
          const dy = playerState.y - target.y;
          const arriveRadius = (store.PLAYER_HALF || 14) + 2;
          if (Math.hypot(dx, dy) <= arriveRadius) {
            store.isPathActive = false;
            store.activePathTarget = null;
          }
        }
        store.lastStateReceivedAt = Date.now();
        store.lastTick = snapshot.lastTick;
        if (Number.isFinite(snapshot.keyframeInterval) && snapshot.keyframeInterval >= 1) {
          const normalizedCadence = Math.floor(snapshot.keyframeInterval);
          store.keyframeInterval = normalizedCadence;
          if (!Number.isFinite(store.defaultKeyframeInterval)) {
            store.defaultKeyframeInterval = normalizedCadence;
          }
          if (typeof store.updateRenderModeUI === "function") {
            store.updateRenderModeUI();
          }
        }
        store.updateDiagnostics();
        if (store.renderInventory) {
          store.renderInventory();
        }
      } else if (parsed.type === "heartbeat") {
        const ackAt = Date.now();
        const roundTrip = computeRtt(payload, ackAt);
        if (roundTrip !== null) {
          store.setLatency(roundTrip);
          store.lastHeartbeatRoundTrip = roundTrip;
        }
        store.lastHeartbeatAckAt = ackAt;
        store.updateDiagnostics();
      } else if (parsed.type === "console_ack") {
        handleConsoleAck(store, payload);
      } else if (
        parsed.type === "keyframe" ||
        parsed.type === "keyframeNack"
      ) {
        handleKeyframeMessage(store, payload);
      }
  };

  const handleSocketDrop = () => {
    handleConnectionLoss(store);
  };

  store.socket.onerror = handleSocketDrop;
  store.socket.onclose = handleSocketDrop;
}

// sendCurrentIntent pushes the latest movement intent to the server.
export function sendCurrentIntent(store) {
  if (!store.socket || store.socket.readyState !== WebSocket.OPEN) {
    return;
  }
  const payload = buildInputPayload(store.currentIntent, store.currentFacing);
  sendMessage(store, payload, {
    onSent: () => {
      store.lastIntentSentAt = Date.now();
      store.updateDiagnostics();
    },
  });
  if (store.players[store.playerId]) {
    store.players[store.playerId].facing = store.currentFacing;
  }
}

// sendMoveTo requests server-driven navigation toward a world position.
export function sendMoveTo(store, x, y) {
  const worldDims = getWorldDims(store);
  const { x: clampedX, y: clampedY } = clampToWorld(
    x,
    y,
    worldDims,
    store.PLAYER_HALF,
  );
  if (!store.socket || store.socket.readyState !== WebSocket.OPEN) {
    store.activePathTarget = { x: clampedX, y: clampedY };
    store.isPathActive = false;
    return;
  }
  const payload = buildPathPayload(clampedX, clampedY);
  sendMessage(store, payload, {
    onSent: () => {
      store.activePathTarget = { x: clampedX, y: clampedY };
      store.isPathActive = true;
      store.lastPathRequestAt = Date.now();
      store.updateDiagnostics();
    },
  });
}

// sendCancelPath stops the current server-driven navigation, if any.
export function sendCancelPath(store) {
  store.isPathActive = false;
  store.activePathTarget = null;
  store.lastPathRequestAt = null;
  if (!store.socket || store.socket.readyState !== WebSocket.OPEN) {
    store.updateDiagnostics();
    return;
  }
  const payload = buildCancelPathPayload();
  sendMessage(store, payload, {
    onSent: () => {
      store.updateDiagnostics();
    },
  });
}

// sendAction dispatches a one-off action message for abilities.
export function sendAction(store, action, params = undefined) {
  if (!store.socket || store.socket.readyState !== WebSocket.OPEN) {
    return;
  }
  if (!action) {
    return;
  }
  const payload = buildActionPayload(action, params);
  sendMessage(store, payload);
}

// sendConsoleCommand dispatches a debug console command to the server.
export function sendConsoleCommand(store, cmd, params = undefined) {
  if (!store.socket || store.socket.readyState !== WebSocket.OPEN) {
    return;
  }
  if (typeof cmd !== "string" || cmd.length === 0) {
    return;
  }
  const payload = buildConsolePayload(cmd, params);
  sendMessage(store, payload);
}

function dispatchHeartbeat(store, sentAt = Date.now()) {
  if (!store.socket || store.socket.readyState !== WebSocket.OPEN) {
    return;
  }
  const timestamp = Number.isFinite(sentAt) ? sentAt : Date.now();
  const payload = buildHeartbeatPayload(timestamp);
  sendMessage(store, payload, {
    onSent: () => {
      store.lastHeartbeatSentAt = Date.now();
      store.updateDiagnostics();
    },
  });
}

function createHeartbeatEnv(store) {
  return {
    now: () => Date.now(),
    setInterval: (fn, ms) => {
      const id = globalThis.setInterval(fn, ms);
      store.heartbeatTimer = id;
      store.updateDiagnostics();
      return id;
    },
    clearInterval: (id) => {
      globalThis.clearInterval(id);
      if (store.heartbeatTimer === id) {
        store.heartbeatTimer = null;
        store.updateDiagnostics();
      }
    },
    send: (timestamp) => {
      dispatchHeartbeat(store, timestamp);
    },
  };
}

function getHeartbeatController(store) {
  let controller = heartbeatControllers.get(store);
  if (!controller) {
    controller = createHeartbeat(createHeartbeatEnv(store));
    heartbeatControllers.set(store, controller);
  }
  return controller;
}

// startHeartbeat kicks off the repeating heartbeat timer.
export function startHeartbeat(store) {
  const controller = getHeartbeatController(store);
  controller.start(HEARTBEAT_INTERVAL);
}

// stopHeartbeat clears the heartbeat timer if it exists.
export function stopHeartbeat(store) {
  const controller = heartbeatControllers.get(store);
  if (controller) {
    controller.stop();
    return;
  }
  if (store.heartbeatTimer !== null) {
    globalThis.clearInterval(store.heartbeatTimer);
    store.heartbeatTimer = null;
    store.updateDiagnostics();
  }
}

// sendHeartbeat emits a heartbeat payload and records timing.
export function sendHeartbeat(store) {
  dispatchHeartbeat(store);
}

// closeSocketSilently tears down handlers and closes without triggering loops.
function closeSocketSilently(store) {
  if (!store.socket) return;
  stopHeartbeat(store);
  store.socket.onopen = null;
  store.socket.onmessage = null;
  store.socket.onclose = null;
  store.socket.onerror = null;
  try {
    store.socket.close();
  } catch (err) {
    console.error("Failed to close socket", err);
  }
  store.socket = null;
  if (typeof store.updateRenderModeUI === "function") {
    store.updateRenderModeUI();
  }
}

// scheduleReconnect queues another join attempt after a delay.
function scheduleReconnect(store) {
  if (store.reconnectTimeout !== null) return;
  store.reconnectTimeout = setTimeout(() => {
    store.reconnectTimeout = null;
    joinGame(store);
  }, 1000);
}

// handleConnectionLoss resets state and begins the reconnect process.
function handleConnectionLoss(store) {
  closeSocketSilently(store);
  store.setLatency(null);
  store.lastStateReceivedAt = null;
  store.lastIntentSentAt = null;
  store.lastHeartbeatSentAt = null;
  store.lastHeartbeatAckAt = null;
  store.lastHeartbeatRoundTrip = null;
  store.lastMessageSentAt = null;
  store.messagesSent = 0;
  store.bytesSent = 0;
  store.lastTick = null;
  store.currentIntent = { dx: 0, dy: 0 };
  store.currentFacing = DEFAULT_FACING;
  store.directionOrder = [];
  store.isPathActive = false;
  store.activePathTarget = null;
  store.lastPathRequestAt = null;
  stopKeyframeRetryLoop(store);
  store.patchState = createPatchState();
  store.updateDiagnostics();
  if (typeof store.updateRenderModeUI === "function") {
    store.updateRenderModeUI();
  }
  if (store.playerId === null) {
    return;
  }
  store.setStatusBase("Connection lost. Rejoining...");
  store.playerId = null;
  store.players = {};
  store.displayPlayers = {};
  store.npcs = {};
  store.displayNPCs = {};
  resetEffectLifecycleState(store);
  if (typeof store.resetEffectLifecycleDiagnostics === "function") {
    store.resetEffectLifecycleDiagnostics();
  }
  store.lastEffectLifecycleSummary = null;
  if (store.effectManager && typeof store.effectManager.clear === "function") {
    store.effectManager.clear();
  }
  scheduleReconnect(store);
}

export async function resetWorld(store, config) {
  const rawSeed = config?.seed;
  let seed = "";
  if (typeof rawSeed === "string") {
    seed = rawSeed.trim();
  } else if (rawSeed != null) {
    seed = String(rawSeed).trim();
  }
  const { goblinCount, ratCount, npcCount } = splitNpcCounts(config, {
    goblinCount: DEFAULT_GOBLIN_COUNT,
    ratCount: DEFAULT_RAT_COUNT,
    npcCount: DEFAULT_NPC_COUNT,
  });
  const payload = {
    obstacles: !!config?.obstacles,
    obstaclesCount: normalizeCount(
      config?.obstaclesCount,
      DEFAULT_OBSTACLE_COUNT,
    ),
    goldMines: !!config?.goldMines,
    goldMineCount: normalizeCount(
      config?.goldMineCount,
      DEFAULT_GOLD_MINE_COUNT,
    ),
    npcs: !!config?.npcs,
    goblinCount,
    ratCount,
    npcCount,
    lava: !!config?.lava,
    lavaCount: normalizeCount(config?.lavaCount, DEFAULT_LAVA_COUNT),
    seed,
  };

  let response;
  try {
    response = await fetch("/world/reset", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    throw new Error(`request failed: ${message}`);
  }

  if (!response.ok) {
    let errorText = "";
    try {
      errorText = await response.text();
    } catch (err) {
      errorText = "";
    }
    const fallback = `request failed with status ${response.status}`;
    throw new Error(errorText || fallback);
  }

  let data;
  try {
    data = await response.json();
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    throw new Error(`invalid response: ${message}`);
  }

  const normalized = normalizeWorldConfig(data.config);
  store.worldConfig = normalized;
  if (typeof store.updateWorldConfigUI === "function") {
    store.updateWorldConfigUI();
  }

  return normalized;
}

export const __networkInternals = {
  computeKeyframeRetryDelay,
  processKeyframeRetryLoop,
  requestKeyframeSnapshot,
  KEYFRAME_RETRY_CONSTANTS: {
    BASE_DELAY: KEYFRAME_RETRY_BASE_DELAY,
    MAX_STEP: KEYFRAME_RETRY_MAX_STEP,
    MAX_ATTEMPTS: KEYFRAME_RETRY_MAX_ATTEMPTS,
  },
};
