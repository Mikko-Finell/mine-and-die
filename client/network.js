const HEARTBEAT_INTERVAL = 2000;
const DEFAULT_FACING = "down";
export const DEFAULT_WORLD_SEED = "prototype";
const DEFAULT_OBSTACLE_COUNT = 2;
const DEFAULT_GOLD_MINE_COUNT = 1;
const DEFAULT_GOBLIN_COUNT = 2;
const DEFAULT_RAT_COUNT = 1;
const DEFAULT_NPC_COUNT = DEFAULT_GOBLIN_COUNT + DEFAULT_RAT_COUNT;
const DEFAULT_LAVA_COUNT = 3;
const VALID_FACINGS = new Set(["up", "down", "left", "right"]);

function normalizeCount(value, fallback) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) {
    return fallback;
  }
  return Math.max(0, Math.floor(parsed));
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
  const goblinProvided = Object.prototype.hasOwnProperty.call(
    config,
    "goblinCount",
  );
  if (goblinProvided) {
    normalized.goblinCount = normalizeCount(
      config.goblinCount,
      normalized.goblinCount,
    );
  }
  const ratProvided = Object.prototype.hasOwnProperty.call(
    config,
    "ratCount",
  );
  if (ratProvided) {
    normalized.ratCount = normalizeCount(config.ratCount, normalized.ratCount);
  }
  let totalProvided = null;
  if (Object.prototype.hasOwnProperty.call(config, "npcCount")) {
    totalProvided = normalizeCount(
      config.npcCount,
      normalized.goblinCount + normalized.ratCount,
    );
  }
  if (totalProvided !== null && !goblinProvided && !ratProvided) {
    const goblins = Math.min(2, totalProvided);
    const rats = Math.max(0, totalProvided - goblins);
    normalized.goblinCount = goblins;
    normalized.ratCount = rats;
  }
  normalized.npcCount = normalized.goblinCount + normalized.ratCount;
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

  return normalized;
}

// normalizeFacing guards against invalid facing values from the network.
function normalizeFacing(facing) {
  return typeof facing === "string" && VALID_FACINGS.has(facing)
    ? facing
    : DEFAULT_FACING;
}

function ensureEffectTriggerState(store) {
  if (!Array.isArray(store.pendingEffectTriggers)) {
    store.pendingEffectTriggers = [];
  }
  if (!(store.processedEffectTriggerIds instanceof Set)) {
    store.processedEffectTriggerIds = new Set();
  }
}

function queueEffectTriggers(store, triggers) {
  if (!Array.isArray(triggers) || triggers.length === 0) {
    return;
  }
  ensureEffectTriggerState(store);
  for (const trigger of triggers) {
    if (!trigger || typeof trigger !== "object") {
      continue;
    }
    const id = typeof trigger.id === "string" ? trigger.id : null;
    if (id && store.processedEffectTriggerIds.has(id)) {
      continue;
    }
    store.pendingEffectTriggers.push(trigger);
    if (id) {
      store.processedEffectTriggerIds.add(id);
    }
  }
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
    store.pendingEffectTriggers = [];
    store.processedEffectTriggerIds = new Set();
    queueEffectTriggers(store, payload.effectTriggers);
    store.worldConfig = normalizeWorldConfig(payload.config);
    if (store.effectManager && typeof store.effectManager.clear === "function") {
      store.effectManager.clear();
    }
    if (typeof store.updateWorldConfigUI === "function") {
      store.updateWorldConfigUI();
    }
    if (!store.players[store.playerId]) {
      store.players[store.playerId] = {
        id: store.playerId,
        x: 80,
        y: 80,
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
        queueEffectTriggers(store, payload.effectTriggers);
        if (payload.config) {
          store.worldConfig = normalizeWorldConfig(payload.config);
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
        if (typeof payload.rtt === "number") {
          const roundTrip = Math.max(0, payload.rtt);
          store.setLatency(roundTrip);
          store.lastHeartbeatRoundTrip = roundTrip;
        } else if (typeof payload.clientTime === "number") {
          const roundTrip = Date.now() - payload.clientTime;
          store.setLatency(Math.max(0, roundTrip));
          store.lastHeartbeatRoundTrip = Math.max(0, roundTrip);
        }
        store.lastHeartbeatAckAt = Date.now();
        store.updateDiagnostics();
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

// startHeartbeat kicks off the repeating heartbeat timer.
export function startHeartbeat(store) {
  stopHeartbeat(store);
  sendHeartbeat(store);
  store.heartbeatTimer = setInterval(() => sendHeartbeat(store), HEARTBEAT_INTERVAL);
  store.updateDiagnostics();
}

// stopHeartbeat clears the heartbeat timer if it exists.
export function stopHeartbeat(store) {
  if (store.heartbeatTimer !== null) {
    clearInterval(store.heartbeatTimer);
    store.heartbeatTimer = null;
    store.updateDiagnostics();
  }
}

// sendHeartbeat emits a heartbeat payload and records timing.
export function sendHeartbeat(store) {
  if (!store.socket || store.socket.readyState !== WebSocket.OPEN) {
    return;
  }
  const sentAt = Date.now();
  const payload = {
    type: "heartbeat",
    sentAt,
  };
  sendMessage(store, payload, {
    onSent: () => {
      store.lastHeartbeatSentAt = Date.now();
      store.updateDiagnostics();
    },
  });
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
  const parseCount = (value, fallback) => normalizeCount(value, fallback);
  const configObject =
    config && typeof config === "object" ? config : Object.create(null);
  const hasGoblinCount = Object.prototype.hasOwnProperty.call(
    configObject,
    "goblinCount",
  );
  const hasRatCount = Object.prototype.hasOwnProperty.call(
    configObject,
    "ratCount",
  );
  let goblinCount = hasGoblinCount
    ? parseCount(config?.goblinCount, DEFAULT_GOBLIN_COUNT)
    : DEFAULT_GOBLIN_COUNT;
  let ratCount = hasRatCount
    ? parseCount(config?.ratCount, DEFAULT_RAT_COUNT)
    : DEFAULT_RAT_COUNT;
  if (!hasGoblinCount && !hasRatCount) {
    const total = parseCount(config?.npcCount, DEFAULT_NPC_COUNT);
    const goblins = Math.min(2, total);
    goblinCount = goblins;
    ratCount = Math.max(0, total - goblins);
  }
  const npcTotal = goblinCount + ratCount;
  const payload = {
    obstacles: !!config?.obstacles,
    obstaclesCount: parseCount(
      config?.obstaclesCount,
      DEFAULT_OBSTACLE_COUNT,
    ),
    goldMines: !!config?.goldMines,
    goldMineCount: parseCount(
      config?.goldMineCount,
      DEFAULT_GOLD_MINE_COUNT,
    ),
    npcs: !!config?.npcs,
    goblinCount,
    ratCount,
    npcCount: npcTotal,
    lava: !!config?.lava,
    lavaCount: parseCount(config?.lavaCount, DEFAULT_LAVA_COUNT),
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
