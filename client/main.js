import { joinGame, resetWorld, sendMoveTo } from "./network.js";
import { startRenderLoop } from "./render.js";
import { registerInputHandlers } from "./input.js";

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
const worldResetForm = document.getElementById("world-reset-form");
const worldResetStatus = document.getElementById("world-reset-status");
const worldResetObstacles = document.getElementById("world-reset-obstacles");
const worldResetNPCs = document.getElementById("world-reset-npcs");
const worldResetLava = document.getElementById("world-reset-lava");

const diagnosticsEls = {
  connection: document.getElementById("diag-connection"),
  players: document.getElementById("diag-players"),
  stateAge: document.getElementById("diag-state-age"),
  intent: document.getElementById("diag-intent"),
  intentAge: document.getElementById("diag-intent-age"),
  heartbeat: document.getElementById("diag-heartbeat"),
  latency: document.getElementById("diag-latency"),
  simLatency: document.getElementById("diag-sim-latency"),
  messages: document.getElementById("diag-messages"),
};

const DEFAULT_INVENTORY_SLOTS = 12;

const ITEM_METADATA = {
  gold: { name: "Gold Coin", icon: "🪙" },
  health_potion: { name: "Lesser Healing Potion", icon: "🧪" },
};

const WORLD_RESET_KEYS = ["obstacles", "npcs", "lava"];

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
  inventoryPanel,
  inventoryGrid,
  worldResetForm,
  worldResetStatusEl: worldResetStatus,
  worldResetInputs: {
    obstacles: worldResetObstacles,
    npcs: worldResetNPCs,
    lava: worldResetLava,
  },
  worldResetDirtyFields: { obstacles: false, npcs: false, lava: false },
  TILE_SIZE: 40,
  GRID_WIDTH: canvas.width / 40,
  GRID_HEIGHT: canvas.height / 40,
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
  socket: null,
  reconnectTimeout: null,
  isJoining: false,
  currentIntent: { dx: 0, dy: 0 },
  currentFacing: "down",
  isPathActive: false,
  activePathTarget: null,
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
  effects: [],
  inventorySlotCount: DEFAULT_INVENTORY_SLOTS,
  worldConfig: { obstacles: true, npcs: true, lava: true },
  isResettingWorld: false,
  updateWorldConfigUI: null,
  lastWorldResetAt: null,
};

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
    const x = clamp(
      (event.clientX - rect.left) * scaleX,
      store.PLAYER_HALF,
      store.canvas.width - store.PLAYER_HALF,
    );
    const y = clamp(
      (event.clientY - rect.top) * scaleY,
      store.PLAYER_HALF,
      store.canvas.height - store.PLAYER_HALF,
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
    store.statusEl.textContent = store.statusBaseText || "";
  }
  if (store.latencyDisplay) {
    if (store.latencyMs != null) {
      store.latencyDisplay.textContent = `${Math.round(store.latencyMs)} ms`;
      store.latencyDisplay.dataset.state = "active";
    } else {
      store.latencyDisplay.textContent = "—";
      store.latencyDisplay.dataset.state = "idle";
    }
  }
}

// setStatusBase records the base status string before latency decorations.
function setStatusBase(text) {
  store.statusBaseText = text;
  renderStatus();
  updateDiagnostics();
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

  els.latency.textContent = formatLatency(store.latencyMs);
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
}

function syncWorldResetControls() {
  const cfg = store.worldConfig || {};
  if (!store.worldResetDirtyFields) {
    store.worldResetDirtyFields = { obstacles: false, npcs: false, lava: false };
  }

  WORLD_RESET_KEYS.forEach((key) => {
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
  });
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

  if (!store.worldResetDirtyFields) {
    store.worldResetDirtyFields = { obstacles: false, npcs: false, lava: false };
  }

  const registerDirtyTracking = (key) => {
    const input = store.worldResetInputs[key];
    if (!input) {
      return;
    }

    const updateDirtyState = () => {
      const expected = store.worldConfig ? store.worldConfig[key] !== false : true;
      store.worldResetDirtyFields[key] = input.checked !== expected;
    };

    input.addEventListener("change", updateDirtyState);
    input.addEventListener("input", updateDirtyState);
  };

  WORLD_RESET_KEYS.forEach(registerDirtyTracking);

  store.worldResetForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (store.isResettingWorld) {
      return;
    }

    const desiredConfig = {
      obstacles: !!store.worldResetInputs.obstacles?.checked,
      npcs: !!store.worldResetInputs.npcs?.checked,
      lava: !!store.worldResetInputs.lava?.checked,
    };

    setWorldResetPending(true);
    showWorldResetStatus("Restarting world...");
    try {
      await resetWorld(store, desiredConfig);
      WORLD_RESET_KEYS.forEach((key) => {
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
