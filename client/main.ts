import { LitElement, html } from "lit";
import type { PropertyValues } from "lit";
import { classMap } from "lit/directives/class-map.js";
import { GameClientOrchestrator, type ClientHeartbeatTelemetry } from "./client-manager";
import {
  InMemoryInputStore,
  KeyboardInputController,
  type InputBindings,
  type InputActionDispatcher,
} from "./input";
import { WebSocketNetworkClient, type WorldConfigurationSnapshot } from "./network";
import { CanvasRenderer, type Renderer } from "./render";
import { InMemoryWorldStateStore } from "./world-state";

const HEALTH_CHECK_URL = "/health";
const JOIN_URL = "/join";
const WEBSOCKET_URL = "/ws";
const HEARTBEAT_INTERVAL_MS = 2000;
const PROTOCOL_VERSION = 1;

const RENDERER_CONFIGURATION = {
  dimensions: { width: 800, height: 600 },
  layers: [
    { id: "effect-area", zIndex: 1 },
    { id: "effect-target", zIndex: 2 },
    { id: "effect-visual", zIndex: 3 },
  ],
} as const;

const ORCHESTRATOR_CONFIGURATION = {
  autoConnect: true,
  reconcileIntervalMs: 0,
  keyframeRetryDelayMs: 1000,
} as const;

const INPUT_BINDINGS: InputBindings = {
  attackAction: "attack",
  fireballAction: "fireball",
  cameraLockKey: "c",
  movementKeys: {
    w: "up",
    a: "left",
    s: "down",
    d: "right",
  },
} as const;

type PanelKey = "telemetry" | "world" | "inventory";

type PathCommandDetail =
  | { readonly kind: "move"; readonly x: number; readonly y: number }
  | { readonly kind: "cancel" };

interface LogEntry {
  timestamp: string;
  message: string;
}

type SessionState = "idle" | "connecting" | "connected" | "shuttingDown" | "error";

class GameClientApp extends LitElement {
  static properties = {
    healthStatus: { state: true },
    serverTime: { state: true },
    heartbeat: { state: true },
    logs: { state: true },
    activeTab: { state: true },
    playerId: { state: true },
    worldDimensions: { state: true },
  } as const;

  private clockInterval: number | undefined;
  private readonly renderer: CanvasRenderer;
  private readonly worldStateStore: InMemoryWorldStateStore;
  private readonly networkClient: WebSocketNetworkClient;
  private readonly orchestrator: GameClientOrchestrator;
  private readonly inputStore: InMemoryInputStore;
  private readonly inputDispatcher: InputActionDispatcher;
  private readonly inputController: KeyboardInputController;
  private sessionState: SessionState = "idle";
  private connectionStatus: SessionState = "idle";
  private connectionError: string | null = null;
  private lastHeartbeatTelemetry: ClientHeartbeatTelemetry | null = null;
  private heartbeatAcknowledged = false;
  private inputRegistered = false;

  playerId: string | null;
  worldDimensions: WorldConfigurationSnapshot | null;

  constructor() {
    super();
    this.clockInterval = undefined;
    this.renderer = new CanvasRenderer(RENDERER_CONFIGURATION);
    this.worldStateStore = new InMemoryWorldStateStore();
    this.networkClient = new WebSocketNetworkClient({
      joinUrl: JOIN_URL,
      websocketUrl: WEBSOCKET_URL,
      heartbeatIntervalMs: HEARTBEAT_INTERVAL_MS,
      protocolVersion: PROTOCOL_VERSION,
    });
    this.orchestrator = new GameClientOrchestrator(ORCHESTRATOR_CONFIGURATION, {
      network: this.networkClient,
      renderer: this.renderer,
      worldState: this.worldStateStore,
    });
    this.inputStore = new InMemoryInputStore({
      onCameraLockToggle: (locked) => {
        this.addLog(locked ? "Camera lock enabled." : "Camera lock disabled.");
      },
    });
    this.inputDispatcher = this.orchestrator.createInputDispatcher({
      onPathCommand: (active) => {
        this.inputStore.setPathActive(active);
      },
    });
    this.inputController = new KeyboardInputController({
      store: this.inputStore,
      dispatcher: this.inputDispatcher,
      bindings: INPUT_BINDINGS,
    });
    this.inputRegistered = false;
    this.sessionState = "idle";
    this.connectionStatus = "idle";
    this.connectionError = null;
    this.lastHeartbeatTelemetry = null;
    this.heartbeatAcknowledged = false;
    this.healthStatus = "Checking…";
    this.serverTime = "--";
    this.heartbeat = "Disconnected";
    this.logs = [] as LogEntry[];
    this.activeTab = "telemetry";
    this.playerId = null;
    this.worldDimensions = null;
    this.addLog("Booting client…");
    this.updateHeartbeatStatus();
  }

