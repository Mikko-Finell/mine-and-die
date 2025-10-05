const DEFAULT_FACING = "down";
const FACING_OFFSETS = {
  up: { x: 0, y: -1 },
  down: { x: 0, y: 1 },
  left: { x: -1, y: 0 },
  right: { x: 1, y: 0 },
};

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
    ctx.fillStyle = "#334155";
    ctx.fillRect(obstacle.x, obstacle.y, obstacle.width, obstacle.height);
    ctx.strokeStyle = "#475569";
    ctx.strokeRect(obstacle.x, obstacle.y, obstacle.width, obstacle.height);
  });

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
