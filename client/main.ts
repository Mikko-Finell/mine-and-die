import { LitElement, html } from "lit";
import type { PropertyValues } from "lit";
import { classMap } from "lit/directives/class-map.js";
import { WebSocketNetworkClient } from "./network";
import {
  GameClientOrchestrator,
  type ClientManagerConfiguration,
} from "./client-manager";
import {
  CanvasRenderer,
  type Renderer,
  type RendererConfiguration,
} from "./render";
import { InMemoryWorldStateStore } from "./world-state";

const HEALTH_CHECK_URL = "/health";
const JOIN_URL = "/join";
const WEBSOCKET_URL = "/ws";
const HEARTBEAT_INTERVAL_MS = 2000;
const PROTOCOL_VERSION = 1;

const ORCHESTRATOR_CONFIGURATION: ClientManagerConfiguration = {
  autoConnect: true,
  reconcileIntervalMs: 0,
  keyframeRetryDelayMs: 1000,
};

const RENDERER_CONFIGURATION: RendererConfiguration = {
  dimensions: {
    width: 960,
    height: 540,
  },
  layers: [
    { id: "effect-area", zIndex: 1 },
    { id: "effect-target", zIndex: 2 },
    { id: "effect-visual", zIndex: 3 },
  ],
};

type PanelKey = "telemetry" | "world" | "inventory";

interface LogEntry {
  timestamp: string;
  message: string;
}

class GameClientApp extends LitElement {
  static properties = {
    healthStatus: { state: true },
    serverTime: { state: true },
    connectionStatus: { state: true },
    connectionError: { state: true },
    logs: { state: true },
    activeTab: { state: true },
    playerId: { state: true },
  } as const;

  private clockInterval: number | undefined;
  private readonly networkClient: WebSocketNetworkClient;
  private readonly worldState: InMemoryWorldStateStore;
  private readonly renderer: CanvasRenderer;
  private readonly orchestrator: GameClientOrchestrator;
  private isBootingSession: boolean;
  private isShuttingDownSession: boolean;
  private hasActiveSession: boolean;

  playerId: string | null;
  connectionStatus: string;
  connectionError: string | null;

  constructor() {
    super();
    this.clockInterval = undefined;
    this.networkClient = new WebSocketNetworkClient({
      joinUrl: JOIN_URL,
      websocketUrl: WEBSOCKET_URL,
      heartbeatIntervalMs: HEARTBEAT_INTERVAL_MS,
      protocolVersion: PROTOCOL_VERSION,
    });
    this.worldState = new InMemoryWorldStateStore();
    this.renderer = new CanvasRenderer(RENDERER_CONFIGURATION);
    this.orchestrator = new GameClientOrchestrator(ORCHESTRATOR_CONFIGURATION, {
      network: this.networkClient,
      renderer: this.renderer,
      worldState: this.worldState,
    });
    this.isBootingSession = false;
    this.isShuttingDownSession = false;
    this.hasActiveSession = false;
    this.healthStatus = "Checking…";
    this.serverTime = "--";
    this.connectionStatus = "Idle";
    this.connectionError = null;
    this.logs = [] as LogEntry[];
    this.activeTab = "telemetry";
    this.playerId = null;
    this.addLog("Booting client…");
  }

  createRenderRoot(): Element | ShadowRoot {
    return this;
  }

  connectedCallback(): void {
    super.connectedCallback();
    this.updateServerTime();
    void this.fetchHealth();
    this.startSession();
    this.clockInterval = window.setInterval(() => {
      this.updateServerTime();
    }, 1000);
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    if (this.clockInterval) {
      window.clearInterval(this.clockInterval);
      this.clockInterval = undefined;
    }
    void this.shutdownSession();
  }

  private addLog(message: string): void {
    const entry: LogEntry = {
      timestamp: new Date().toLocaleTimeString(),
      message,
    };
    this.logs = [entry, ...this.logs].slice(0, 50);
  }

  private async fetchHealth(): Promise<void> {
    this.healthStatus = "Checking…";
    try {
      const response = await fetch(HEALTH_CHECK_URL, { cache: "no-cache" });
      if (!response.ok) {
        throw new Error(`health check failed with ${response.status}`);
      }
      const text = (await response.text()).trim();
      const status = text || "ok";
      this.healthStatus = status;
      this.addLog(`Health check succeeded: ${status}`);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      this.healthStatus = "offline";
      this.addLog(`Health check failed: ${message}`);
    }
  }

  private updateServerTime(): void {
    const now = new Date();
    this.serverTime = now.toLocaleTimeString();
  }