  createRenderRoot(): Element | ShadowRoot {
    return this;
  }

  connectedCallback(): void {
    super.connectedCallback();
    this.updateServerTime();
    void this.fetchHealth();
    this.startSession();
    if (!this.inputRegistered) {
      this.inputController.register();
      this.inputRegistered = true;
    }
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
    if (this.inputRegistered) {
      this.inputController.unregister();
      this.inputRegistered = false;
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
    this.updateHeartbeatStatus();
  }

  private handleRefreshRequested(): void {
    void this.fetchHealth();
  }

  private startSession(): void {
    if (this.sessionState !== "idle" && this.sessionState !== "error") {
      return;
    }

    this.sessionState = "connecting";
    this.connectionStatus = "connecting";
    this.connectionError = null;
    this.lastHeartbeatTelemetry = null;
    this.worldDimensions = null;
    this.heartbeatAcknowledged = false;
    this.playerId = null;
    this.addLog("Joining world…");
    this.updateHeartbeatStatus();

    void this.orchestrator
      .boot({
        onReady: () => {
          this.sessionState = "connected";
          this.connectionStatus = "connected";
          this.connectionError = null;
          const join = this.orchestrator.getJoinResponse();
          if (join) {
            this.playerId = this.orchestrator.playerId;
            this.worldDimensions = join.world;
            if (this.playerId) {
              this.addLog(`Joined world as ${this.playerId}.`);
            }
            const catalogSize = Object.keys(join.effectCatalog).length;
            this.addLog(`Received ${catalogSize} effect catalog entries.`);
          }
          this.addLog("Connected to world stream.");
          this.updateHeartbeatStatus();
        },
        onError: (error: Error) => {
          if (this.sessionState === "shuttingDown") {
            return;
          }
          this.sessionState = "error";
          this.connectionStatus = "error";
          const message = error instanceof Error ? error.message : String(error);
          this.connectionError = message;
          this.playerId = this.orchestrator.playerId;
          this.addLog(`Client error: ${message}`);
          this.lastHeartbeatTelemetry = null;
          this.heartbeatAcknowledged = false;
          this.updateHeartbeatStatus();
        },
        onHeartbeat: (telemetry: ClientHeartbeatTelemetry) => {
          this.handleHeartbeatTelemetry(telemetry);
        },
      })
      .catch((error: unknown) => {
        const message = error instanceof Error ? error.message : String(error);
        this.sessionState = "error";
        this.connectionStatus = "error";
        this.connectionError = message;
        this.playerId = null;
        this.addLog(`Failed to establish session: ${message}`);
        this.lastHeartbeatTelemetry = null;
        this.heartbeatAcknowledged = false;
        this.updateHeartbeatStatus();
      });
  }

  private async shutdownSession(): Promise<void> {
    if (this.sessionState === "shuttingDown" || this.sessionState === "idle") {
      return;
    }

    this.sessionState = "shuttingDown";
    this.connectionStatus = "shuttingDown";

    try {
      await this.orchestrator.shutdown();
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      this.addLog(`Shutdown error: ${message}`);
    } finally {
      this.sessionState = "idle";
      this.connectionStatus = "idle";
      this.connectionError = null;
      this.playerId = null;
      this.lastHeartbeatTelemetry = null;
      this.heartbeatAcknowledged = false;
      this.inputStore.setPathActive(false);
      this.worldDimensions = null;
      this.updateHeartbeatStatus();
      this.addLog("Disconnected from world.");
    }
  }

  private handleTabChange(event: CustomEvent<PanelKey>): void {
    this.activeTab = event.detail;
  }

  private handleWorldReset(event: CustomEvent<{ seed: string }>): void {
    const { seed } = event.detail;
    const seedMessage = seed ? `with seed ${seed}` : "with random seed";
    this.addLog(`World reset requested ${seedMessage}.`);
  }

  private handleHeartbeatTelemetry(telemetry: ClientHeartbeatTelemetry): void {
    this.lastHeartbeatTelemetry = telemetry;
    if (!this.heartbeatAcknowledged) {
      const latency = telemetry.roundTripTimeMs !== null
        ? `${Math.round(telemetry.roundTripTimeMs)} ms`
        : "unknown";
      this.addLog(`Heartbeat acknowledged (RTT ${latency}).`);
      this.heartbeatAcknowledged = true;
    }
    this.updateHeartbeatStatus();
  }

  private handlePathCommand(event: CustomEvent<PathCommandDetail>): void {
    const detail = event.detail;
    if (!detail) {
      return;
    }

    if (detail.kind === "move") {
      this.inputDispatcher.sendPathCommand({ x: detail.x, y: detail.y });
      return;
    }

    if (detail.kind === "cancel") {
      this.inputDispatcher.cancelPath();
    }
  }

  private updateHeartbeatStatus(): void {
    if (this.connectionStatus === "connected") {
      const telemetry = this.lastHeartbeatTelemetry;
      if (!telemetry) {
        this.heartbeat = "Awaiting heartbeat…";
        return;
      }

      const latencyPart = telemetry.roundTripTimeMs !== null
        ? `rtt ${Math.round(telemetry.roundTripTimeMs)} ms`
        : "rtt —";
      const age = Date.now() - telemetry.receivedAt;
      const agePart = `ack ${this.formatRelativeTime(age)} ago`;
      this.heartbeat = `${latencyPart} · ${agePart}`;
      return;
    }

    if (this.connectionStatus === "connecting") {
      this.heartbeat = "Connecting…";
      return;
    }

    if (this.connectionStatus === "shuttingDown") {
      this.heartbeat = "Disconnecting…";
      return;
    }

    if (this.connectionStatus === "error") {
      this.heartbeat = this.connectionError ? `Error: ${this.connectionError}` : "Error";
      return;
    }

    this.heartbeat = "Disconnected";
  }

  private formatRelativeTime(durationMs: number): string {
    const clamped = Math.max(0, durationMs);
    if (clamped < 1000) {
      return `${Math.round(clamped)} ms`;
    }
    const seconds = clamped / 1000;
    if (seconds < 10) {
      return `${seconds.toFixed(1)} s`;
    }
    if (seconds < 60) {
      return `${Math.round(seconds)} s`;
    }
    const minutes = seconds / 60;
    if (minutes < 10) {
      return `${minutes.toFixed(1)} min`;
    }
    return `${Math.round(minutes)} min`;
  }

  render() {
    return html`
      <app-shell
        heading="Mine &amp; Die"
        subtitle="Multiplayer sandbox in active development."
        .renderer=${this.renderer}
        .healthStatus=${this.healthStatus}
        .logs=${this.logs}
        .serverTime=${this.serverTime}
        .heartbeat=${this.heartbeat}
        .activeTab=${this.activeTab}
        .connectionStatus=${this.connectionStatus}
        .connectionError=${this.connectionError ?? ""}
        .worldDimensions=${this.worldDimensions}
        @refresh-requested=${this.handleRefreshRequested}
        @tab-change=${this.handleTabChange}
        @world-reset-requested=${this.handleWorldReset}
        @path-command=${this.handlePathCommand}
      ></app-shell>
      <hud-network
        .serverTime=${this.serverTime}
        .heartbeat=${this.heartbeat}
        .playerId=${this.playerId ?? ""}
        .connectionStatus=${this.connectionStatus}
        .connectionError=${this.connectionError ?? ""}
      ></hud-network>
    `;
  }
}

class AppShell extends LitElement {
  static properties = {
    heading: { type: String },
    subtitle: { type: String },
    renderer: { attribute: false },
    healthStatus: { type: String },
    logs: { attribute: false },
    serverTime: { type: String },
    heartbeat: { type: String },
    activeTab: { attribute: false },
    connectionStatus: { type: String },
    connectionError: { type: String },
    worldDimensions: { attribute: false },
  } as const;

