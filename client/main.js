import { LitElement, html, nothing } from "./vendor/lit.js";
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
import { startRenderLoop } from "./render.js";
import {
  RENDER_MODE_PATCH,
  RENDER_MODE_SNAPSHOT,
  normalizeRenderMode,
  isRenderMode,
  getValidRenderModes,
} from "./render-modes.js";
import { registerInputHandlers } from "./input.js";
import { createVendorBanner } from "./vendor/example-banner.js";

class LitProbeElement extends LitElement {
  createRenderRoot() {
    return this;
  }

  render() {
    return html`<span class="debug-metric__value" data-lit-status
      >Lit ready</span
    >`;
  }
}

if (!customElements.get("lit-probe")) {
  customElements.define("lit-probe", LitProbeElement);
}

const renderDebugPanelContent = () => html`
  <header class="debug-panel__header">
    <div class="debug-panel__heading">
      <h2 class="debug-panel__title">Testing &amp; Debug</h2>
      <p class="debug-panel__subtitle">
        Inspect the simulation, tweak latency, and manage the world state while you test.
      </p>
    </div>
    <button
      id="debug-panel-toggle"
      type="button"
      class="debug-panel__toggle"
      aria-controls="debug-panel-body"
      aria-expanded="true"
    >
      Hide panel
    </button>
  </header>
  <div id="debug-panel-body" class="debug-panel__body">
    <div class="debug-panel__summary">
      <div class="debug-panel__status">
        <span class="debug-panel__status-label">Session status</span>
        <span id="status" class="debug-panel__status-text" aria-live="polite"
          >Preparing session…</span
        >
      </div>
      <div class="debug-panel__metrics">
        <div class="debug-metric">
          <span class="debug-metric__label">Observed latency</span>
          <span id="latency-display" class="debug-metric__value" aria-live="polite">—</span>
        </div>
        <div class="debug-metric">
          <span class="debug-metric__label">UI framework</span>
          <lit-probe aria-live="polite"></lit-probe>
        </div>
        <label class="debug-metric debug-metric--input" for="latency-input">
          <span class="debug-metric__label">Simulated latency</span>
          <div class="debug-metric__input">
            <input
              type="number"
              id="latency-input"
              min="0"
              step="10"
              value="0"
              inputmode="numeric"
              aria-describedby="latency-input-hint"
            />
            <span class="debug-metric__suffix">ms</span>
          </div>
          <span id="latency-input-hint" class="debug-metric__hint"
            >Inject artificial delay to mimic slower connections.</span
          >
        </label>
      </div>
    </div>
    <div class="debug-panel__sections">
      <details class="debug-section" open>
        <summary class="debug-section__summary">Live diagnostics</summary>
        <div class="debug-section__content">
          <div id="diagnostics" class="diagnostics-grid">
            <div class="diagnostic-stat">
              <span class="diagnostic-stat__label">Connection</span>
              <span id="diag-connection" class="diagnostic-stat__value">—</span>
            </div>
            <div class="diagnostic-stat">
              <span class="diagnostic-stat__label">Players tracked</span>
              <span id="diag-players" class="diagnostic-stat__value">—</span>
            </div>
            <div class="diagnostic-stat">
              <span class="diagnostic-stat__label">Last state update</span>
              <span id="diag-state-age" class="diagnostic-stat__value">—</span>
            </div>
            <div class="diagnostic-stat">
              <span class="diagnostic-stat__label">Server tick</span>
              <span id="diag-tick" class="diagnostic-stat__value">Tick: —</span>
            </div>
            <div class="diagnostic-stat">
              <span class="diagnostic-stat__label">Input vector</span>
              <span id="diag-intent" class="diagnostic-stat__value">—</span>
            </div>
            <div class="diagnostic-stat">
              <span class="diagnostic-stat__label">Last input send</span>
              <span id="diag-intent-age" class="diagnostic-stat__value">—</span>
            </div>
            <div class="diagnostic-stat">
              <span class="diagnostic-stat__label">Heartbeat</span>
              <span id="diag-heartbeat" class="diagnostic-stat__value">—</span>
            </div>
            <div class="diagnostic-stat">
              <span class="diagnostic-stat__label">Latency (observed)</span>
              <span id="diag-latency" class="diagnostic-stat__value">RTT: —</span>
            </div>
            <div class="diagnostic-stat">
              <span class="diagnostic-stat__label">Simulated latency</span>
              <span id="diag-sim-latency" class="diagnostic-stat__value">0 ms</span>
            </div>
            <div class="diagnostic-stat">
              <span class="diagnostic-stat__label">Messages sent</span>
              <span id="diag-messages" class="diagnostic-stat__value">none</span>
            </div>
            <div class="diagnostic-stat">
              <span class="diagnostic-stat__label">Patch baseline</span>
              <span id="diag-patch-baseline" class="diagnostic-stat__value">—</span>
            </div>
            <div class="diagnostic-stat">
              <span class="diagnostic-stat__label">Patch batch</span>
              <span id="diag-patch-batch" class="diagnostic-stat__value">—</span>
            </div>
            <div class="diagnostic-stat">
              <span class="diagnostic-stat__label">Patched entities</span>
              <span id="diag-patch-entities" class="diagnostic-stat__value">—</span>
            </div>
            <div class="diagnostic-stat">
              <span class="diagnostic-stat__label">Patch recovery</span>
              <span id="diag-patch-recovery" class="diagnostic-stat__value">—</span>
            </div>
          </div>
        </div>
      </details>
      <details class="debug-section">
        <summary class="debug-section__summary">Console commands</summary>
        <div class="debug-section__content">
          <div class="debug-console">
            <button
              type="button"
              class="debug-console__button"
              @click=${() => window.debugDropGold(100)}
            >
              Drop 100 gold
            </button>
            <button
              type="button"
              class="debug-console__button"
              @click=${() => window.debugPickupGold()}
            >
              Pickup gold
            </button>
            <button
              type="button"
              class="debug-console__button"
              @click=${() => window.debugNetworkStats()}
            >
              Network stats → console
            </button>
          </div>
        </div>
      </details>
      <details class="debug-section">
        <summary class="debug-section__summary">Rendering mode</summary>
        <div class="debug-section__content">
          <div class="debug-render">
            <button
              type="button"
              class="debug-render__button"
              @click=${() => window.debugSetRenderMode("snapshot")}
            >
              Force full snapshots
            </button>
            <button
              type="button"
              class="debug-render__button"
              @click=${() => window.debugSetRenderMode("patch")}
            >
              Force incremental patches
            </button>
            <button
              type="button"
              class="debug-render__button"
              @click=${() => window.debugToggleRenderMode()}
            >
              Toggle mode
            </button>
          </div>
        </div>
      </details>
    </div>
  </div>
`;

