import { EffectManager } from "../../js-effects/manager.js";
import { createPatchState } from "../../patches.js";
import { RENDER_MODE_SNAPSHOT } from "../../render-modes.js";

function createMockContext() {
  const noop = () => {};
  const gradient = { addColorStop: noop };
  return {
    canvas: { width: 1280, height: 720 },
    fillStyle: "#000000",
    strokeStyle: "#000000",
    lineWidth: 1,
    globalAlpha: 1,
    font: "",
    textAlign: "left",
    textBaseline: "alphabetic",
    save: noop,
    restore: noop,
    translate: noop,
    rotate: noop,
    scale: noop,
    beginPath: noop,
    closePath: noop,
    moveTo: noop,
    lineTo: noop,
    stroke: noop,
    fill: noop,
    fillRect: noop,
    strokeRect: noop,
    clearRect: noop,
    arc: noop,
    ellipse: noop,
    quadraticCurveTo: noop,
    fillText: noop,
    createLinearGradient: () => gradient,
    drawImage: noop,
  };
}

export function createEffectTestStore({
  player = {
    id: "player-1",
    x: 480,
    y: 320,
    facing: "down",
    maxHealth: 100,
    health: 100,
  },
  world = { width: 2400, height: 1800 },
} = {}) {
  const ctx = createMockContext();
  const canvas = ctx.canvas;
  const effectManager = new EffectManager();

  const store = {
    statusEl: null,
    canvas,
    ctx,
    TILE_SIZE: 40,
    GRID_WIDTH: 60,
    GRID_HEIGHT: 45,
    WORLD_WIDTH: world.width,
    WORLD_HEIGHT: world.height,
    PLAYER_SIZE: 28,
    PLAYER_HALF: 14,
    LERP_RATE: 12,
    statusBaseText: "",
    latencyMs: null,
    simulatedLatencyMs: 0,
    playerId: player?.id ?? null,
    currentFacing: player?.facing ?? "down",
    isPathActive: false,
    activePathTarget: null,
    renderMode: RENDER_MODE_SNAPSHOT,
    keyframeInterval: null,
    defaultKeyframeInterval: null,
    lastTimestamp: performance.now(),
    keys: new Set(),
    directionOrder: [],
    lastStateReceivedAt: null,
    lastTick: 0,
    effectManager,
    effectInstancesById: new Map(),
    pendingEffectTriggers: [],
    processedEffectTriggerIds: new Set(),
    patchState: createPatchState(),
    camera: { x: 0, y: 0, lockOnPlayer: true },
    inventorySlotCount: 20,
    worldConfig: { width: world.width, height: world.height },
    obstacles: [],
    groundItems: {},
    renderInventory: null,
    updateDiagnostics: () => {},
    setStatusBase: () => {},
    setLatency: () => {},
    updateWorldConfigUI: () => {},
    updateRenderModeUI: () => {},
    lastEffectLifecycleSummary: null,
    displayPlayers: {},
    displayNPCs: {},
    players: {},
    npcs: {},
  };

  if (player && typeof player.id === "string") {
    store.players[player.id] = { ...player };
    store.displayPlayers[player.id] = { x: player.x, y: player.y };
  }

  return store;
}
