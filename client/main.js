import {
  joinGame,
  resetWorld,
  sendMoveTo,
  sendConsoleCommand,
  DEFAULT_WORLD_SEED,
  DEFAULT_WORLD_WIDTH,
  DEFAULT_WORLD_HEIGHT,
} from "./network.js";
import { createPatchState } from "./patches.js";
import {
  startRenderLoop,
  RENDER_MODE_PATCH,
  RENDER_MODE_SNAPSHOT,
} from "./render.js";
import { registerInputHandlers } from "./input.js";
import { createVendorBanner } from "./vendor/example-banner.js";

console.debug(createVendorBanner("example-banner"));

const statusEl = document.getElementById("status");
const latencyDisplay = document.getElementById("latency-display");
const debugPanel = document.getElementById("debug-panel");
const debugPanelBody = document.getElementById("debug-panel-body");
const debugPanelToggle = document.getElementById("debug-panel-toggle");
const canvas = document.getElementById("game-canvas");
const ctx = canvas.getContext("2d");
const latencyInput = document.getElementById("latency-input");
const inventoryPanel = document.getElementById("inventory-panel");
const inventoryGrid = document.getElementById("inventory-grid");
const hudTick = document.getElementById("hud-tick");
const hudRtt = document.getElementById("hud-rtt");
const worldResetForm = document.getElementById("world-reset-form");
const worldResetStatus = document.getElementById("world-reset-status");
const worldResetObstacles = document.getElementById("world-reset-obstacles");
const worldResetObstaclesCount = document.getElementById(
  "world-reset-obstacles-count",
);
const worldResetNPCs = document.getElementById("world-reset-npcs");
const worldResetGoblinsCount = document.getElementById(
  "world-reset-goblins-count",
);
const worldResetRatsCount = document.getElementById("world-reset-rats-count");
const worldResetLava = document.getElementById("world-reset-lava");
const worldResetLavaCount = document.getElementById("world-reset-lava-count");
const worldResetGoldMines = document.getElementById("world-reset-gold-mines");
const worldResetGoldMineCount = document.getElementById(
  "world-reset-gold-mines-count",
);
const worldResetSeed = document.getElementById("world-reset-seed");

const diagnosticsEls = {
  connection: document.getElementById("diag-connection"),
  players: document.getElementById("diag-players"),
  stateAge: document.getElementById("diag-state-age"),
  tick: document.getElementById("diag-tick"),
  intent: document.getElementById("diag-intent"),
  intentAge: document.getElementById("diag-intent-age"),
  heartbeat: document.getElementById("diag-heartbeat"),
  latency: document.getElementById("diag-latency"),
  simLatency: document.getElementById("diag-sim-latency"),
  messages: document.getElementById("diag-messages"),
  patchBaseline: document.getElementById("diag-patch-baseline"),
  patchBatch: document.getElementById("diag-patch-batch"),
  patchEntities: document.getElementById("diag-patch-entities"),
  patchRecovery: document.getElementById("diag-patch-recovery"),
};

const hudNetworkEls = {
  tick: hudTick,
  rtt: hudRtt,
};

const DEFAULT_INVENTORY_SLOTS = 4;

const ITEM_METADATA = {
  gold: { name: "Gold Coin", icon: "🪙" },
  health_potion: { name: "Lesser Healing Potion", icon: "🧪" },
  rat_tail: { name: "Rat Tail", icon: "🐀" },
};

const WORLD_RESET_TOGGLE_KEYS = ["obstacles", "npcs", "lava", "goldMines"];
const WORLD_RESET_COUNT_KEYS = [
  "obstaclesCount",
  "goblinCount",
  "ratCount",
  "lavaCount",
  "goldMineCount",
];
const WORLD_RESET_COUNT_BY_TOGGLE = {
  obstacles: ["obstaclesCount"],
  npcs: ["goblinCount", "ratCount"],
  lava: ["lavaCount"],
  goldMines: ["goldMineCount"],
};
const WORLD_RESET_CONFIG_KEYS = [
  ...WORLD_RESET_TOGGLE_KEYS,
  ...WORLD_RESET_COUNT_KEYS,
  "seed",
];

const DEFAULT_WORLD_CONFIG = {
  obstacles: true,
  obstaclesCount: 2,
  goldMines: true,
  goldMineCount: 1,
  npcs: true,
  goblinCount: 2,
  ratCount: 1,
  npcCount: 3,
  lava: true,
  lavaCount: 3,
  seed: DEFAULT_WORLD_SEED,
  width: DEFAULT_WORLD_WIDTH,
  height: DEFAULT_WORLD_HEIGHT,
};