  private handleRefreshRequested(): void {
    void this.fetchHealth();
  }

  private startSession(): void {
    if (this.isBootingSession || this.isShuttingDownSession || this.hasActiveSession) {
      return;
    }
    this.isBootingSession = true;
    this.hasActiveSession = false;
    this.connectionStatus = "Connecting…";
    this.connectionError = null;
    this.playerId = null;
    this.addLog("Connecting to world…");
    const bootPromise = this.orchestrator.boot({
      onReady: () => {
        this.isBootingSession = false;
        this.connectionStatus = "Connected";
        this.connectionError = null;
        this.hasActiveSession = true;
        const player = this.orchestrator.playerId ?? null;
        this.playerId = player;
        if (player) {
          this.addLog(`Connected to world as ${player}.`);
        } else {
          this.addLog("Connected to world.");
        }
      },
      onError: (error) => {
        this.isBootingSession = false;
        this.handleOrchestratorError(error);
      },
    });
    void bootPromise.finally(() => {
      this.isBootingSession = false;
    });
  }

  private async shutdownSession(): Promise<void> {
    if (this.isShuttingDownSession) {
      return;
    }
    this.isShuttingDownSession = true;
    let encounteredError = false;
    try {
      await this.orchestrator.shutdown();
    } catch (error) {
      encounteredError = true;
      this.handleOrchestratorError(error);
    } finally {
      const wasActive = this.hasActiveSession;
      if (!encounteredError) {
        this.connectionStatus = "Disconnected";
        this.connectionError = null;
      }
      this.playerId = null;
      this.isBootingSession = false;
      this.isShuttingDownSession = false;
      this.hasActiveSession = false;
      if (wasActive && !encounteredError) {
        this.addLog("Disconnected from world.");
      }
    }
  }

  private handleOrchestratorError(cause: unknown): void {
    const error = cause instanceof Error ? cause : new Error(String(cause));
    const message = error.message || "Unknown error";
    this.connectionStatus = "Error";
    this.connectionError = message;
    this.playerId = this.orchestrator.playerId ?? null;
    this.hasActiveSession = false;
    this.addLog(`Client error: ${message}`);
  }

  private handleTabChange(event: CustomEvent<PanelKey>): void {
    this.activeTab = event.detail;
  }

  private handleWorldReset(event: CustomEvent<{ seed: string }>): void {
    const { seed } = event.detail;
    const seedMessage = seed ? `with seed ${seed}` : "with random seed";
    this.addLog(`World reset requested ${seedMessage}.`);
  }

  render() {
    return html`
      <app-shell
        heading="Mine &amp; Die"
        subtitle="Multiplayer sandbox in active development."
        .healthStatus=${this.healthStatus}
        .logs=${this.logs}
        .serverTime=${this.serverTime}
        .connectionStatus=${this.connectionStatus}
        .connectionError=${this.connectionError ?? ""}
        .renderer=${this.renderer}
        .activeTab=${this.activeTab}
        @refresh-requested=${this.handleRefreshRequested}
        @tab-change=${this.handleTabChange}
        @world-reset-requested=${this.handleWorldReset}
      ></app-shell>
      <hud-network
        .serverTime=${this.serverTime}
        .connectionStatus=${this.connectionStatus}
        .connectionError=${this.connectionError ?? ""}
        .playerId=${this.playerId ?? ""}
      ></hud-network>
    `;
  }
}

class AppShell extends LitElement {
  static properties = {
    heading: { type: String },
    subtitle: { type: String },
    healthStatus: { type: String },
    logs: { attribute: false },
    serverTime: { type: String },
    connectionStatus: { type: String },
    connectionError: { type: String },
    renderer: { attribute: false },
    activeTab: { attribute: false },
  } as const;

  heading!: string;
  subtitle!: string;
  healthStatus!: string;
  logs!: LogEntry[];
  serverTime!: string;
  connectionStatus!: string;
  connectionError!: string;
  renderer!: Renderer | null;
  activeTab!: PanelKey;

  constructor() {
    super();
    this.heading = "";
    this.subtitle = "";
    this.healthStatus = "--";
    this.logs = [];
    this.serverTime = "--";
    this.connectionStatus = "Idle";
    this.connectionError = "";
    this.renderer = null;
    this.activeTab = "telemetry";
  }

  createRenderRoot(): Element | ShadowRoot {
    return this;
  }

  private handleRefreshClick(): void {
    this.dispatchEvent(
      new CustomEvent("refresh-requested", {
        bubbles: true,
        composed: true,
      }),
    );
  }

