const statusEl = document.getElementById("status");
const canvas = document.getElementById("game-canvas");
const ctx = canvas.getContext("2d");

const tileSize = 32;
let playerId = null;
let world = null;
let players = [];
const pressedKeys = new Set();
let lastMoveSent = 0;

const keyMap = {
  KeyW: { dx: 0, dy: -1 },
  KeyA: { dx: -1, dy: 0 },
  KeyS: { dx: 0, dy: 1 },
  KeyD: { dx: 1, dy: 0 },
};

function setStatus(text) {
  statusEl.textContent = text;
}

function resizeCanvas() {
  if (!world) return;
  canvas.width = world.width * tileSize;
  canvas.height = world.height * tileSize;
}

function drawWorld() {
  if (!world) {
    ctx.fillStyle = "#000";
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    return;
  }

  for (let y = 0; y < world.height; y += 1) {
    for (let x = 0; x < world.width; x += 1) {
      const tile = world.tiles[y * world.width + x];
      ctx.fillStyle = tile === 1 ? "#444" : "#1f1f1f";
      ctx.fillRect(x * tileSize, y * tileSize, tileSize, tileSize);
      if (tile !== 1) {
        ctx.fillStyle = "#252525";
        ctx.fillRect(
          x * tileSize + 4,
          y * tileSize + 4,
          tileSize - 8,
          tileSize - 8,
        );
      }
    }
  }
}

function drawPlayers() {
  if (!players) return;
  for (const p of players) {
    const screenX = p.x * tileSize;
    const screenY = p.y * tileSize;
    ctx.fillStyle = p.id === playerId ? "#ffd166" : "#ef476f";
    ctx.fillRect(screenX + 4, screenY + 4, tileSize - 8, tileSize - 8);
  }
}

function render() {
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  drawWorld();
  drawPlayers();
  requestAnimationFrame(render);
}

async function joinGame() {
  try {
    setStatus("Joining game...");
    const response = await fetch("/join");
    if (!response.ok) {
      throw new Error(`Join failed: ${response.status}`);
    }
    const data = await response.json();
    playerId = data.youId;
    world = data.world;
    players = data.players || [];
    resizeCanvas();
    setStatus(`Connected as ${playerId}`);
    connectStateStream();
  } catch (err) {
    setStatus(`Unable to join: ${err.message}`);
  }
}

function connectStateStream() {
  if (!playerId) return;
  const stream = new EventSource(`/state?id=${encodeURIComponent(playerId)}`);
  stream.onopen = () => setStatus(`Connected as ${playerId}`);
  stream.onerror = () => setStatus("Connection lost. Attempting to reconnect...");
  stream.onmessage = (event) => {
    try {
      const payload = JSON.parse(event.data);
      if (payload.type === "state") {
        world = payload.world;
        players = payload.players;
        resizeCanvas();
      }
    } catch (err) {
      console.error("Failed to parse state", err);
    }
  };
}

function scheduleMovement() {
  const now = performance.now();
  if (now - lastMoveSent < 120) {
    return;
  }
  lastMoveSent = now;

  if (!playerId) return;
  let dx = 0;
  let dy = 0;
  pressedKeys.forEach((code) => {
    const dir = keyMap[code];
    if (dir) {
      dx += dir.dx;
      dy += dir.dy;
    }
  });

  dx = Math.max(-1, Math.min(1, dx));
  dy = Math.max(-1, Math.min(1, dy));

  if (dx === 0 && dy === 0) return;

  fetch("/move", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ id: playerId, dx, dy }),
  }).catch((err) => {
    console.warn("Move failed", err);
  });
}

window.addEventListener("keydown", (event) => {
  if (keyMap[event.code]) {
    event.preventDefault();
    pressedKeys.add(event.code);
    scheduleMovement();
  }
});

window.addEventListener("keyup", (event) => {
  if (keyMap[event.code]) {
    event.preventDefault();
    pressedKeys.delete(event.code);
  }
});

setInterval(scheduleMovement, 150);
render();
joinGame();