const store = {
  statusEl,
  canvas,
  ctx,
  latencyInput,
  latencyDisplay,
  debugPanel,
  debugPanelBody,
  debugPanelToggle,
  diagnosticsEls,
  hudNetworkEls,
  inventoryPanel,
  inventoryGrid,
  worldResetForm,
  worldResetStatusEl: worldResetStatus,
  worldResetInputs: {
    obstacles: worldResetObstacles,
    obstaclesCount: worldResetObstaclesCount,
    npcs: worldResetNPCs,
    goblinCount: worldResetGoblinsCount,
    ratCount: worldResetRatsCount,
    lava: worldResetLava,
    lavaCount: worldResetLavaCount,
    goldMines: worldResetGoldMines,
    goldMineCount: worldResetGoldMineCount,
    seed: worldResetSeed,
  },
  worldResetDirtyFields: {
    obstacles: false,
    obstaclesCount: false,
    npcs: false,
    goblinCount: false,
    ratCount: false,
    lava: false,
    lavaCount: false,
    goldMines: false,
    goldMineCount: false,
    seed: false,
  },
  TILE_SIZE: 40,
  GRID_WIDTH: canvas.width / 40,
  GRID_HEIGHT: canvas.height / 40,
  WORLD_WIDTH: DEFAULT_WORLD_CONFIG.width,
  WORLD_HEIGHT: DEFAULT_WORLD_CONFIG.height,
  PLAYER_SIZE: 28,
  PLAYER_HALF: 28 / 2,
  LERP_RATE: 12,
  statusBaseText: "Preparing session…",
  latencyMs: null,
  simulatedLatencyMs: 0,
  playerId: null,
  players: {},
  displayPlayers: {},
  npcs: {},
  displayNPCs: {},
  obstacles: [],
  groundItems: {},
  effectManager: null,
  socket: null,
  reconnectTimeout: null,
  isJoining: false,
  currentIntent: { dx: 0, dy: 0 },
  currentFacing: "down",
  isPathActive: false,
  activePathTarget: null,
  renderMode: RENDER_MODE_SNAPSHOT,
  heartbeatTimer: null,
  lastTimestamp: performance.now(),
  latencyInputListener: null,
  keys: new Set(),
  directionOrder: [],
  lastStateReceivedAt: null,
  lastIntentSentAt: null,
  lastHeartbeatSentAt: null,
  lastHeartbeatAckAt: null,
  lastHeartbeatRoundTrip: null,
  lastMessageSentAt: null,
  messagesSent: 0,
  bytesSent: 0,
  lastPathRequestAt: null,
  lastTick: null,
  effects: [],
  pendingEffectTriggers: [],
  processedEffectTriggerIds: new Set(),
  keyframeRetryTimer: null,
  camera: {
    x: 0,
    y: 0,
    lockOnPlayer: true,
  },
  inventorySlotCount: DEFAULT_INVENTORY_SLOTS,
  worldConfig: { ...DEFAULT_WORLD_CONFIG },
  isResettingWorld: false,
  updateWorldConfigUI: null,
  patchState: createPatchState(),
  lastWorldResetAt: null,
  lastConsoleAck: null,
};

window.debugDropGold = (qty) => {
  const amount = Number(qty);
  if (!Number.isFinite(amount) || amount <= 0) {
    console.warn("debugDropGold expects a positive number");
    return;
  }
  sendConsoleCommand(store, "drop_gold", { qty: Math.trunc(amount) });
};

window.debugPickupGold = () => {
  sendConsoleCommand(store, "pickup_gold");
};

window.debugNetworkStats = () => {
  const tickValue =
    typeof store.lastTick === "number" && Number.isFinite(store.lastTick)
      ? Math.floor(store.lastTick)
      : null;
  const rttValueRaw =
    typeof store.lastHeartbeatRoundTrip === "number" &&
    Number.isFinite(store.lastHeartbeatRoundTrip)
      ? store.lastHeartbeatRoundTrip
      : typeof store.latencyMs === "number" && Number.isFinite(store.latencyMs)
        ? store.latencyMs
        : null;
  const rttValue =
    rttValueRaw != null ? Math.max(0, Math.round(rttValueRaw)) : null;
  const tickLabel = formatTickLabel(tickValue);
  const rttLabel = formatRttLabel(rttValue);
  console.info(`[network] ${tickLabel} · ${rttLabel}`);
  return { tick: tickValue, rttMs: rttValue };
};

const VALID_RENDER_MODES = new Set([
  RENDER_MODE_SNAPSHOT,
  RENDER_MODE_PATCH,
]);

function normalizeRenderMode(value) {
  if (typeof value !== "string") {
    return null;
  }
  const normalized = value.trim().toLowerCase();
  if (normalized === RENDER_MODE_PATCH) {
    return RENDER_MODE_PATCH;
  }
  if (normalized === RENDER_MODE_SNAPSHOT) {
    return RENDER_MODE_SNAPSHOT;
  }
  return null;
}

function applyRenderMode(nextMode) {
  const resolved =
    typeof nextMode === "string" ? normalizeRenderMode(nextMode) : nextMode;
  const finalMode = VALID_RENDER_MODES.has(resolved)
    ? resolved
    : null;
  if (!finalMode) {
    console.warn(
      `[render] Unknown mode "${nextMode}". Expected ` +
        `${Array.from(VALID_RENDER_MODES).join(" or ")}.`,
    );
    return store.renderMode;
  }
  if (store.renderMode === finalMode) {
    console.info(`[render] Already using ${finalMode} rendering.`);
    return finalMode;
  }
  store.renderMode = finalMode;
  store.displayPlayers = {};
  store.displayNPCs = {};
  console.info(`[render] Switched to ${finalMode} rendering.`);
  if (typeof store.updateDiagnostics === "function") {
    store.updateDiagnostics();
  }
  return finalMode;
}

store.setRenderMode = applyRenderMode;