const renderWorldControlsContent = () => html`
  <h2>World controls</h2>
  <form id="world-reset-form" class="world-reset-form">
    <div class="world-reset-grid">
      <div class="world-reset-toggle">
        <label class="world-reset-toggle__item">
          <input
            id="world-reset-obstacles"
            name="obstacles"
            type="checkbox"
            checked
            class="world-reset-toggle__input"
          />
          <span class="world-reset-toggle__label">Obstacles</span>
          <span class="world-reset-toggle__hint"
            >Enable procedural rock walls.</span
          >
        </label>
        <label class="world-reset-toggle__item">
          <input
            id="world-reset-npcs"
            name="npcs"
            type="checkbox"
            checked
            class="world-reset-toggle__input"
          />
          <span class="world-reset-toggle__label">NPCs</span>
          <span class="world-reset-toggle__hint"
            >Spawn goblins and rats to fight.</span
          >
        </label>
        <label class="world-reset-toggle__item">
          <input
            id="world-reset-lava"
            name="lava"
            type="checkbox"
            checked
            class="world-reset-toggle__input"
          />
          <span class="world-reset-toggle__label">Lava geysers</span>
          <span class="world-reset-toggle__hint"
            >Place environmental hazards around the map.</span
          >
        </label>
        <label class="world-reset-toggle__item">
          <input
            id="world-reset-gold-mines"
            name="goldMines"
            type="checkbox"
            checked
            class="world-reset-toggle__input"
          />
          <span class="world-reset-toggle__label">Gold mines</span>
          <span class="world-reset-toggle__hint"
            >Seed ore veins and interactable deposits.</span
          >
        </label>
      </div>
      <div class="world-reset-quantities">
        <label class="world-reset-quantity">
          <span class="world-reset-quantity__label">Obstacles</span>
          <input
            id="world-reset-obstacles-count"
            name="obstaclesCount"
            type="number"
            inputmode="numeric"
            min="0"
            step="1"
            value="2"
            class="world-reset-quantity__input"
            aria-label="Obstacle count"
          />
          <span class="world-reset-quantity__hint"
            >Choose how many rock walls to generate.</span
          >
        </label>
        <label class="world-reset-quantity">
          <span class="world-reset-quantity__label">Goblins</span>
          <input
            id="world-reset-goblins-count"
            name="goblinCount"
            type="number"
            inputmode="numeric"
            min="0"
            step="1"
            value="2"
            class="world-reset-quantity__input"
            aria-label="Goblin count"
          />
          <span class="world-reset-quantity__hint"
            >Choose how many goblins spawn.</span
          >
        </label>
        <label class="world-reset-quantity">
          <span class="world-reset-quantity__label">Rats</span>
          <input
            id="world-reset-rats-count"
            name="ratCount"
            type="number"
            inputmode="numeric"
            min="0"
            step="1"
            value="1"
            class="world-reset-quantity__input"
            aria-label="Rat count"
          />
          <span class="world-reset-quantity__hint"
            >Choose how many rats spawn.</span
          >
        </label>
        <label class="world-reset-quantity">
          <span class="world-reset-quantity__label">Lava geysers</span>
          <input
            id="world-reset-lava-count"
            name="lavaCount"
            type="number"
            inputmode="numeric"
            min="0"
            step="1"
            value="3"
            class="world-reset-quantity__input"
            aria-label="Lava geyser count"
          />
          <span class="world-reset-quantity__hint"
            >Choose how many geysers erupt.</span
          >
        </label>
        <label class="world-reset-quantity">
          <span class="world-reset-quantity__label">Gold mines</span>
          <input
            id="world-reset-gold-mines-count"
            name="goldMineCount"
            type="number"
            inputmode="numeric"
            min="0"
            step="1"
            value="1"
            class="world-reset-quantity__input"
            aria-label="Gold mine count"
          />
          <span class="world-reset-quantity__hint"
            >Choose how many ore veins are generated.</span
          >
        </label>
      </div>
    </div>
  </form>
`;

