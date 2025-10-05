import { joinGame } from "./network.js";
import { startRenderLoop } from "./render.js";
import { registerInputHandlers } from "./input.js";

const statusEl = document.getElementById("status");
const canvas = document.getElementById("game-canvas");
const ctx = canvas.getContext("2d");
const latencyInput = document.getElementById("latency-input");
const diagnosticsToggle = document.getElementById("diagnostics-toggle");
const diagnosticsSection = document.getElementById("diagnostics");
const inventoryPanel = document.getElementById("inventory-panel");
const inventoryGrid = document.getElementById("inventory-grid");

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

const store = {
  statusEl,
  canvas,
  ctx,
  latencyInput,
  diagnosticsToggle,
  diagnosticsSection,
  diagnosticsEls,
  inventoryPanel,
  inventoryGrid,
  TILE_SIZE: 40,
  GRID_WIDTH: canvas.width / 40,
  GRID_HEIGHT: canvas.height / 40,
  PLAYER_SIZE: 28,
  PLAYER_HALF: 28 / 2,
  LERP_RATE: 12,
  statusBaseText: "",
  latencyMs: null,
  simulatedLatencyMs: 0,
  playerId: null,
  players: {},
  displayPlayers: {},
  obstacles: [],
  socket: null,
  reconnectTimeout: null,
  isJoining: false,
  currentIntent: { dx: 0, dy: 0 },
  currentFacing: "down",
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
  effects: [],
  inventorySlotCount: DEFAULT_INVENTORY_SLOTS,
};

// updateDiagnosticsToggle syncs the toggle label with the current panel state.
function updateDiagnosticsToggle() {
  if (!store.diagnosticsToggle || !store.diagnosticsSection) {
    return;
  }
  const isVisible = !store.diagnosticsSection.hasAttribute("hidden");
  store.diagnosticsToggle.textContent = isVisible
    ? "Hide diagnostics"
    : "Show diagnostics";
  store.diagnosticsToggle.setAttribute("aria-expanded", String(isVisible));
}

// setDiagnosticsVisibility shows or hides the diagnostics block.
function setDiagnosticsVisibility(visible) {
  if (!store.diagnosticsSection) {
    return;
  }
  if (visible) {
    store.diagnosticsSection.removeAttribute("hidden");
  } else {
    store.diagnosticsSection.setAttribute("hidden", "");
  }
  updateDiagnosticsToggle();
}

// initializeDiagnosticsToggle wires the button that expands diagnostics.
function initializeDiagnosticsToggle() {
  if (!store.diagnosticsToggle || !store.diagnosticsSection) {
    return;
  }
  store.diagnosticsToggle.addEventListener("click", () => {
    const isVisible = !store.diagnosticsSection.hasAttribute("hidden");
    setDiagnosticsVisibility(!isVisible);
  });
  updateDiagnosticsToggle();
}

// renderStatus updates the status line with any latency text.
function renderStatus() {
  if (!store.statusEl) return;
  if (store.latencyMs != null) {
    store.statusEl.textContent = `${store.statusBaseText} (latency: ${Math.round(
      store.latencyMs
    )} ms)`;
  } else {
    store.statusEl.textContent = store.statusBaseText;
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
  els.players.textContent = String(Object.keys(store.players).length);
  els.stateAge.textContent = formatAgo(store.lastStateReceivedAt);
  const intentLabel =
    store.currentIntent.dx === 0 && store.currentIntent.dy === 0
      ? "idle"
      : `dx:${store.currentIntent.dx.toFixed(2)} dy:${store.currentIntent.dy.toFixed(2)}`;
  els.intent.textContent = intentLabel;
  els.intentAge.textContent = formatAgo(store.lastIntentSentAt);

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
store.setDiagnosticsVisibility = setDiagnosticsVisibility;
store.renderInventory = renderInventory;

initializeDiagnosticsToggle();
attachLatencyInputListener();
setSimulatedLatency(store, 0);
updateDiagnostics();
renderInventory();

registerInputHandlers(store);
startRenderLoop(store);
joinGame(store);