window.debugSetRenderMode = (mode) => applyRenderMode(mode);
window.debugToggleRenderMode = () =>
  applyRenderMode(
    store.renderMode === RENDER_MODE_PATCH
      ? RENDER_MODE_SNAPSHOT
      : RENDER_MODE_PATCH,
  );

const clamp = (value, min, max) => Math.min(max, Math.max(min, value));

function initializeCanvasPathing() {
  if (!store.canvas) {
    return;
  }
  const handlePointerDown = (event) => {
    if (event.button !== 0) {
      return;
    }
    event.preventDefault();
    const rect = store.canvas.getBoundingClientRect();
    const scaleX = store.canvas.width / rect.width;
    const scaleY = store.canvas.height / rect.height;
    const localX = (event.clientX - rect.left) * scaleX;
    const localY = (event.clientY - rect.top) * scaleY;
    const cameraX = store.camera?.x || 0;
    const cameraY = store.camera?.y || 0;
    const worldWidth =
      typeof store.WORLD_WIDTH === "number"
        ? store.WORLD_WIDTH
        : store.canvas.width;
    const worldHeight =
      typeof store.WORLD_HEIGHT === "number"
        ? store.WORLD_HEIGHT
        : store.canvas.height;
    const x = clamp(
      cameraX + localX,
      store.PLAYER_HALF,
      worldWidth - store.PLAYER_HALF,
    );
    const y = clamp(
      cameraY + localY,
      store.PLAYER_HALF,
      worldHeight - store.PLAYER_HALF,
    );
    sendMoveTo(store, x, y);
  };
  store.canvas.addEventListener("pointerdown", handlePointerDown);
  store.canvas.addEventListener("contextmenu", (event) => event.preventDefault());
}

function initializeDebugPanelToggle() {
  if (!store.debugPanelToggle || !store.debugPanelBody || !store.debugPanel) {
    return;
  }

  const applyState = () => {
    const collapsed = store.debugPanel?.dataset?.collapsed === "true";
    if (collapsed) {
      store.debugPanelBody.setAttribute("hidden", "");
    } else {
      store.debugPanelBody.removeAttribute("hidden");
    }
    store.debugPanelToggle.textContent = collapsed ? "Show panel" : "Hide panel";
    store.debugPanelToggle.setAttribute("aria-expanded", String(!collapsed));
  };

  store.debugPanelToggle.addEventListener("click", () => {
    const collapsed = store.debugPanel?.dataset?.collapsed === "true";
    store.debugPanel.dataset.collapsed = collapsed ? "false" : "true";
    applyState();
  });

  applyState();
}

// renderStatus updates the status line with any latency text.
function renderStatus() {
  if (store.statusEl) {
    const baseText = store.statusBaseText || "";
    const camera = store.camera;
    let cameraSuffix = "";
    if (camera) {
      cameraSuffix = camera.lockOnPlayer
        ? "Camera locked — press C to unlock."
        : "Camera unlocked — press C to lock.";
    }
    const separator = baseText && cameraSuffix ? " " : "";
    store.statusEl.textContent = `${baseText}${separator}${cameraSuffix}`.trim();
  }
  if (store.latencyDisplay) {
    const hasLatency =
      typeof store.latencyMs === "number" && Number.isFinite(store.latencyMs);
    store.latencyDisplay.textContent = formatRttLabel(
      hasLatency ? store.latencyMs : null,
    );
    store.latencyDisplay.dataset.state = hasLatency ? "active" : "idle";
  }
}

// setStatusBase records the base status string before latency decorations.
function setStatusBase(text) {
  store.statusBaseText = text;
  renderStatus();
  updateDiagnostics();
}

function ensureCamera() {
  if (!store.camera) {
    store.camera = { x: 0, y: 0, lockOnPlayer: true };
  }
  return store.camera;
}

function setCameraLock(lockOnPlayer) {
  const camera = ensureCamera();
  camera.lockOnPlayer = !!lockOnPlayer;
  if (camera.lockOnPlayer) {
    const viewportWidth = store.canvas?.width || store.WORLD_WIDTH || 0;
    const viewportHeight = store.canvas?.height || store.WORLD_HEIGHT || 0;
    const worldWidth =
      typeof store.WORLD_WIDTH === "number"
        ? store.WORLD_WIDTH
        : viewportWidth;
    const worldHeight =
      typeof store.WORLD_HEIGHT === "number"
        ? store.WORLD_HEIGHT
        : viewportHeight;
    const target =
      store.displayPlayers[store.playerId] || store.players[store.playerId];
    if (target) {
      camera.x = target.x - viewportWidth / 2;
      camera.y = target.y - viewportHeight / 2;
    } else {
      camera.x = worldWidth / 2 - viewportWidth / 2;
      camera.y = worldHeight / 2 - viewportHeight / 2;
    }
  }
  renderStatus();
  updateDiagnostics();
}

function toggleCameraLock() {
  const camera = ensureCamera();
  setCameraLock(!camera.lockOnPlayer);
}

// setLatency stores the latest measured round-trip time.
function setLatency(value) {
  store.latencyMs = value;
  renderStatus();
  updateDiagnostics();
}

