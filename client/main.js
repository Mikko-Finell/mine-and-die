const statusEl = document.getElementById("status");
const canvas = document.getElementById("game-canvas");
const ctx = canvas.getContext("2d");

const TILE_SIZE = 40;
const GRID_WIDTH = canvas.width / TILE_SIZE;
const GRID_HEIGHT = canvas.height / TILE_SIZE;
const PLAYER_SIZE = 28;
const PLAYER_HALF = PLAYER_SIZE / 2;
const HEARTBEAT_INTERVAL = 2000;
const LERP_RATE = 12;

let playerId = null;
let players = {};
let socket = null;
let reconnectTimeout = null;
let isJoining = false;
let displayPlayers = {};
let currentIntent = { dx: 0, dy: 0 };
let heartbeatTimer = null;
let latencyMs = null;

const keys = new Set();
let lastTimestamp = performance.now();

let statusBaseText = "";

function renderStatus() {
  if (latencyMs != null) {
    statusEl.textContent = `${statusBaseText} (latency: ${Math.round(
      latencyMs
    )} ms)`;
  } else {
    statusEl.textContent = statusBaseText;
  }
}

function setStatusBase(text) {
  statusBaseText = text;
  renderStatus();
}

function setLatency(value) {
  latencyMs = value;
  renderStatus();
}

async function joinGame() {
  if (isJoining) return;
  isJoining = true;
  if (reconnectTimeout !== null) {
    clearTimeout(reconnectTimeout);
    reconnectTimeout = null;
  }
  setStatusBase("Joining game...");
  try {
    const response = await fetch("/join", { method: "POST" });
    if (!response.ok) {
      throw new Error(`join failed: ${response.status}`);
    }
    const payload = await response.json();
    playerId = payload.id;
    players = Object.fromEntries(payload.players.map((p) => [p.id, p]));
    if (!players[playerId]) {
      players[playerId] = { id: playerId, x: 80, y: 80 };
    }
    displayPlayers = {};
    Object.values(players).forEach((p) => {
      displayPlayers[p.id] = { x: p.x, y: p.y };
    });
    currentIntent = { dx: 0, dy: 0 };
    setLatency(null);
    setStatusBase(`Connected as ${playerId}. Use WASD to move.`);
    connectEvents();
  } catch (err) {
    setLatency(null);
    setStatusBase(`Unable to join: ${err.message}`);
    setTimeout(joinGame, 1500);
  } finally {
    isJoining = false;
  }
}

function closeSocketSilently() {
  if (!socket) return;
  stopHeartbeat();
  socket.onopen = null;
  socket.onmessage = null;
  socket.onclose = null;
  socket.onerror = null;
  try {
    socket.close();
  } catch (err) {
    console.error("Failed to close socket", err);
  }
  socket = null;
}

function scheduleReconnect() {
  if (reconnectTimeout !== null) return;
  reconnectTimeout = setTimeout(() => {
    reconnectTimeout = null;
    joinGame();
  }, 1000);
}

function handleConnectionLoss() {
  closeSocketSilently();
  setLatency(null);
  if (playerId === null) {
    return;
  }
  setStatusBase("Connection lost. Rejoining...");
  playerId = null;
  players = {};
  displayPlayers = {};
  scheduleReconnect();
}

function connectEvents() {
  if (!playerId) return;
  closeSocketSilently();

  const protocol = window.location.protocol === "https:" ? "wss" : "ws";
  const wsUrl = `${protocol}://${window.location.host}/ws?id=${encodeURIComponent(
    playerId
  )}`;
  socket = new WebSocket(wsUrl);

  socket.onopen = () => {
    setStatusBase(`Connected as ${playerId}. Use WASD to move.`);
    setLatency(null);
    sendCurrentIntent();
    startHeartbeat();
  };

  socket.onmessage = (event) => {
    try {
      const payload = JSON.parse(event.data);
      if (payload.type === "state") {
        players = Object.fromEntries(payload.players.map((p) => [p.id, p]));
        if (players[playerId]) {
          if (!displayPlayers[playerId]) {
            displayPlayers[playerId] = {
              x: players[playerId].x,
              y: players[playerId].y,
            };
          }
        } else {
          setStatusBase("Server no longer recognizes this player. Rejoining...");
          handleConnectionLoss();
          return;
        }
        Object.values(players).forEach((p) => {
          if (!displayPlayers[p.id]) {
            displayPlayers[p.id] = { x: p.x, y: p.y };
          }
        });
        Object.keys(displayPlayers).forEach((id) => {
          if (!players[id]) {
            delete displayPlayers[id];
          }
        });
      } else if (payload.type === "heartbeat") {
        if (typeof payload.rtt === "number") {
          setLatency(Math.max(0, payload.rtt));
        } else if (typeof payload.clientTime === "number") {
          const roundTrip = Date.now() - payload.clientTime;
          setLatency(Math.max(0, roundTrip));
        }
      }
    } catch (err) {
      console.error("Failed to parse event", err);
    }
  };

  const handleSocketDrop = () => {
    handleConnectionLoss();
  };

  socket.onerror = handleSocketDrop;
  socket.onclose = handleSocketDrop;
}