  render() {
    return html`
      <main class="page">
        <header class="page-header">
          <div>
            <h1>${this.heading}</h1>
            <p class="page-header__subtitle">${this.subtitle}</p>
          </div>
          <div class="page-header__controls">
            <button
              type="button"
              class="interface-tabs__tab interface-tabs__tab--active"
              @click=${this.handleRefreshClick}
            >
              Refresh status
            </button>
            <span class="hud-network__item">${this.healthStatus}</span>
          </div>
        </header>
        <game-canvas
          .activeTab=${this.activeTab}
          .logs=${this.logs}
          .healthStatus=${this.healthStatus}
          .serverTime=${this.serverTime}
          .connectionStatus=${this.connectionStatus}
          .connectionError=${this.connectionError}
          .renderer=${this.renderer}
        ></game-canvas>
      </main>
    `;
  }
}

class GameCanvas extends LitElement {
  static properties = {
    activeTab: { attribute: false },
    logs: { attribute: false },
    healthStatus: { type: String },
    serverTime: { type: String },
    connectionStatus: { type: String },
    connectionError: { type: String },
    renderer: { attribute: false },
  } as const;

  private canvasElement: HTMLCanvasElement | null = null;
  private mountedRenderer: Renderer | null = null;

  activeTab!: PanelKey;
  logs!: LogEntry[];
  healthStatus!: string;
  serverTime!: string;
  connectionStatus!: string;
  connectionError!: string;
  renderer!: Renderer | null;

  constructor() {
    super();
    this.activeTab = "telemetry";
    this.logs = [];
    this.healthStatus = "--";
    this.serverTime = "--";
    this.connectionStatus = "Idle";
    this.connectionError = "";
    this.renderer = null;
  }

  createRenderRoot(): Element | ShadowRoot {
    return this;
  }

  protected firstUpdated(): void {
    this.canvasElement = this.querySelector("canvas");
    this.attachRenderer();
    if (!this.renderer && this.canvasElement) {
      this.drawBootScreen(this.canvasElement);
    }
  }

  protected updated(changedProperties: PropertyValues<this>): void {
    super.updated(changedProperties);
    if (changedProperties.has("renderer")) {
      this.attachRenderer();
    }
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    if (this.mountedRenderer) {
      this.mountedRenderer.unmount();
      this.mountedRenderer = null;
    }
  }

  private attachRenderer(): void {
    if (!this.canvasElement) {
      return;
    }
    if (!this.renderer) {
      if (this.mountedRenderer) {
        this.mountedRenderer.unmount();
        this.mountedRenderer = null;
      }
      this.drawBootScreen(this.canvasElement);
      return;
    }

    if (this.mountedRenderer && this.mountedRenderer !== this.renderer) {
      this.mountedRenderer.unmount();
      this.mountedRenderer = null;
    }

    const context = this.canvasElement.getContext("2d");
    if (!context) {
      return;
    }

    const { width, height } = this.renderer.configuration.dimensions;
    if (Number.isFinite(width) && Number.isFinite(height)) {
      this.canvasElement.width = Math.max(0, Math.floor(width));
      this.canvasElement.height = Math.max(0, Math.floor(height));
    }

    this.renderer.mount({ canvas: this.canvasElement, context });
    this.mountedRenderer = this.renderer;
  }

  private drawBootScreen(canvas: HTMLCanvasElement): void {
    const context = canvas.getContext("2d");
    if (!context) {
      return;
    }
    const { width, height } = canvas;
    const gradient = context.createLinearGradient(0, 0, width, height);
    gradient.addColorStop(0, "#0f172a");
    gradient.addColorStop(1, "#1e293b");
    context.fillStyle = gradient;
    context.fillRect(0, 0, width, height);

    context.fillStyle = "#38bdf8";
    context.font = "24px 'Segoe UI', sans-serif";
    context.textAlign = "center";
    context.fillText("Mine & Die", width / 2, height / 2);
  }

  render() {
    const baseDimensions = this.renderer?.configuration.dimensions ?? RENDERER_CONFIGURATION.dimensions;
    const width = Math.max(0, Math.floor(baseDimensions.width));
    const height = Math.max(0, Math.floor(baseDimensions.height));
    const statusText = this.connectionStatus && this.connectionStatus.trim().length > 0 ? this.connectionStatus : "Idle";
    const errorText = this.connectionError?.trim() ?? "";

    return html`
      <section class="play-area">
        <div class="play-area__main">
          <div class="play-area__canvas">
            <canvas width=${width} height=${height} aria-label="Game viewport"></canvas>
          </div>
          <p class="play-area__status" role="status" aria-live="polite">${statusText}</p>
          ${errorText
            ? html`<p class="play-area__error" role="alert" aria-live="assertive">${errorText}</p>`
            : null}
        </div>
        <tabs-nav .activeTab=${this.activeTab}></tabs-nav>
        <panel-viewport
          .activeTab=${this.activeTab}
          .logs=${this.logs}
          .healthStatus=${this.healthStatus}
          .serverTime=${this.serverTime}
          .connectionStatus=${this.connectionStatus}
          .connectionError=${this.connectionError}
        ></panel-viewport>
      </section>
    `;
  }
}