// formatAgo renders a human-friendly time delta for diagnostics labels.
function formatAgo(timestamp) {
  if (!timestamp) return "—";
  const delta = Math.max(0, Date.now() - timestamp);
  if (delta < 1000) {
    return `${delta} ms ago`;
  }
  if (delta < 60_000) {
    return `${(delta / 1000).toFixed(1)} s ago`;
  }
  const minutes = Math.floor(delta / 60_000);
  return `${minutes} min ago`;
}

// formatLatency formats a latency value with units for display.
function formatLatency(value) {
  if (value == null) return "—";
  return `${Math.round(value)} ms`;
}

function formatTickLabel(value) {
  if (typeof value === "number" && Number.isFinite(value) && value >= 0) {
    return `Tick: ${Math.floor(value)}`;
  }
  return "Tick: —";
}

function formatRttLabel(value) {
  if (typeof value === "number" && Number.isFinite(value) && value >= 0) {
    return `RTT: ${Math.round(value)} ms`;
  }
  return "RTT: —";
}

// updateDiagnostics refreshes the diagnostics sidebar with live values.
function updateDiagnostics() {
  const els = store.diagnosticsEls;
  if (!els.connection) {
    return;
  }
  const socketStates = ["connecting", "open", "closing", "closed"];
  const connectionText = store.socket
    ? socketStates[store.socket.readyState] || "unknown"
    : "disconnected";
  els.connection.textContent = connectionText;
  const playerCount = Object.keys(store.players).length;
  const npcCount = Object.keys(store.npcs || {}).length;
  const npcLabel = `${npcCount} NPC${npcCount === 1 ? "" : "s"}`;
  els.players.textContent = `${playerCount} players · ${npcLabel}`;
  els.stateAge.textContent = formatAgo(store.lastStateReceivedAt);
  const tickLabel = formatTickLabel(store.lastTick);
  if (els.tick) {
    els.tick.textContent = tickLabel;
  }
  if (store.hudNetworkEls?.tick) {
    store.hudNetworkEls.tick.textContent = tickLabel;
  }
  let intentLabel;
  if (store.isPathActive && store.activePathTarget) {
    const target = store.activePathTarget;
    intentLabel = `path → (${Math.round(target.x)}, ${Math.round(target.y)})`;
  } else if (store.currentIntent.dx === 0 && store.currentIntent.dy === 0) {
    intentLabel = "idle";
  } else {
    intentLabel = `dx:${store.currentIntent.dx.toFixed(2)} dy:${store.currentIntent.dy.toFixed(2)}`;
  }
  els.intent.textContent = intentLabel;
  const lastIntentTs =
    typeof store.lastIntentSentAt === "number" ? store.lastIntentSentAt : 0;
  const lastPathTs =
    typeof store.lastPathRequestAt === "number" ? store.lastPathRequestAt : 0;
  const lastMovementAt = Math.max(lastIntentTs, lastPathTs);
  els.intentAge.textContent = formatAgo(lastMovementAt || null);

  const heartbeatStatus = store.heartbeatTimer !== null ? "active" : "idle";
  const heartbeatParts = [heartbeatStatus];
  if (store.lastHeartbeatSentAt) {
    heartbeatParts.push(`sent ${formatAgo(store.lastHeartbeatSentAt)}`);
  }
  if (store.lastHeartbeatAckAt) {
    heartbeatParts.push(`ack ${formatAgo(store.lastHeartbeatAckAt)}`);
  }
  if (store.lastHeartbeatRoundTrip != null) {
    heartbeatParts.push(`rtt ${formatLatency(store.lastHeartbeatRoundTrip)}`);
  }
  els.heartbeat.textContent = heartbeatParts.join(" · ");
  const latestRtt =
    typeof store.lastHeartbeatRoundTrip === "number" &&
    Number.isFinite(store.lastHeartbeatRoundTrip)
      ? store.lastHeartbeatRoundTrip
      : typeof store.latencyMs === "number" && Number.isFinite(store.latencyMs)
        ? store.latencyMs
        : null;
  const rttLabel = formatRttLabel(latestRtt);
  if (els.latency) {
    els.latency.textContent = rttLabel;
  }
  if (store.hudNetworkEls?.rtt) {
    store.hudNetworkEls.rtt.textContent = rttLabel;
  }
  els.simLatency.textContent = `${store.simulatedLatencyMs} ms`;

  if (store.messagesSent === 0) {
    els.messages.textContent = "none";
  } else {
    const lastSentText = store.lastMessageSentAt
      ? `last ${formatAgo(store.lastMessageSentAt)}`
      : "";
    const base = `${store.messagesSent} (${store.bytesSent} bytes)`;
    els.messages.textContent = lastSentText ? `${base} · ${lastSentText}` : base;
  }

  if (els.patchBaseline || els.patchBatch || els.patchEntities) {
    const patchState =
      store.patchState && typeof store.patchState === "object"
        ? store.patchState
        : null;

    let baselineText = "—";
    let batchText = "—";
    let entitiesText = "—";
    let recoveryText = "—";

    if (patchState) {
      const toFiniteTick = (value) =>
        typeof value === "number" && Number.isFinite(value) && value >= 0
          ? Math.floor(value)
          : null;
      const baselineTick = toFiniteTick(patchState.baseline?.tick);
      const patchedTick = toFiniteTick(patchState.patched?.tick);
      const sourceLabel =
        typeof patchState.lastUpdateSource === "string" &&
        patchState.lastUpdateSource.trim().length > 0
          ? patchState.lastUpdateSource.trim()
          : null;

      let tickSummary = null;
      if (baselineTick !== null && patchedTick !== null) {
        tickSummary =
          baselineTick === patchedTick
            ? `Tick ${baselineTick}`
            : `Tick ${baselineTick} → ${patchedTick}`;
      } else if (baselineTick !== null) {
        tickSummary = `Tick ${baselineTick} (baseline)`;
      } else if (patchedTick !== null) {
        tickSummary = `Tick ${patchedTick} (patched)`;
      }

      if (tickSummary) {
        baselineText = tickSummary;
        if (sourceLabel) {
          baselineText += ` · ${sourceLabel}`;
        }
      } else if (sourceLabel) {
        baselineText = sourceLabel;
      }

      const toNonNegativeInt = (value) =>
        typeof value === "number" && Number.isFinite(value) && value >= 0
          ? Math.floor(value)
          : 0;
      const appliedCount = toNonNegativeInt(patchState.lastAppliedPatchCount);
      const errors = Array.isArray(patchState.errors) ? patchState.errors : [];
      const errorCount = errors.length;
      const lastError =
        patchState.lastError && typeof patchState.lastError === "object"
          ? patchState.lastError
          : null;

      const formatErrorLabel = () => {
        if (errorCount === 0) {
          return "no errors";
        }
        const message =
          typeof lastError?.message === "string" && lastError.message.trim().length > 0
            ? lastError.message.trim()
            : "error";
        const truncated =
          message.length > 72 ? `${message.slice(0, 69)}…` : message;
        const errorTick = toFiniteTick(lastError?.tick);
        const tickSuffix = errorTick !== null ? `@${errorTick}` : "";
        const countLabel = `${errorCount} error${errorCount === 1 ? "" : "s"}`;
        return tickSuffix
          ? `${countLabel} ${tickSuffix} (${truncated})`
          : `${countLabel} (${truncated})`;
      };

      batchText = `Applied ${appliedCount} · ${formatErrorLabel()}`;

      const countEntries = (record) => {
        if (!record || typeof record !== "object") {
          return 0;
        }
        return Object.keys(record).length;
      };
      const baselineCounts = {
        players: countEntries(patchState.baseline?.players),
        npcs: countEntries(patchState.baseline?.npcs),
        effects: countEntries(patchState.baseline?.effects),
        groundItems: countEntries(patchState.baseline?.groundItems),
      };
      const patchedCounts = {
        players: countEntries(patchState.patched?.players),
        npcs: countEntries(patchState.patched?.npcs),
        effects: countEntries(patchState.patched?.effects),
        groundItems: countEntries(patchState.patched?.groundItems),
      };

      const formatCountLabel = (label, baselineValue, patchedValue) => {
        const base = toNonNegativeInt(baselineValue);
        const patched = toNonNegativeInt(patchedValue);
        if (base === patched) {
          return `${label} ${patched}`;
        }
        return `${label} ${base}→${patched}`;
      };

      const entityParts = [
        formatCountLabel("Players", baselineCounts.players, patchedCounts.players),
        formatCountLabel("NPCs", baselineCounts.npcs, patchedCounts.npcs),
        formatCountLabel("Effects", baselineCounts.effects, patchedCounts.effects),
        formatCountLabel("Items", baselineCounts.groundItems, patchedCounts.groundItems),
      ];

      entitiesText = entityParts.filter(Boolean).join(" · ");

      const pendingRequestMap =
        patchState.pendingKeyframeRequests instanceof Map
          ? patchState.pendingKeyframeRequests
          : null;
      const pendingRequestCount = pendingRequestMap ? pendingRequestMap.size : 0;
      const lastRecoveryEntry = Array.isArray(patchState.recoveryLog) && patchState.recoveryLog.length > 0
        ? patchState.recoveryLog[patchState.recoveryLog.length - 1]
        : patchState.lastRecovery && typeof patchState.lastRecovery === "object"
          ? patchState.lastRecovery
          : null;
      const pendingSeqs = pendingRequestMap
        ? Array.from(pendingRequestMap.keys()).filter((value) =>
            typeof value === "number" && Number.isFinite(value),
          )
        : [];
      pendingSeqs.sort((a, b) => a - b);
      const formatRecoveryLabel = () => {
        if (pendingRequestCount > 0) {
          const latestPending = pendingSeqs[pendingSeqs.length - 1] ?? null;
          const seqLabel =
            typeof latestPending === "number" && Number.isFinite(latestPending)
              ? `seq ${Math.floor(latestPending)}`
              : "pending";
          if (
            lastRecoveryEntry &&
            lastRecoveryEntry.status === "requested" &&
            lastRecoveryEntry.sequence === latestPending
          ) {
            const requestedAt =
              typeof lastRecoveryEntry.requestedAt === "number" &&
              Number.isFinite(lastRecoveryEntry.requestedAt)
                ? lastRecoveryEntry.requestedAt
                : null;
            const age = requestedAt ? formatAgo(requestedAt) : null;
            return age ? `request ${seqLabel} (${age})` : `request ${seqLabel}`;
          }
          return pendingRequestCount === 1
            ? `request ${seqLabel}`
            : `${pendingRequestCount} requests`;
        }
        if (!lastRecoveryEntry) {
          return "none";
        }
        const seqLabel =
          typeof lastRecoveryEntry.sequence === "number" && Number.isFinite(lastRecoveryEntry.sequence)
            ? `seq ${Math.floor(lastRecoveryEntry.sequence)}`
            : "seq ?";
        if (lastRecoveryEntry.status === "recovered") {
          const latency =
            typeof lastRecoveryEntry.latencyMs === "number" && Number.isFinite(lastRecoveryEntry.latencyMs)
              ? `${Math.max(0, Math.floor(lastRecoveryEntry.latencyMs))} ms`
              : null;
          const resolvedAt =
            typeof lastRecoveryEntry.resolvedAt === "number" &&
            Number.isFinite(lastRecoveryEntry.resolvedAt)
              ? formatAgo(lastRecoveryEntry.resolvedAt)
              : null;
          const parts = ["recovered", seqLabel];
          if (latency) {
            parts.push(`(${latency})`);
          }
          if (resolvedAt) {
            parts.push(`· ${resolvedAt}`);
          }
          return parts.join(" ");
        }
        if (lastRecoveryEntry.status === "requested") {
          const requestedAt =
            typeof lastRecoveryEntry.requestedAt === "number" &&
            Number.isFinite(lastRecoveryEntry.requestedAt)
              ? formatAgo(lastRecoveryEntry.requestedAt)
              : null;
          return requestedAt ? `request ${seqLabel} (${requestedAt})` : `request ${seqLabel}`;
        }
        if (lastRecoveryEntry.status === "expired") {
          const expiredAt =
            typeof lastRecoveryEntry.resolvedAt === "number" && Number.isFinite(lastRecoveryEntry.resolvedAt)
              ? formatAgo(lastRecoveryEntry.resolvedAt)
              : null;
          return expiredAt ? `expired ${seqLabel} (${expiredAt})` : `expired ${seqLabel}`;
        }
        if (lastRecoveryEntry.status === "rate_limited") {
          const notedAt =
            typeof lastRecoveryEntry.resolvedAt === "number" && Number.isFinite(lastRecoveryEntry.resolvedAt)
              ? formatAgo(lastRecoveryEntry.resolvedAt)
              : null;
          return notedAt ? `rate_limited ${seqLabel} (${notedAt})` : `rate_limited ${seqLabel}`;
        }
        if (typeof lastRecoveryEntry.status === "string" && lastRecoveryEntry.status.length > 0) {
          return `${lastRecoveryEntry.status} ${seqLabel}`;
        }
        return seqLabel;
      };

      const nackCounts =
        patchState.keyframeNackCounts && typeof patchState.keyframeNackCounts === "object"
          ? patchState.keyframeNackCounts
          : {};
      const expiredNacks = toNonNegativeInt(nackCounts.expired);
      const rateLimitedNacks = toNonNegativeInt(nackCounts.rate_limited);
      const nackLabels = [];
      if (expiredNacks > 0) {
        nackLabels.push(`expired ${expiredNacks}`);
      }
      if (rateLimitedNacks > 0) {
        nackLabels.push(`rate ${rateLimitedNacks}`);
      }
      const baseRecoveryLabel = formatRecoveryLabel();
      if (baseRecoveryLabel && nackLabels.length > 0) {
        recoveryText = `${baseRecoveryLabel} · ${nackLabels.join(" · ")}`;
      } else if (baseRecoveryLabel) {
        recoveryText = baseRecoveryLabel;
      } else if (nackLabels.length > 0) {
        recoveryText = nackLabels.join(" · ");
      } else {
        recoveryText = baseRecoveryLabel;
      }
    }

    if (els.patchBaseline) {
      els.patchBaseline.textContent = baselineText;
    }
    if (els.patchBatch) {
      els.patchBatch.textContent = batchText;
    }
    if (els.patchEntities) {
      els.patchEntities.textContent = entitiesText;
    }
    if (els.patchRecovery) {
      els.patchRecovery.textContent = recoveryText;
    }
  }
}

