const HEARTBEAT_INTERVAL = 2000;
const DEFAULT_FACING = "down";
const VALID_FACINGS = new Set(["up", "down", "left", "right"]);

// normalizeFacing guards against invalid facing values from the network.
function normalizeFacing(facing) {
  return typeof facing === "string" && VALID_FACINGS.has(facing)
    ? facing
    : DEFAULT_FACING;
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
    store.setLatency(null);
    store.setStatusBase(`Connected as ${store.playerId}. Use WASD to move.`);
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
    store.setStatusBase(`Connected as ${store.playerId}. Use WASD to move.`);
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
  scheduleReconnect(store);
}