class TabsNav extends LitElement {
  static properties = {
    activeTab: { attribute: false },
  } as const;

  activeTab!: PanelKey;

  constructor() {
    super();
    this.activeTab = "telemetry";
  }

  createRenderRoot(): Element | ShadowRoot {
    return this;
  }

  private selectTab(tab: PanelKey): void {
    if (this.activeTab === tab) {
      return;
    }
    this.dispatchEvent(
      new CustomEvent<PanelKey>("tab-change", {
        detail: tab,
        bubbles: true,
        composed: true,
      }),
    );
  }

  render() {
    const tabs: Array<{ id: PanelKey; label: string }> = [
      { id: "telemetry", label: "Telemetry" },
      { id: "world", label: "World" },
      { id: "inventory", label: "Inventory" },
    ];

    return html`
      <nav class="tabs-nav" aria-label="Client panels">
        <div class="interface-tabs__list">
          ${tabs.map((tab) => {
            const classes = classMap({
              "interface-tabs__tab": true,
              "interface-tabs__tab--active": this.activeTab === tab.id,
            });
            return html`
              <button
                type="button"
                class=${classes}
                @click=${() => {
                  this.selectTab(tab.id);
                }}
              >
                ${tab.label}
              </button>
            `;
          })}
        </div>
      </nav>
    `;
  }
}

class PanelViewport extends LitElement {
  static properties = {
    activeTab: { attribute: false },
    logs: { attribute: false },
    healthStatus: { type: String },
    serverTime: { type: String },
    connectionStatus: { type: String },
    connectionError: { type: String },
  } as const;

  activeTab!: PanelKey;
  logs!: LogEntry[];
  healthStatus!: string;
  serverTime!: string;
  connectionStatus!: string;
  connectionError!: string;

  constructor() {
    super();
    this.activeTab = "telemetry";
    this.logs = [];
    this.healthStatus = "--";
    this.serverTime = "--";
    this.connectionStatus = "Idle";
    this.connectionError = "";
  }

  createRenderRoot(): Element | ShadowRoot {
    return this;
  }

  render() {
    return html`
      <section class="panel-viewport">
        <debug-panel
          .healthStatus=${this.healthStatus}
          .serverTime=${this.serverTime}
          .connectionStatus=${this.connectionStatus}
          .connectionError=${this.connectionError}
          .logs=${this.logs}
          ?hidden=${this.activeTab !== "telemetry"}
        ></debug-panel>
        <world-controls ?hidden=${this.activeTab !== "world"}></world-controls>
        <inventory-panel ?hidden=${this.activeTab !== "inventory"}></inventory-panel>
      </section>
    `;
  }
}

class DebugPanel extends LitElement {
  static properties = {
    healthStatus: { type: String },
    serverTime: { type: String },
    connectionStatus: { type: String },
    connectionError: { type: String },
    logs: { attribute: false },
  } as const;

  healthStatus!: string;
  serverTime!: string;
  connectionStatus!: string;
  connectionError!: string;
  logs!: LogEntry[];

  constructor() {
    super();
    this.healthStatus = "--";
    this.serverTime = "--";
    this.connectionStatus = "Idle";
    this.connectionError = "";
    this.logs = [];
  }

  createRenderRoot(): Element | ShadowRoot {
    return this;
  }