function ensureWorldResetDirtyFields() {
  if (!store.worldResetDirtyFields) {
    store.worldResetDirtyFields = {};
  }
  WORLD_RESET_CONFIG_KEYS.forEach((key) => {
    if (typeof store.worldResetDirtyFields[key] !== "boolean") {
      store.worldResetDirtyFields[key] = false;
    }
  });
}

function getConfigCount(cfg, key) {
  const raw = Number(cfg?.[key]);
  if (Number.isFinite(raw)) {
    return Math.max(0, Math.floor(raw));
  }
  const fallback = Number(DEFAULT_WORLD_CONFIG[key]);
  if (Number.isFinite(fallback)) {
    return Math.max(0, Math.floor(fallback));
  }
  return 0;
}

function updateCountDisabledState(toggleKey) {
  const mapping = WORLD_RESET_COUNT_BY_TOGGLE[toggleKey];
  if (!mapping) {
    return;
  }
  const toggleInput = store.worldResetInputs[toggleKey];
  const enabled = !!toggleInput?.checked;
  const keys = Array.isArray(mapping) ? mapping : [mapping];
  keys.forEach((countKey) => {
    const countInput = store.worldResetInputs[countKey];
    if (countInput) {
      countInput.disabled = !enabled;
    }
  });
}

function syncWorldResetControls() {
  const cfg = store.worldConfig || DEFAULT_WORLD_CONFIG;
  ensureWorldResetDirtyFields();

  WORLD_RESET_TOGGLE_KEYS.forEach((key) => {
    const input = store.worldResetInputs[key];
    if (!input) {
      return;
    }

    const desired = cfg[key] !== false;
    if (input.checked === desired) {
      store.worldResetDirtyFields[key] = false;
    }

    if (!store.worldResetDirtyFields[key]) {
      input.checked = desired;
    }
    updateCountDisabledState(key);
  });

  WORLD_RESET_COUNT_KEYS.forEach((key) => {
    const input = store.worldResetInputs[key];
    if (!input) {
      return;
    }

    const desiredValue = getConfigCount(cfg, key);
    const currentValue = Number.parseInt(input.value, 10);
    if (Number.isFinite(currentValue) && currentValue === desiredValue) {
      store.worldResetDirtyFields[key] = false;
    }

    if (!store.worldResetDirtyFields[key]) {
      input.value = String(desiredValue);
    }
  });

  const seedInput = store.worldResetInputs.seed;
  if (seedInput) {
    const desiredSeed =
      typeof cfg.seed === "string" && cfg.seed.trim().length > 0
        ? cfg.seed
        : DEFAULT_WORLD_SEED;
    if (!store.worldResetDirtyFields.seed) {
      seedInput.value = desiredSeed;
    }
  }
}