const renderInventoryContent = () => html`
  <h2>Inventory</h2>
  <div id="inventory-grid" class="inventory-grid" role="list"></div>
`;

const INTERFACE_TAB_STORAGE_KEY = "mine-and-die.interface-panel";

const INTERFACE_PANELS = [
  {
    id: "debug-panel",
    tabId: "interface-tab-telemetry",
    label: "Telemetry",
    panelClass: "debug-panel",
    ariaLabel: "Testing and debug tools",
    dataCollapsed: "false",
    renderContent: renderDebugPanelContent,
  },
  {
    id: "world-controls-panel",
    tabId: "interface-tab-world",
    label: "World Controls",
    panelClass: "",
    ariaLabel: null,
    dataCollapsed: null,
    renderContent: renderWorldControlsContent,
  },
  {
    id: "inventory-panel",
    tabId: "interface-tab-inventory",
    label: "Inventory",
    panelClass: "inventory-panel",
    ariaLabel: null,
    dataCollapsed: null,
    renderContent: renderInventoryContent,
  },
];

const PANEL_IDS = new Set(INTERFACE_PANELS.map((panel) => panel.id));
const DEFAULT_PANEL_ID = "debug-panel";

class GameClientApp extends LitElement {
  static properties = {
    activePanelId: { type: String },
  };

  constructor() {
    super();
    this.activePanelId = DEFAULT_PANEL_ID;
    this.#pendingFocusId = null;
  }

  createRenderRoot() {
    return this;
  }

  connectedCallback() {
    super.connectedCallback();
    const stored = this.#readStoredPanelId();
    if (stored && PANEL_IDS.has(stored)) {
      this.setActivePanel(stored, { notify: false });
    } else if (!this.activePanelId || !PANEL_IDS.has(this.activePanelId)) {
      this.setActivePanel(DEFAULT_PANEL_ID, { notify: false });
    }
  }

  updated() {
    if (!this.#pendingFocusId) {
      return;
    }
    const focusTarget = this.querySelector(
      `[data-panel-id="${this.#pendingFocusId}"]`,
    );
    this.#pendingFocusId = null;
    if (focusTarget && typeof focusTarget.focus === "function") {
      focusTarget.focus();
    }
  }

  setActivePanel(panelId, options = {}) {
    if (!PANEL_IDS.has(panelId)) {
      return;
    }
    const { focus = false, notify = true } = options;
    const changed = this.activePanelId !== panelId;
    this.activePanelId = panelId;
    if (focus) {
      this.#pendingFocusId = panelId;
      if (!changed) {
        this.requestUpdate();
      }
    }
    this.#persistPanelId(panelId);
    if (notify && (changed || focus)) {
      this.dispatchEvent(
        new CustomEvent("interface-panel-change", {
          detail: { panelId },
          bubbles: true,
          composed: true,
        }),
      );
    }
  }

