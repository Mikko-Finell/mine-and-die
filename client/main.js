const statusEl = document.getElementById("status");
const canvas = document.getElementById("game-canvas");
const ctx = canvas.getContext("2d");

const TILE_SIZE = 40;
const GRID_WIDTH = canvas.width / TILE_SIZE;
const GRID_HEIGHT = canvas.height / TILE_SIZE;
const PLAYER_SIZE = 28;
const PLAYER_HALF = PLAYER_SIZE / 2;
const MOVE_SPEED = 160; // pixels per second

let playerId = null;
let players = {};
let eventSource = null;
let pendingSync = false;
let pendingPosition = { x: 80, y: 80 };
let lastSentPosition = { x: 80, y: 80 };
let isJoining = false;

const keys = new Set();
let lastTimestamp = performance.now();

function clamp(value, min, max) {
  return Math.min(Math.max(value, min), max);
}

function updateStatus(text) {
  statusEl.textContent = text;
}

async function joinGame() {
  if (isJoining) return;
  isJoining = true;
  updateStatus("Joining game...");
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
    pendingPosition = { x: players[playerId].x, y: players[playerId].y };
    lastSentPosition = { ...pendingPosition };
    updateStatus(`Connected as ${playerId}. Use WASD to move.`);
    connectEvents();
  } catch (err) {
    updateStatus(`Unable to join: ${err.message}`);
    setTimeout(joinGame, 1500);
  } finally {
    isJoining = false;
  }
}

function connectEvents() {
  if (!playerId) return;
  if (eventSource) {
    eventSource.close();
  }

  eventSource = new EventSource(`/events?id=${encodeURIComponent(playerId)}`);
  eventSource.onmessage = (event) => {
    try {
      const payload = JSON.parse(event.data);
      if (payload.type === "state") {
        players = Object.fromEntries(payload.players.map((p) => [p.id, p]));
        if (players[playerId]) {
          pendingPosition = { x: players[playerId].x, y: players[playerId].y };
        }
      }
    } catch (err) {
      console.error("Failed to parse event", err);
    }
  };

  eventSource.onerror = () => {
    updateStatus("Connection lost. Rejoining...");
    if (eventSource) {
      eventSource.close();
      eventSource = null;
    }
    playerId = null;
    players = {};
    pendingSync = false;
    setTimeout(joinGame, 1000);
  };
}

function handleKey(event, isPressed) {
  if (["w", "a", "s", "d"].includes(event.key.toLowerCase())) {
    event.preventDefault();
    if (isPressed) {
      keys.add(event.key.toLowerCase());
    } else {
      keys.delete(event.key.toLowerCase());
    }
  }
}

document.addEventListener("keydown", (event) => handleKey(event, true));
document.addEventListener("keyup", (event) => handleKey(event, false));

function gameLoop(now) {
  const dt = Math.min((now - lastTimestamp) / 1000, 0.2);
  lastTimestamp = now;

  if (playerId && players[playerId]) {
    const player = players[playerId];
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
      const nextX = clamp(
        player.x + dx * MOVE_SPEED * dt,
        PLAYER_HALF,
        canvas.width - PLAYER_HALF
      );
      const nextY = clamp(
        player.y + dy * MOVE_SPEED * dt,
        PLAYER_HALF,
        canvas.height - PLAYER_HALF
      );
      if (nextX !== player.x || nextY !== player.y) {
        player.x = nextX;
        player.y = nextY;
        pendingPosition = { x: nextX, y: nextY };
        pendingSync = true;
      }
    }
  }

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

  Object.values(players).forEach((player) => {
    ctx.fillStyle = player.id === playerId ? "#38bdf8" : "#f97316";
    ctx.fillRect(
      player.x - PLAYER_HALF,
      player.y - PLAYER_HALF,
      PLAYER_SIZE,
      PLAYER_SIZE
    );
  });
}

async function syncPosition() {
  if (!pendingSync || !playerId) {
    return;
  }
  const { x, y } = pendingPosition;
  if (x === lastSentPosition.x && y === lastSentPosition.y) {
    pendingSync = false;
    return;
  }

  try {
    await fetch("/move", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ id: playerId, x, y }),
      keepalive: true,
    });
    lastSentPosition = { x, y };
  } catch (err) {
    console.error("Failed to sync position", err);
  }

  pendingSync = false;
}

setInterval(syncPosition, 100);
requestAnimationFrame(gameLoop);
joinGame();