function setWorldResetPending(pending) {
  store.isResettingWorld = pending;
  if (!store.worldResetForm) {
    return;
  }
  const elements = store.worldResetForm.querySelectorAll("input, button");
  elements.forEach((element) => {
    if ("disabled" in element) {
      element.disabled = pending;
    }
  });
  if (!pending) {
    WORLD_RESET_TOGGLE_KEYS.forEach((key) => updateCountDisabledState(key));
  }
}

function showWorldResetStatus(message, isError = false) {
  if (!store.worldResetStatusEl) {
    return;
  }
  store.worldResetStatusEl.textContent = message || "";
  store.worldResetStatusEl.dataset.error = isError ? "true" : "false";
}

function initializeWorldResetControls() {
  if (!store.worldResetForm) {
    return;
  }

  ensureWorldResetDirtyFields();

  const registerToggleDirtyTracking = (key) => {
    const input = store.worldResetInputs[key];
    if (!input) {
      return;
    }

    const updateDirtyState = () => {
      const expectedConfig = store.worldConfig || DEFAULT_WORLD_CONFIG;
      const expected = expectedConfig[key] !== false;
      store.worldResetDirtyFields[key] = input.checked !== expected;
      updateCountDisabledState(key);
    };

    input.addEventListener("change", updateDirtyState);
    input.addEventListener("input", updateDirtyState);
    updateCountDisabledState(key);
  };

  WORLD_RESET_TOGGLE_KEYS.forEach(registerToggleDirtyTracking);

  const registerCountDirtyTracking = (key) => {
    const input = store.worldResetInputs[key];
    if (!input) {
      return;
    }

    const updateDirtyState = () => {
      const expected = getConfigCount(store.worldConfig || DEFAULT_WORLD_CONFIG, key);
      const parsed = Number.parseInt(input.value, 10);
      const normalized = Number.isFinite(parsed) ? Math.max(0, parsed) : expected;
      store.worldResetDirtyFields[key] = normalized !== expected;
    };

    input.addEventListener("input", updateDirtyState);
    input.addEventListener("change", updateDirtyState);
  };

  WORLD_RESET_COUNT_KEYS.forEach(registerCountDirtyTracking);

  const seedInput = store.worldResetInputs.seed;
  if (seedInput) {
    const updateSeedDirtyState = () => {
      const expected =
        typeof store.worldConfig?.seed === "string" &&
        store.worldConfig.seed.trim().length > 0
          ? store.worldConfig.seed
          : DEFAULT_WORLD_SEED;
      store.worldResetDirtyFields.seed = seedInput.value !== expected;
    };
    seedInput.addEventListener("input", updateSeedDirtyState);
    seedInput.addEventListener("change", updateSeedDirtyState);
  }

  store.worldResetForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (store.isResettingWorld) {
      return;
    }

    const parseCountValue = (key) => {
      const input = store.worldResetInputs[key];
      if (!input) {
        return getConfigCount(store.worldConfig || DEFAULT_WORLD_CONFIG, key);
      }
      const parsed = Number.parseInt(input.value, 10);
      if (Number.isFinite(parsed)) {
        return Math.max(0, parsed);
      }
      return getConfigCount(store.worldConfig || DEFAULT_WORLD_CONFIG, key);
    };

    const desiredGoblinCount = parseCountValue("goblinCount");
    const desiredRatCount = parseCountValue("ratCount");
    const desiredConfig = {
      obstacles: !!store.worldResetInputs.obstacles?.checked,
      obstaclesCount: parseCountValue("obstaclesCount"),
      goldMines: !!store.worldResetInputs.goldMines?.checked,
      goldMineCount: parseCountValue("goldMineCount"),
      npcs: !!store.worldResetInputs.npcs?.checked,
      goblinCount: desiredGoblinCount,
      ratCount: desiredRatCount,
      npcCount: desiredGoblinCount + desiredRatCount,
      lava: !!store.worldResetInputs.lava?.checked,
      lavaCount: parseCountValue("lavaCount"),
      seed: store.worldResetInputs.seed?.value?.trim() || "",
    };

    setWorldResetPending(true);
    showWorldResetStatus("Restarting world...");
    try {
      await resetWorld(store, desiredConfig);
      WORLD_RESET_CONFIG_KEYS.forEach((key) => {
        store.worldResetDirtyFields[key] = false;
      });
      syncWorldResetControls();
      store.lastWorldResetAt = Date.now();
      const timestamp = new Date(store.lastWorldResetAt).toLocaleTimeString();
      showWorldResetStatus(`World restarted at ${timestamp}.`);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      showWorldResetStatus(`Failed to restart world: ${message}`, true);
    } finally {
      setWorldResetPending(false);
    }
  });

  setWorldResetPending(false);
  syncWorldResetControls();
  showWorldResetStatus("");
}

