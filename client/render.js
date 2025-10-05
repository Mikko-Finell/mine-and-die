const DEFAULT_FACING = "down";
const FACING_OFFSETS = {
  up: { x: 0, y: -1 },
  down: { x: 0, y: 1 },
  left: { x: -1, y: 0 },
  right: { x: 1, y: 0 },
};

const EFFECT_STYLES = {
  attack: {
    fill: "rgba(239, 68, 68, 0.25)",
    stroke: "rgba(239, 68, 68, 0.8)",
  },
  fireball: {
    fill: "rgba(251, 191, 36, 0.35)",
    stroke: "rgba(251, 146, 60, 0.95)",
  },
};

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

    drawScene(store);
    requestAnimationFrame(gameLoop);
  }

  requestAnimationFrame(gameLoop);
}

// drawScene paints the background, obstacles, effects, and players.
function drawScene(store) {
  const { ctx, canvas } = store;
  ctx.fillStyle = "#0f172a";
  ctx.fillRect(0, 0, canvas.width, canvas.height);

  ctx.strokeStyle = "#1e293b";
  ctx.lineWidth = 1;
  for (let x = 0; x <= store.GRID_WIDTH; x++) {
    ctx.beginPath();
    ctx.moveTo(x * store.TILE_SIZE, 0);
    ctx.lineTo(x * store.TILE_SIZE, canvas.height);
    ctx.stroke();
  }
  for (let y = 0; y <= store.GRID_HEIGHT; y++) {
    ctx.beginPath();
    ctx.moveTo(0, y * store.TILE_SIZE);
    ctx.lineTo(canvas.width, y * store.TILE_SIZE);
    ctx.stroke();
  }

  store.obstacles.forEach((obstacle) => {
    drawObstacle(ctx, obstacle);
  });

  drawEffects(store);

  Object.entries(store.displayPlayers).forEach(([id, position]) => {
    ctx.fillStyle = id === store.playerId ? "#38bdf8" : "#f97316";
    ctx.fillRect(
      position.x - store.PLAYER_HALF,
      position.y - store.PLAYER_HALF,
      store.PLAYER_SIZE,
      store.PLAYER_SIZE
    );

    const player = store.players[id];
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
}

// drawEffects renders translucent rectangles for every active effect.
function drawEffects(store) {
  const { ctx } = store;
  if (!Array.isArray(store.effects) || store.effects.length === 0) {
    return;
  }
  store.effects.forEach((effect) => {
    if (!effect || typeof effect !== "object") {
      return;
    }
    const style = EFFECT_STYLES[effect.type];
    if (!style) {
      return;
    }
    const { x, y, width, height } = effect;
    if (
      typeof x !== "number" ||
      typeof y !== "number" ||
      typeof width !== "number" ||
      typeof height !== "number"
    ) {
      return;
    }
    ctx.save();
    ctx.fillStyle = style.fill;
    ctx.strokeStyle = style.stroke;
    ctx.lineWidth = 2;
    ctx.fillRect(x, y, width, height);
    ctx.strokeRect(x, y, width, height);
    ctx.restore();
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

  const nuggetColors = ["#facc15", "#fde68a", "#eab308"];
  const rng = createObstacleRng(obstacle);
  const nuggetCount = Math.max(4, Math.round((width * height) / 350));

  for (let i = 0; i < nuggetCount; i++) {
    const radiusBase = Math.min(width, height) * (0.08 + rng() * 0.12);
    const radiusX = Math.max(2, radiusBase * (0.9 + rng() * 0.6));
    const radiusY = Math.max(2, radiusBase * (0.6 + rng() * 0.5));
    const nuggetX = clampValue(x + radiusX + rng() * (width - radiusX * 2), x, x + width);
    const nuggetY = clampValue(y + radiusY + rng() * (height - radiusY * 2), y, y + height);

    ctx.beginPath();
    ctx.ellipse(
      nuggetX,
      nuggetY,
      radiusX,
      radiusY,
      rng() * Math.PI,
      0,
      Math.PI * 2
    );
    ctx.fillStyle = nuggetColors[i % nuggetColors.length];
    ctx.fill();

    ctx.lineWidth = 1;
    ctx.strokeStyle = "rgba(250, 204, 21, 0.5)";
    ctx.stroke();
  }

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