  heading!: string;
  subtitle!: string;
  renderer: Renderer | null;
  healthStatus!: string;
  logs!: LogEntry[];
  serverTime!: string;
  heartbeat!: string;
  activeTab!: PanelKey;
  connectionStatus!: SessionState;
  connectionError!: string;
  worldDimensions: WorldConfigurationSnapshot | null;

  constructor() {
    super();
    this.heading = "";
    this.subtitle = "";
    this.renderer = null;
    this.healthStatus = "--";
    this.logs = [];
    this.serverTime = "--";
    this.heartbeat = "--";
    this.activeTab = "telemetry";
    this.connectionStatus = "idle";
    this.connectionError = "";
    this.worldDimensions = null;
  }

  createRenderRoot(): Element | ShadowRoot {
    return this;
  }

  private formatConnectionStatusLabel(): string {
    switch (this.connectionStatus) {
      case "connecting":
        return "Connecting";
      case "connected":
        return "Connected";
      case "shuttingDown":
        return "Disconnecting";
      case "error":
        return "Error";
      default:
        return "Idle";
    }
  }

  private getConnectionStatusVariant(): string {
    switch (this.connectionStatus) {
      case "connected":
        return "connected";
      case "connecting":
        return "connecting";
      case "shuttingDown":
        return "disconnecting";
      case "error":
        return "error";
      default:
        return "idle";
    }
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
    const statusClasses = classMap({
      "connection-status__pill": true,
      [`connection-status__pill--${this.getConnectionStatusVariant()}`]: true,
    });

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
            <div class="connection-status">
              <span class=${statusClasses}>${this.formatConnectionStatusLabel()}</span>
              ${this.connectionError
                ? html`<span class="connection-status__error">${this.connectionError}</span>`
                : null}
            </div>
          </div>
        </header>
        <game-canvas
          .renderer=${this.renderer}
          .activeTab=${this.activeTab}
          .logs=${this.logs}
          .healthStatus=${this.healthStatus}
          .serverTime=${this.serverTime}
          .heartbeat=${this.heartbeat}
          .worldDimensions=${this.worldDimensions}
        ></game-canvas>
      </main>
    `;
  }
}

class GameCanvas extends LitElement {
  static properties = {
    renderer: { attribute: false },
    activeTab: { attribute: false },
    logs: { attribute: false },
    healthStatus: { type: String },
    serverTime: { type: String },
    heartbeat: { type: String },
    worldDimensions: { attribute: false },
  } as const;

  private canvasElement: HTMLCanvasElement | null = null;
  private mountedRenderer: Renderer | null = null;
  private pointerHandlersAttached = false;

  renderer: Renderer | null;
  activeTab!: PanelKey;
  logs!: LogEntry[];
  healthStatus!: string;
  serverTime!: string;
  heartbeat!: string;
  worldDimensions: WorldConfigurationSnapshot | null;

  private readonly handlePointerDown = (event: PointerEvent): void => {
    if (event.button === 0) {
      if (this.activeTab !== "telemetry") {
        return;
      }
      event.preventDefault();
      const position = this.translatePointerToWorld(event);
      if (!position) {
        return;
      }
      this.dispatchPathCommand({ kind: "move", x: position.x, y: position.y });
      return;
    }

    if (event.button === 2) {
      event.preventDefault();
      this.dispatchPathCommand({ kind: "cancel" });
    }
  };

  private readonly handleContextMenu = (event: MouseEvent): void => {
    event.preventDefault();
  };

  constructor() {
    super();
    this.renderer = null;
    this.activeTab = "telemetry";
    this.logs = [];
    this.healthStatus = "--";
    this.serverTime = "--";
    this.heartbeat = "--";
    this.worldDimensions = null;
  }

  createRenderRoot(): Element | ShadowRoot {
    return this;
  }

  protected firstUpdated(): void {
    this.canvasElement = this.querySelector("canvas");
    this.registerPointerHandlers();
    this.attachRenderer();
    if (!this.renderer && this.canvasElement) {
      this.drawBootScreen(this.canvasElement);
    }
  }

  protected updated(changed: PropertyValues<GameCanvas>): void {
    if (changed.has("renderer")) {
      this.attachRenderer();
    }
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    this.unregisterPointerHandlers();
    this.detachRenderer();
    if (this.canvasElement) {
      this.drawBootScreen(this.canvasElement);
    }
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

  private attachRenderer(): void {
    const canvas = this.canvasElement ?? (this.querySelector("canvas") as HTMLCanvasElement | null);
    if (!canvas) {
      return;
    }
    this.canvasElement = canvas;
    this.registerPointerHandlers();

    if (!this.renderer) {
      this.detachRenderer();
      this.drawBootScreen(canvas);
      return;
    }

    if (this.mountedRenderer === this.renderer) {
      return;
    }

    this.detachRenderer();
    const context = canvas.getContext("2d");
    if (!context) {
      return;
    }

    const dimensions = this.renderer.configuration?.dimensions;
    if (dimensions && Number.isFinite(dimensions.width) && Number.isFinite(dimensions.height)) {
      canvas.width = Math.max(0, Math.floor(dimensions.width));
      canvas.height = Math.max(0, Math.floor(dimensions.height));
    }

    this.renderer.mount({ canvas, context });
    this.mountedRenderer = this.renderer;
  }

  private detachRenderer(): void {
    if (this.mountedRenderer) {
      this.mountedRenderer.unmount();
      this.mountedRenderer = null;
    }
  }

  private registerPointerHandlers(): void {
    const canvas = this.canvasElement;
    if (!canvas || this.pointerHandlersAttached) {
      return;
    }
    canvas.addEventListener("pointerdown", this.handlePointerDown);
    canvas.addEventListener("contextmenu", this.handleContextMenu);
    this.pointerHandlersAttached = true;
  }

  private unregisterPointerHandlers(): void {
    const canvas = this.canvasElement;
    if (!canvas) {
      this.pointerHandlersAttached = false;
      return;
    }
    if (!this.pointerHandlersAttached) {
      return;
    }
    canvas.removeEventListener("pointerdown", this.handlePointerDown);
    canvas.removeEventListener("contextmenu", this.handleContextMenu);
    this.pointerHandlersAttached = false;
  }

  private translatePointerToWorld(event: PointerEvent): { x: number; y: number } | null {
    const canvas = this.canvasElement;
    if (!canvas) {
      return null;
    }
    const rect = canvas.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) {
      return null;
    }
    const scaleX = canvas.width / rect.width;
    const scaleY = canvas.height / rect.height;
    const localX = (event.clientX - rect.left) * scaleX;
    const localY = (event.clientY - rect.top) * scaleY;

    const width = this.worldDimensions?.width ?? canvas.width;
    const height = this.worldDimensions?.height ?? canvas.height;

    return {
      x: this.clamp(localX, 0, width),
      y: this.clamp(localY, 0, height),
    };
  }

  private clamp(value: number, min: number, max: number): number {
    if (value < min) {
      return min;
    }
    if (value > max) {
      return max;
    }
    return value;
  }

  private dispatchPathCommand(detail: PathCommandDetail): void {
    this.dispatchEvent(
      new CustomEvent<PathCommandDetail>("path-command", {
        detail,
        bubbles: true,
        composed: true,
      }),
    );
  }

  render() {
    return html`
      <section class="play-area">
        <div class="play-area__main">
          <canvas width="800" height="600" aria-label="Game viewport"></canvas>
        </div>
        <tabs-nav .activeTab=${this.activeTab}></tabs-nav>
        <panel-viewport
          .activeTab=${this.activeTab}
          .logs=${this.logs}
          .healthStatus=${this.healthStatus}
          .serverTime=${this.serverTime}
          .heartbeat=${this.heartbeat}
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
    heartbeat: { type: String },
  } as const;

