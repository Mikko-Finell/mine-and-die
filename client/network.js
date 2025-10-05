const HEARTBEAT_INTERVAL = 2000;
const DEFAULT_FACING = "down";
const VALID_FACINGS = new Set(["up", "down", "left", "right"]);

function normalizeFacing(facing) {
  return typeof facing === "string" && VALID_FACINGS.has(facing)
    ? facing
    : DEFAULT_FACING;
}

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
    store.obstacles = Array.isArray(payload.obstacles) ? payload.obstacles : [];
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
    store.currentIntent = { dx: 0, dy: 0 };
    store.currentFacing = normalizeFacing(store.players[store.playerId].facing);
    store.directionOrder = [];
    store.setLatency(null);
    store.setStatusBase(`Connected as ${store.playerId}. Use WASD to move.`);
    connectEvents(store);
    store.updateDiagnostics();
  } catch (err) {
    store.setLatency(null);
    store.setStatusBase(`Unable to join: ${err.message}`);
    setTimeout(() => joinGame(store), 1500);
  } finally {
    store.isJoining = false;
  }
}

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
        if (Array.isArray(payload.obstacles)) {
          store.obstacles = payload.obstacles;
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
        store.lastStateReceivedAt = Date.now();
        store.updateDiagnostics();
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

export function startHeartbeat(store) {
  stopHeartbeat(store);
  sendHeartbeat(store);
  store.heartbeatTimer = setInterval(() => sendHeartbeat(store), HEARTBEAT_INTERVAL);
  store.updateDiagnostics();
}

export function stopHeartbeat(store) {
  if (store.heartbeatTimer !== null) {
    clearInterval(store.heartbeatTimer);
    store.heartbeatTimer = null;
    store.updateDiagnostics();
  }
}

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

function scheduleReconnect(store) {
  if (store.reconnectTimeout !== null) return;
  store.reconnectTimeout = setTimeout(() => {
    store.reconnectTimeout = null;
    joinGame(store);
  }, 1000);
}

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
  scheduleReconnect(store);
}
