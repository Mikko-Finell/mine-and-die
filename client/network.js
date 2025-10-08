import { computeRtt, createHeartbeat } from "./heartbeat.js";

const HEARTBEAT_INTERVAL = 2000;
const DEFAULT_FACING = "down";
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
    entries.push([
      id,
      {
        id,
        x: Number.isFinite(x) ? x : 0,
        y: Number.isFinite(y) ? y : 0,
        qty: Number.isFinite(qty) ? qty : 0,
      },
    ]);
  }
  return Object.fromEntries(entries);
}

export { normalizeWorldConfig, normalizeGroundItems };

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

// sendMessage serializes payloads, applies simulated latency, and tracks stats.
export function sendMessage(store, payload, { onSent } = {}) {
  if (!store.socket || store.socket.readyState !== WebSocket.OPEN) {
    return;
  }
  const messageText = JSON.stringify(payload);
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
    queueEffectTriggers(store, payload.effectTriggers);
    store.worldConfig = normalizeWorldConfig(payload.config);
    store.WORLD_WIDTH = store.worldConfig.width;
    store.WORLD_HEIGHT = store.worldConfig.height;
    if (store.effectManager && typeof store.effectManager.clear === "function") {
      store.effectManager.clear();
    }
    if (typeof store.updateWorldConfigUI === "function") {
      store.updateWorldConfigUI();
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
  store.updateDiagnostics();

  store.socket.onopen = () => {
    store.setStatusBase(`Connected as ${store.playerId}. Use WASD or click to move.`);
    store.setLatency(null);
    sendCurrentIntent(store);
    startHeartbeat(store);
    store.updateDiagnostics();
  };

  store.socket.onmessage = (event) => {
    try {
      const payload = JSON.parse(event.data);
      if (payload.type === "state") {
        store.players = Object.fromEntries(
          payload.players.map((p) => [p.id, { ...p, facing: normalizeFacing(p.facing) }])
        );
        store.npcs = Object.fromEntries(
          Array.isArray(payload.npcs)
            ? payload.npcs.map((npc) => [npc.id, { ...npc, facing: normalizeFacing(npc.facing) }])
            : []
        );
        if (Array.isArray(payload.obstacles)) {
          store.obstacles = payload.obstacles;
        }
        if (Array.isArray(payload.effects)) {
          store.effects = payload.effects;
        } else {
          store.effects = [];
        }
        store.groundItems = normalizeGroundItems(payload.groundItems);
        queueEffectTriggers(store, payload.effectTriggers);
        if (payload.config) {
          store.worldConfig = normalizeWorldConfig(payload.config);
          store.WORLD_WIDTH = store.worldConfig.width;
          store.WORLD_HEIGHT = store.worldConfig.height;
          if (typeof store.updateWorldConfigUI === "function") {
            store.updateWorldConfigUI();
          }
        }
        if (store.players[store.playerId]) {
          store.players[store.playerId].facing = normalizeFacing(
            store.players[store.playerId].facing
          );
          if (!store.displayPlayers[store.playerId]) {
            store.displayPlayers[store.playerId] = {
              x: store.players[store.playerId].x,
              y: store.players[store.playerId].y,
            };
          }
          store.currentFacing = store.players[store.playerId].facing;
        } else {
          store.setStatusBase("Server no longer recognizes this player. Rejoining...");
          handleConnectionLoss(store);
          return;
        }
        Object.values(store.players).forEach((p) => {
          if (!store.displayPlayers[p.id]) {
            store.displayPlayers[p.id] = { x: p.x, y: p.y };
          }
        });
        Object.keys(store.displayPlayers).forEach((id) => {
          if (!store.players[id]) {
            delete store.displayPlayers[id];
          }
        });
        Object.values(store.npcs).forEach((npc) => {
          if (!store.displayNPCs[npc.id]) {
            store.displayNPCs[npc.id] = { x: npc.x, y: npc.y };
          }
        });
        Object.keys(store.displayNPCs).forEach((id) => {
          if (!store.npcs[id]) {
            delete store.displayNPCs[id];
          }
        });
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
        store.updateDiagnostics();
        if (store.renderInventory) {
          store.renderInventory();
        }
      } else if (payload.type === "heartbeat") {
        const now = Date.now();
        const roundTrip = computeRtt(payload, now);
        if (roundTrip !== null) {
          store.setLatency(roundTrip);
          store.lastHeartbeatRoundTrip = roundTrip;
        }
        store.lastHeartbeatAckAt = now;
        store.updateDiagnostics();
      } else if (payload.type === "console_ack") {
        handleConsoleAck(store, payload);
      }
    } catch (err) {
      console.error("Failed to parse event", err);
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
  const payload = {
    type: "input",
    dx: store.currentIntent.dx,
    dy: store.currentIntent.dy,
    facing: store.currentFacing,
  };
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
  const canvas = store.canvas;
  const fallbackWidth = canvas ? canvas.width : store.GRID_WIDTH * store.TILE_SIZE;
  const fallbackHeight = canvas ? canvas.height : store.GRID_HEIGHT * store.TILE_SIZE;
  const worldWidth =
    typeof store.WORLD_WIDTH === "number" ? store.WORLD_WIDTH : fallbackWidth;
  const worldHeight =
    typeof store.WORLD_HEIGHT === "number" ? store.WORLD_HEIGHT : fallbackHeight;
  const maxX = Math.max(store.PLAYER_HALF, worldWidth - store.PLAYER_HALF);
  const maxY = Math.max(store.PLAYER_HALF, worldHeight - store.PLAYER_HALF);
  const clampedX = Math.max(store.PLAYER_HALF, Math.min(x, maxX));
  const clampedY = Math.max(store.PLAYER_HALF, Math.min(y, maxY));
  if (!store.socket || store.socket.readyState !== WebSocket.OPEN) {
    store.activePathTarget = { x: clampedX, y: clampedY };
    store.isPathActive = false;
    return;
  }
  const payload = { type: "path", x: clampedX, y: clampedY };
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
  const payload = { type: "cancelPath" };
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
  const payload = { type: "action", action };
  if (params && typeof params === "object" && Object.keys(params).length > 0) {
    payload.params = params;
  }
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
  const payload = { type: "console", cmd };
  if (params && typeof params === "object") {
    if (Object.prototype.hasOwnProperty.call(params, "qty")) {
      const qtyValue = Number(params.qty);
      if (Number.isFinite(qtyValue)) {
        payload.qty = Math.trunc(qtyValue);
      }
    }
  }
  sendMessage(store, payload);
}

const heartbeatBindings = new WeakMap();

function getHeartbeatBinding(store) {
  let binding = heartbeatBindings.get(store);
  if (binding) {
    return binding;
  }

  const now = () => Date.now();
  binding = { intervalId: null };

  const env = {
    now,
    setInterval: (cb, ms) => {
      const id = setInterval(cb, ms);
      binding.intervalId = id;
      store.heartbeatTimer = id;
      store.updateDiagnostics();
      return id;
    },
    clearInterval: (id) => {
      clearInterval(id);
      if (binding.intervalId === id) {
        binding.intervalId = null;
      }
      store.heartbeatTimer = null;
      store.updateDiagnostics();
    },
    send: () => sendHeartbeatOnce(store, now),
  };

  binding.controller = createHeartbeat(env);
  binding.env = env;
  heartbeatBindings.set(store, binding);
  return binding;
}

// startHeartbeat kicks off the repeating heartbeat timer.
export function startHeartbeat(store) {
  const binding = getHeartbeatBinding(store);
  binding.controller.start(HEARTBEAT_INTERVAL);
}

// stopHeartbeat clears the heartbeat timer if it exists.
export function stopHeartbeat(store) {
  const binding = heartbeatBindings.get(store);
  if (!binding) {
    return;
  }
  binding.controller.stop();
}

function sendHeartbeatOnce(store, nowFn = () => Date.now()) {
  if (!store.socket || store.socket.readyState !== WebSocket.OPEN) {
    return;
  }
  const sentAt = nowFn();
  const payload = {
    type: "heartbeat",
    sentAt,
  };
  sendMessage(store, payload, {
    onSent: () => {
      store.lastHeartbeatSentAt = nowFn();
      store.updateDiagnostics();
    },
  });
}

// sendHeartbeat emits a heartbeat payload and records timing.
export function sendHeartbeat(store) {
  const binding = heartbeatBindings.get(store);
  const nowFn = binding ? binding.env.now : undefined;
  sendHeartbeatOnce(store, nowFn);
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
  store.currentIntent = { dx: 0, dy: 0 };
  store.currentFacing = DEFAULT_FACING;
  store.directionOrder = [];
  store.isPathActive = false;
  store.activePathTarget = null;
  store.lastPathRequestAt = null;
  store.updateDiagnostics();
  if (store.playerId === null) {
    return;
  }
  store.setStatusBase("Connection lost. Rejoining...");
  store.playerId = null;
  store.players = {};
  store.displayPlayers = {};
  store.npcs = {};
  store.displayNPCs = {};
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