function formatItemName(type) {
  if (typeof type !== "string" || type.length === 0) {
    return "Unknown";
  }
  const metadata = ITEM_METADATA[type];
  if (metadata && metadata.name) {
    return metadata.name;
  }
  return type
    .split("_")
    .filter(Boolean)
    .map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1))
    .join(" ");
}

function renderInventory() {
  if (!store.inventoryGrid || !store.playerId) {
    return;
  }
  const player = store.players[store.playerId];
  const slots = Array.isArray(player?.inventory?.slots)
    ? player.inventory.slots
    : [];
  const slotMap = new Map();
  let maxSlotIndex = store.inventorySlotCount - 1;
  for (const slot of slots) {
    if (typeof slot?.slot !== "number" || slot.slot < 0) {
      continue;
    }
    slotMap.set(slot.slot, slot.item);
    if (slot.slot > maxSlotIndex) {
      maxSlotIndex = slot.slot;
    }
  }

  const slotCount = Math.max(maxSlotIndex + 1, store.inventorySlotCount);
  store.inventoryGrid.replaceChildren();

  for (let i = 0; i < slotCount; i += 1) {
    const cell = document.createElement("div");
    cell.className = "inventory-slot";
    const item = slotMap.get(i);

    if (item && typeof item.type === "string") {
      const metadata = ITEM_METADATA[item.type] || {};
      const iconEl = document.createElement("div");
      iconEl.className = "inventory-item-icon";
      iconEl.textContent = metadata.icon || "⬜";
      cell.appendChild(iconEl);

      const nameEl = document.createElement("div");
      nameEl.className = "inventory-item-name";
      nameEl.textContent = formatItemName(item.type);
      cell.appendChild(nameEl);

      const quantityEl = document.createElement("div");
      quantityEl.className = "inventory-item-quantity";
      quantityEl.textContent = `x${Math.max(0, Number(item.quantity) || 0)}`;
      cell.appendChild(quantityEl);
    } else {
      const emptyEl = document.createElement("div");
      emptyEl.className = "inventory-empty";
      emptyEl.textContent = "Empty";
      cell.appendChild(emptyEl);
    }

    store.inventoryGrid.appendChild(cell);
  }
}

