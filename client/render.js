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

  Object.entries(store.displayPlayers).forEach(([id, position]) => {
    ctx.fillStyle = id === store.playerId ? "#38bdf8" : "#f97316";
    ctx.fillRect(
      position.x - store.PLAYER_HALF,
      position.y - store.PLAYER_HALF,
      store.PLAYER_SIZE,
      store.PLAYER_SIZE
    );
  });
}