function handleKey(event, isPressed) {
  if (["w", "a", "s", "d"].includes(event.key.toLowerCase())) {
    event.preventDefault();
    if (isPressed) {
      keys.add(event.key.toLowerCase());
    } else {
      keys.delete(event.key.toLowerCase());
    }
    updateIntentFromKeys();
  }
}

document.addEventListener("keydown", (event) => handleKey(event, true));
document.addEventListener("keyup", (event) => handleKey(event, false));

function gameLoop(now) {
  const dt = Math.min((now - lastTimestamp) / 1000, 0.2);
  lastTimestamp = now;

  const lerpAmount = Math.min(1, dt * LERP_RATE);
  Object.entries(players).forEach(([id, player]) => {
    if (!displayPlayers[id]) {
      displayPlayers[id] = { x: player.x, y: player.y };
    }
    const display = displayPlayers[id];
    display.x += (player.x - display.x) * lerpAmount;
    display.y += (player.y - display.y) * lerpAmount;
  });

  Object.keys(displayPlayers).forEach((id) => {
    if (!players[id]) {
      delete displayPlayers[id];
    }
  });

  drawScene();
  requestAnimationFrame(gameLoop);
}

function drawScene() {
  ctx.fillStyle = "#0f172a";
  ctx.fillRect(0, 0, canvas.width, canvas.height);

  ctx.strokeStyle = "#1e293b";
  ctx.lineWidth = 1;
  for (let x = 0; x <= GRID_WIDTH; x++) {
    ctx.beginPath();
    ctx.moveTo(x * TILE_SIZE, 0);
    ctx.lineTo(x * TILE_SIZE, canvas.height);
    ctx.stroke();
  }
  for (let y = 0; y <= GRID_HEIGHT; y++) {
    ctx.beginPath();
    ctx.moveTo(0, y * TILE_SIZE);
    ctx.lineTo(canvas.width, y * TILE_SIZE);
    ctx.stroke();
  }

  Object.entries(displayPlayers).forEach(([id, position]) => {
    ctx.fillStyle = id === playerId ? "#38bdf8" : "#f97316";
    ctx.fillRect(
      position.x - PLAYER_HALF,
      position.y - PLAYER_HALF,
      PLAYER_SIZE,
      PLAYER_SIZE
    );
  });
}

function updateIntentFromKeys() {
  let dx = 0;
  let dy = 0;
  if (keys.has("w")) dy -= 1;
  if (keys.has("s")) dy += 1;
  if (keys.has("a")) dx -= 1;
  if (keys.has("d")) dx += 1;

  if (dx !== 0 || dy !== 0) {
    const length = Math.hypot(dx, dy) || 1;
    dx /= length;
    dy /= length;
  }

  if (dx === currentIntent.dx && dy === currentIntent.dy) {
    return;
  }

  currentIntent = { dx, dy };
  sendCurrentIntent();
}

function sendCurrentIntent() {
  if (!socket || socket.readyState !== WebSocket.OPEN) {
    return;
  }
  socket.send(
    JSON.stringify({
      type: "input",
      dx: currentIntent.dx,
      dy: currentIntent.dy,
    })
  );
}

function startHeartbeat() {
  stopHeartbeat();
  sendHeartbeat();
  heartbeatTimer = setInterval(sendHeartbeat, HEARTBEAT_INTERVAL);
}

function stopHeartbeat() {
  if (heartbeatTimer !== null) {
    clearInterval(heartbeatTimer);
    heartbeatTimer = null;
  }
}

function sendHeartbeat() {
  if (!socket || socket.readyState !== WebSocket.OPEN) {
    return;
  }
  const sentAt = Date.now();
  socket.send(
    JSON.stringify({
      type: "heartbeat",
      sentAt,
    })
  );
}
requestAnimationFrame(gameLoop);
joinGame();