  render() {
    const logText = this.logs
      .map((log) => `[${log.timestamp}] ${log.message}`)
      .join("\n");

    return html`
      <article class="debug-panel">
        <header class="debug-panel__header">
          <div class="debug-panel__heading">
            <h2 class="debug-panel__title">Telemetry</h2>
            <p class="debug-panel__subtitle">
              Live diagnostics from the connected client instance.
            </p>
          </div>
          <button class="debug-panel__toggle" type="button">Collapse</button>
        </header>
        <div class="debug-panel__body">
          <div class="debug-panel__summary">
            <div class="debug-panel__status">
              <span class="debug-panel__status-label">Client health</span>
              <span class="debug-panel__status-text">${this.healthStatus}</span>
            </div>
            <div class="debug-panel__metrics">
              <div class="debug-metric">
                <span class="debug-metric__label">Connection</span>
                <span class="debug-metric__value">${this.connectionStatus}</span>
              </div>
              <div class="debug-metric">
                <span class="debug-metric__label">Last error</span>
                <span class="debug-metric__value">${this.connectionError || "—"}</span>
              </div>
              <div class="debug-metric">
                <span class="debug-metric__label">Server time</span>
                <span class="debug-metric__value">${this.serverTime}</span>
              </div>
            </div>
          </div>
          <section>
            <h3 class="sr-only">Client console output</h3>
            <pre class="console-output">${logText || "Booting client…"}</pre>
          </section>
        </div>
      </article>
    `;
  }
}

class WorldControls extends LitElement {
  createRenderRoot(): Element | ShadowRoot {
    return this;
  }

  private handleSubmit(event: Event): void {
    event.preventDefault();
    const form = event.currentTarget as HTMLFormElement | null;
    if (!form) {
      return;
    }
    const formData = new FormData(form);
    const seed = (formData.get("seed") ?? "").toString().trim();
    this.dispatchEvent(
      new CustomEvent("world-reset-requested", {
        detail: { seed },
        bubbles: true,
        composed: true,
      }),
    );
    form.reset();
  }

  render() {
    return html`
      <section class="world-controls">
        <h2 class="world-controls__title">World controls</h2>
        <form class="world-controls__form" @submit=${this.handleSubmit}>
          <label class="world-controls__label">
            World seed
            <input
              type="text"
              name="seed"
              placeholder="Leave empty for random seed"
              class="world-controls__input"
            />
          </label>
          <button type="submit" class="world-controls__submit">Reset world</button>
        </form>
      </section>
    `;
  }
}

interface InventoryItem {
  id: number;
  name: string;
  icon: string;
  quantity: number;
}

class InventoryPanel extends LitElement {
  static properties = {
    items: { attribute: false },
  } as const;

  items!: InventoryItem[];

  constructor() {
    super();
    this.items = [
      { id: 1, name: "Stone", icon: "🪨", quantity: 64 },
      { id: 2, name: "Wood", icon: "🪵", quantity: 24 },
      { id: 3, name: "Crystal", icon: "💎", quantity: 3 },
      { id: 4, name: "Fiber", icon: "🧵", quantity: 12 },
      { id: 5, name: "Essence", icon: "✨", quantity: 1 },
    ];
  }

  createRenderRoot(): Element | ShadowRoot {
    return this;
  }

  render() {
    return html`
      <section class="inventory-panel">
        <h2>Inventory</h2>
        <div class="inventory-grid">
          ${this.items.map((item) => {
            return html`
              <div class="inventory-slot" role="listitem">
                <span class="inventory-item-icon" aria-hidden="true">${item.icon}</span>
                <span class="inventory-item-name">${item.name}</span>
                <span class="inventory-item-quantity">x${item.quantity}</span>
              </div>
            `;
          })}
        </div>
      </section>
    `;
  }
}

class HudNetwork extends LitElement {
  static properties = {
    serverTime: { type: String },
    connectionStatus: { type: String },
    connectionError: { type: String },
    playerId: { type: String },
  } as const;

  serverTime!: string;
  connectionStatus!: string;
  connectionError!: string;
  playerId!: string;

  constructor() {
    super();
    this.serverTime = "--";
    this.connectionStatus = "Idle";
    this.connectionError = "";
    this.playerId = "";
  }

  createRenderRoot(): Element | ShadowRoot {
    return this;
  }

  render() {
    const errorText = this.connectionError?.trim() ?? "";
    return html`
      <div class="hud-network">
        <span class="hud-network__item">Server time: ${this.serverTime}</span>
        <span class="hud-network__item">Status: ${this.connectionStatus}</span>
        <span class="hud-network__item">
          Player: ${this.playerId ? this.playerId : "—"}
        </span>
        ${errorText
          ? html`<span class="hud-network__item hud-network__item--error">Error: ${errorText}</span>`
          : null}
      </div>
    `;
  }
}

customElements.define("game-client-app", GameClientApp);
customElements.define("app-shell", AppShell);
customElements.define("game-canvas", GameCanvas);
customElements.define("tabs-nav", TabsNav);
customElements.define("panel-viewport", PanelViewport);
customElements.define("debug-panel", DebugPanel);
customElements.define("world-controls", WorldControls);
customElements.define("inventory-panel", InventoryPanel);
customElements.define("hud-network", HudNetwork);