  activeTab!: PanelKey;
  logs!: LogEntry[];
  healthStatus!: string;
  serverTime!: string;
  heartbeat!: string;

  constructor() {
    super();
    this.activeTab = "telemetry";
    this.logs = [];
    this.healthStatus = "--";
    this.serverTime = "--";
    this.heartbeat = "--";
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
          .heartbeat=${this.heartbeat}
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
    heartbeat: { type: String },
    logs: { attribute: false },
  } as const;

  healthStatus!: string;
  serverTime!: string;
  heartbeat!: string;
  logs!: LogEntry[];

  constructor() {
    super();
    this.healthStatus = "--";
    this.serverTime = "--";
    this.heartbeat = "--";
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
                <span class="debug-metric__label">Server time</span>
                <span class="debug-metric__value">${this.serverTime}</span>
              </div>
              <div class="debug-metric">
                <span class="debug-metric__label">Connection status</span>
                <span class="debug-metric__value">${this.heartbeat}</span>
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
    heartbeat: { type: String },
    playerId: { type: String },
    connectionStatus: { type: String },
    connectionError: { type: String },
  } as const;

  serverTime!: string;
  heartbeat!: string;
  playerId!: string;
  connectionStatus!: SessionState;
  connectionError!: string;

  constructor() {
    super();
    this.serverTime = "--";
    this.heartbeat = "--";
    this.playerId = "";
    this.connectionStatus = "idle";
    this.connectionError = "";
  }

  createRenderRoot(): Element | ShadowRoot {
    return this;
  }

  private formatConnectionStatusLabel(): string {
    switch (this.connectionStatus) {
      case "connecting":
        return "Connecting";
      case "connected":
        return "Connected";
      case "shuttingDown":
        return "Disconnecting";
      case "error":
        return "Error";
      default:
        return "Idle";
    }
  }

  render() {
    return html`
      <div class="hud-network">
        <span class="hud-network__item hud-network__item--status">
          Status: ${this.formatConnectionStatusLabel()}
        </span>
        <span class="hud-network__item">Heartbeat: ${this.heartbeat}</span>
        <span class="hud-network__item">Server time: ${this.serverTime}</span>
        <span class="hud-network__item">
          Player: ${this.playerId ? this.playerId : "—"}
        </span>
        ${this.connectionError
          ? html`<span class="hud-network__item hud-network__item--error">
              ${this.connectionError}
            </span>`
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