// setSimulatedLatency updates the artificial latency slider and value.
function setSimulatedLatency(storeRef, value) {
  storeRef.simulatedLatencyMs = Math.max(0, Number.isFinite(value) ? value : 0);
  if (storeRef.latencyInput) {
    storeRef.latencyInput.value = String(storeRef.simulatedLatencyMs);
  }
  updateDiagnostics();
}

// handleSimulatedLatencyInput parses latency overrides typed by the user.
function handleSimulatedLatencyInput() {
  if (!store.latencyInput) {
    return;
  }
  const parsed = Number(store.latencyInput.value);
  if (Number.isFinite(parsed)) {
    setSimulatedLatency(store, Math.max(0, parsed));
  } else if (store.latencyInput.value === "") {
    setSimulatedLatency(store, 0);
  }
}

// attachLatencyInputListener registers the diagnostics latency input handler.
function attachLatencyInputListener() {
  if (!store.latencyInput) {
    return;
  }
  if (store.latencyInputListener) {
    store.latencyInput.removeEventListener("input", store.latencyInputListener);
  }
  store.latencyInputListener = () => handleSimulatedLatencyInput();
  store.latencyInput.addEventListener("input", store.latencyInputListener);
}

store.setStatusBase = setStatusBase;
store.setLatency = setLatency;
store.updateDiagnostics = updateDiagnostics;
store.setSimulatedLatency = (value) => setSimulatedLatency(store, value);
store.renderInventory = renderInventory;
store.updateWorldConfigUI = () => syncWorldResetControls();
store.setCameraLock = setCameraLock;
store.toggleCameraLock = toggleCameraLock;

initializeDebugPanelToggle();
attachLatencyInputListener();
initializeWorldResetControls();
initializeCanvasPathing();
setSimulatedLatency(store, 0);
updateDiagnostics();
renderStatus();
renderInventory();

registerInputHandlers(store);
startRenderLoop(store);
joinGame(store);