  handleTabKeydown(event, index) {
    const key = event.key;
    if (
      key !== "ArrowRight" &&
      key !== "ArrowDown" &&
      key !== "ArrowLeft" &&
      key !== "ArrowUp" &&
      key !== "Home" &&
      key !== "End"
    ) {
      return;
    }
    event.preventDefault();
    const panelCount = INTERFACE_PANELS.length;
    let nextIndex = index;
    if (key === "ArrowRight" || key === "ArrowDown") {
      nextIndex = (index + 1) % panelCount;
    } else if (key === "ArrowLeft" || key === "ArrowUp") {
      nextIndex = (index - 1 + panelCount) % panelCount;
    } else if (key === "Home") {
      nextIndex = 0;
    } else if (key === "End") {
      nextIndex = panelCount - 1;
    }
    const nextPanel = INTERFACE_PANELS[nextIndex];
    if (nextPanel) {
      this.setActivePanel(nextPanel.id, { focus: true });
    }
  }

  render() {
    const activeId = PANEL_IDS.has(this.activePanelId)
      ? this.activePanelId
      : DEFAULT_PANEL_ID;
    return html`
      <section class="interface-tabs" aria-label="Interface panels">
        <div class="interface-tabs__list" role="tablist">
          ${INTERFACE_PANELS.map((panel, index) => {
            const isActive = panel.id === activeId;
            const tabClasses = ["interface-tabs__tab"];
            if (isActive) {
              tabClasses.push("interface-tabs__tab--active");
            }
            return html`
              <button
                id=${panel.tabId}
                class=${tabClasses.join(" ")}
                type="button"
                role="tab"
                aria-selected=${String(isActive)}
                aria-controls=${panel.id}
                tabindex=${isActive ? "0" : "-1"}
                data-panel-id=${panel.id}
                @click=${() => this.setActivePanel(panel.id)}
                @keydown=${(event) => this.handleTabKeydown(event, index)}
              >
                ${panel.label}
              </button>
            `;
          })}
        </div>
        ${INTERFACE_PANELS.map((panel) => {
          const isActive = panel.id === activeId;
          const panelClasses = ["interface-tabs__panel"];
          if (panel.panelClass) {
            panelClasses.push(panel.panelClass);
          }
          if (isActive) {
            panelClasses.push("interface-tabs__panel--active");
          }
          const ariaLabel = panel.ariaLabel ?? nothing;
          const dataCollapsed = panel.dataCollapsed ?? nothing;
          return html`
            <section
              id=${panel.id}
              class=${panelClasses.join(" ")}
              role="tabpanel"
              tabindex=${isActive ? "0" : "-1"}
              aria-labelledby=${panel.tabId}
              aria-label=${ariaLabel}
              data-collapsed=${dataCollapsed}
              ?hidden=${!isActive}
            >
              ${panel.renderContent()}
            </section>
          `;
        })}
      </section>
    `;
  }

  #readStoredPanelId() {
    try {
      if (typeof window !== "undefined" && window.localStorage) {
        return window.localStorage.getItem(INTERFACE_TAB_STORAGE_KEY);
      }
    } catch (error) {
      return null;
    }
    return null;
  }

  #persistPanelId(panelId) {
    try {
      if (typeof window !== "undefined" && window.localStorage) {
        window.localStorage.setItem(INTERFACE_TAB_STORAGE_KEY, panelId);
      }
    } catch (error) {
      /* ignore storage errors */
    }
  }

  #pendingFocusId;
}

if (!customElements.get("game-client-app")) {
  customElements.define("game-client-app", GameClientApp);
}

console.debug(createVendorBanner("example-banner"));

const diagnosticsEls = {
  connection: null,
  players: null,
  stateAge: null,
  tick: null,
  intent: null,
  intentAge: null,
  heartbeat: null,
  latency: null,
  simLatency: null,
  messages: null,
  patchBaseline: null,
  patchBatch: null,
  patchEntities: null,
  patchRecovery: null,
};

const hudNetworkEls = {
  tick: null,
  rtt: null,
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
  statusEl: null,
  canvas: null,
  ctx: null,
  latencyInput: null,
  latencyDisplay: null,
  debugPanel: null,
  debugPanelBody: null,
  debugPanelToggle: null,
  diagnosticsEls,
  hudNetworkEls,
  inventoryPanel: null,
  inventoryGrid: null,
  worldResetForm: null,
  worldResetStatusEl: null,
  worldResetInputs: {
    obstacles: null,
    obstaclesCount: null,
    npcs: null,
    goblinCount: null,
    ratCount: null,
    lava: null,
    lavaCount: null,
    goldMines: null,
    goldMineCount: null,
    seed: null,
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
  GRID_WIDTH: 0,
  GRID_HEIGHT: 0,
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

function applyRenderMode(nextMode) {
  const resolved =
    typeof nextMode === "string" ? normalizeRenderMode(nextMode) : nextMode;
  const finalMode = isRenderMode(resolved)
    ? resolved
    : null;
  if (!finalMode) {
    console.warn(
      `[render] Unknown mode "${nextMode}". Expected ` +
        `${getValidRenderModes().join(" or ")}.`,
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

      const deferredRecent = toNonNegativeInt(patchState.deferredPatchCount);
      const deferredTotal = toNonNegativeInt(patchState.totalDeferredPatchCount);
      const deferredLatency =
        typeof patchState.lastDeferredReplayLatencyMs === "number" &&
        Number.isFinite(patchState.lastDeferredReplayLatencyMs) &&
        patchState.lastDeferredReplayLatencyMs >= 0
          ? Math.floor(patchState.lastDeferredReplayLatencyMs)
          : null;
      const deferredLabels = [];
      if (deferredRecent > 0) {
        deferredLabels.push(`deferred ${deferredRecent}`);
      }
      if (deferredTotal > 0) {
        deferredLabels.push(`total ${deferredTotal}`);
      }
      if (deferredLatency !== null) {
        deferredLabels.push(`replay ${deferredLatency} ms`);
      }
      if (deferredLabels.length > 0) {
        batchText += ` · ${deferredLabels.join(" · ")}`;
      }

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

async function bootstrap() {
  await customElements.whenDefined("game-client-app");
  const appElement = document.querySelector("game-client-app");
  if (appElement?.updateComplete) {
    try {
      await appElement.updateComplete;
    } catch (error) {
      console.warn("Failed to await game-client-app render", error);
    }
  }

  const diagnosticsMapping = [
    ["connection", "diag-connection"],
    ["players", "diag-players"],
    ["stateAge", "diag-state-age"],
    ["tick", "diag-tick"],
    ["intent", "diag-intent"],
    ["intentAge", "diag-intent-age"],
    ["heartbeat", "diag-heartbeat"],
    ["latency", "diag-latency"],
    ["simLatency", "diag-sim-latency"],
    ["messages", "diag-messages"],
    ["patchBaseline", "diag-patch-baseline"],
    ["patchBatch", "diag-patch-batch"],
    ["patchEntities", "diag-patch-entities"],
    ["patchRecovery", "diag-patch-recovery"],
  ];

  diagnosticsMapping.forEach(([key, id]) => {
    store.diagnosticsEls[key] = document.getElementById(id);
  });

  store.statusEl = document.getElementById("status");
  store.latencyDisplay = document.getElementById("latency-display");
  store.latencyInput = document.getElementById("latency-input");
  store.debugPanel = document.getElementById("debug-panel");
  store.debugPanelBody = document.getElementById("debug-panel-body");
  store.debugPanelToggle = document.getElementById("debug-panel-toggle");
  store.inventoryPanel = document.getElementById("inventory-panel");
  store.inventoryGrid = document.getElementById("inventory-grid");

  const canvas = document.getElementById("game-canvas");
  store.canvas = canvas;
  store.ctx = canvas ? canvas.getContext("2d") : null;
  if (canvas) {
    store.GRID_WIDTH = canvas.width / store.TILE_SIZE;
    store.GRID_HEIGHT = canvas.height / store.TILE_SIZE;
  }

  store.hudNetworkEls.tick = document.getElementById("hud-tick");
  store.hudNetworkEls.rtt = document.getElementById("hud-rtt");

  store.worldResetForm = document.getElementById("world-reset-form");
  store.worldResetStatusEl = document.getElementById("world-reset-status");
  const worldInputs = store.worldResetInputs;
  worldInputs.obstacles = document.getElementById("world-reset-obstacles");
  worldInputs.obstaclesCount = document.getElementById(
    "world-reset-obstacles-count",
  );
  worldInputs.npcs = document.getElementById("world-reset-npcs");
  worldInputs.goblinCount = document.getElementById("world-reset-goblins-count");
  worldInputs.ratCount = document.getElementById("world-reset-rats-count");
  worldInputs.lava = document.getElementById("world-reset-lava");
  worldInputs.lavaCount = document.getElementById("world-reset-lava-count");
  worldInputs.goldMines = document.getElementById("world-reset-gold-mines");
  worldInputs.goldMineCount = document.getElementById(
    "world-reset-gold-mines-count",
  );
  worldInputs.seed = document.getElementById("world-reset-seed");

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
}

bootstrap().catch((error) => {
  console.error("Failed to bootstrap game client", error);
});
