import { LitElement, html } from "lit";
import type { PropertyValues } from "lit";
import "@shoelace-style/shoelace/dist/themes/dark.css";
import "@shoelace-style/shoelace/dist/components/button/button.js";
import "@shoelace-style/shoelace/dist/components/card/card.js";
import "@shoelace-style/shoelace/dist/components/input/input.js";
import "@shoelace-style/shoelace/dist/components/tab-group/tab-group.js";
import "@shoelace-style/shoelace/dist/components/tab/tab.js";
import "@shoelace-style/shoelace/dist/components/tab-panel/tab-panel.js";
import "@shoelace-style/shoelace/dist/components/tag/tag.js";
import { setBasePath } from "@shoelace-style/shoelace/dist/utilities/base-path.js";
import { GameClientOrchestrator, type ClientHeartbeatTelemetry } from "./client-manager";
import {
  InMemoryInputStore,
  KeyboardInputController,
  type CommandKind,
  type CommandRejectionDetails,
  type InputBindings,
  type InputActionDispatcher,
} from "./input";
import { WebSocketNetworkClient, type WorldConfigurationSnapshot } from "./network";
import { CanvasRenderer, type Renderer } from "./render";
import { InMemoryWorldStateStore } from "./world-state";

setBasePath("https://cdn.jsdelivr.net/npm/@shoelace-style/shoelace@2.20.1/dist/");

const HEALTH_CHECK_URL = "/health";
const JOIN_URL = "/join";
const WEBSOCKET_URL = "/ws";
const HEARTBEAT_INTERVAL_MS = 2000;
const PROTOCOL_VERSION = 1;

const RENDERER_CONFIGURATION = {
  dimensions: { width: 800, height: 600 },
  layers: [
    { id: "effect-visual", zIndex: 1 },
    { id: "effect-area", zIndex: 2 },
    { id: "effect-target", zIndex: 3 },
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

type TabGroupElement = HTMLElement & {
  show?: (panel: string) => void;
};

type PathCommandDetail =
  | { readonly kind: "move"; readonly x: number; readonly y: number }
  | { readonly kind: "cancel" };

interface LogEntry {
  timestamp: string;
  message: string;
}

interface CommandRejectionDisplay {
  label: string;
  reason: string;
  meta: string | null;
}

type SessionState = "idle" | "connecting" | "connected" | "shuttingDown" | "error";

class GameClientApp extends LitElement {
  static properties = {
    healthStatus: { state: true },
    serverTime: { state: true },
    heartbeat: { state: true },
    logs: { state: true },
    activeTab: { state: true },
    commandRejection: { state: true },
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
  commandRejection: CommandRejectionDisplay | null;

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
      onPathCommand: (state) => {
        this.inputStore.setPathActive(state.active);
        this.inputStore.setPathTarget?.(state.target);
      },
      onCommandRejectionChanged: (rejection) => {
        if (!rejection) {
          this.inputStore.clearCommandRejection?.();
          this.updateCommandRejectionDisplay(null);
          return;
        }
        if (rejection.retry) {
          this.inputStore.clearCommandRejection?.(rejection.kind);
          this.updateCommandRejectionDisplay(null);
          return;
        }
        this.inputStore.setCommandRejection?.(rejection);
        this.updateCommandRejectionDisplay(rejection);
        const description = this.describeCommandKind(rejection.kind);
        this.addLog(`${description} rejected: ${rejection.reason}`);
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
    this.commandRejection = null;
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

  private describeCommandKind(kind: CommandKind): string {
    switch (kind) {
      case "input":
        return "Movement command";
      case "path":
        return "Path command";
      case "cancelPath":
        return "Path cancel command";
      case "action":
        return "Action command";
      default:
        return `${kind} command`;
    }
  }

  private updateCommandRejectionDisplay(rejection: CommandRejectionDetails | null): void {
    if (!rejection) {
      this.commandRejection = null;
      return;
    }

    const label = `${this.describeCommandKind(rejection.kind)} rejected`;
    const metadata: string[] = [`seq ${rejection.sequence}`];
    if (typeof rejection.tick === "number" && Number.isFinite(rejection.tick)) {
      metadata.push(`tick ${Math.floor(rejection.tick)}`);
    }

    this.commandRejection = {
      label,
      reason: rejection.reason,
      meta: metadata.length > 0 ? metadata.join(" · ") : null,
    };
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
        .commandRejection=${this.commandRejection}
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
    commandRejection: { attribute: false },
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
  commandRejection: CommandRejectionDisplay | null;
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
    this.commandRejection = null;
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
      <main class="app-shell">
        <sl-card class="app-shell__header">
          <div class="app-shell__header-content">
            <div>
              <h1>${this.heading}</h1>
              <p class="app-shell__subtitle">${this.subtitle}</p>
            </div>
            <div class="app-shell__metrics">
              <div class="app-shell__metric">
                <span class="app-shell__label">Health</span>
                <span class="app-shell__value">${this.healthStatus}</span>
              </div>
              <div class="app-shell__metric">
                <span class="app-shell__label">Server time</span>
                <span class="app-shell__value">${this.serverTime}</span>
              </div>
              <div class="app-shell__metric">
                <span class="app-shell__label">Heartbeat</span>
                <span class="app-shell__value">${this.heartbeat}</span>
              </div>
            </div>
          </div>
          <div class="app-shell__actions">
            <sl-button size="small" @click=${this.handleRefreshClick}>Refresh status</sl-button>
            <sl-tag size="small">${this.formatConnectionStatusLabel()}</sl-tag>
            ${this.connectionError
              ? html`<span class="app-shell__error">${this.connectionError}</span>`
              : null}
          </div>
        </sl-card>
        <game-canvas
          .renderer=${this.renderer}
          .activeTab=${this.activeTab}
          .logs=${this.logs}
          .healthStatus=${this.healthStatus}
          .serverTime=${this.serverTime}
          .heartbeat=${this.heartbeat}
          .commandRejection=${this.commandRejection}
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
    commandRejection: { attribute: false },
    worldDimensions: { attribute: false },
  } as const;

  private canvasElement: HTMLCanvasElement | null = null;
  private mountedRenderer: Renderer | null = null;
  private pointerHandlersAttached = false;
  private tabGroup: TabGroupElement | null = null;

  renderer: Renderer | null;
  activeTab!: PanelKey;
  logs!: LogEntry[];
  healthStatus!: string;
  serverTime!: string;
  heartbeat!: string;
  commandRejection: CommandRejectionDisplay | null;
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
    this.commandRejection = null;
    this.worldDimensions = null;
  }

  createRenderRoot(): Element | ShadowRoot {
    return this;
  }

  protected firstUpdated(): void {
    this.canvasElement = this.querySelector("canvas");
    this.registerPointerHandlers();
    this.attachRenderer();
    this.syncTabGroup();
    if (!this.renderer && this.canvasElement) {
      this.drawBootScreen(this.canvasElement);
    }
  }

  protected updated(changed: PropertyValues<GameCanvas>): void {
    if (changed.has("renderer")) {
      this.attachRenderer();
    }
    if (changed.has("activeTab")) {
      this.syncTabGroup();
    }
  }

  disconnectedCallback(): void {
    super.disconnectedCallback();
    this.unregisterPointerHandlers();
    this.detachRenderer();
    this.tabGroup = null;
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

  private syncTabGroup(): void {
    const group = (this.tabGroup ?? (this.querySelector("sl-tab-group") as TabGroupElement | null)) ?? null;
    if (!group) {
      this.tabGroup = null;
      return;
    }
    this.tabGroup = group;
    const activePanel = (group as unknown as { activeTab?: { panel?: string | null } | null }).activeTab?.panel ?? null;
    if (activePanel === this.activeTab) {
      return;
    }
    group.show?.(this.activeTab);
  }

  private handleTabShow(event: CustomEvent<{ name: string }>): void {
    const name = (event.detail?.name ?? "") as PanelKey;
    if (!name || this.activeTab === name) {
      return;
    }
    this.dispatchEvent(
      new CustomEvent<PanelKey>("tab-change", {
        detail: name,
        bubbles: true,
        composed: true,
      }),
    );
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
      <section class="game-canvas">
        <sl-card class="canvas-card">
          <canvas width="800" height="600" aria-label="Game viewport"></canvas>
        </sl-card>
        <sl-tab-group
          class="game-tabs"
          placement="top"
          @sl-tab-show=${this.handleTabShow}
        >
          <sl-tab slot="nav" panel="telemetry">Telemetry</sl-tab>
          <sl-tab slot="nav" panel="world">World</sl-tab>
          <sl-tab slot="nav" panel="inventory">Inventory</sl-tab>
          <sl-tab-panel name="telemetry">
            <debug-panel
              .healthStatus=${this.healthStatus}
              .serverTime=${this.serverTime}
              .heartbeat=${this.heartbeat}
              .logs=${this.logs}
              .commandRejection=${this.commandRejection}
            ></debug-panel>
          </sl-tab-panel>
          <sl-tab-panel name="world">
            <world-controls></world-controls>
          </sl-tab-panel>
          <sl-tab-panel name="inventory">
            <inventory-panel></inventory-panel>
          </sl-tab-panel>
        </sl-tab-group>
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
    commandRejection: { attribute: false },
  } as const;

  healthStatus!: string;
  serverTime!: string;
  heartbeat!: string;
  logs!: LogEntry[];
  commandRejection: CommandRejectionDisplay | null;

  constructor() {
    super();
    this.healthStatus = "--";
    this.serverTime = "--";
    this.heartbeat = "--";
    this.logs = [];
    this.commandRejection = null;
  }

  createRenderRoot(): Element | ShadowRoot {
    return this;
  }

  render() {
    const logText = this.logs
      .map((log) => `[${log.timestamp}] ${log.message}`)
      .join("\n");

    return html`
      <sl-card class="panel-card">
        <div class="panel-card__header">
          <h2>Telemetry</h2>
          <p class="panel-card__description">Live diagnostics from the connected client instance.</p>
        </div>
        <div class="panel-card__grid">
          <div class="panel-card__item">
            <span class="panel-card__label">Client health</span>
            <span class="panel-card__value">${this.healthStatus}</span>
          </div>
          <div class="panel-card__item">
            <span class="panel-card__label">Server time</span>
            <span class="panel-card__value">${this.serverTime}</span>
          </div>
          <div class="panel-card__item">
            <span class="panel-card__label">Connection</span>
            <span class="panel-card__value">${this.heartbeat}</span>
          </div>
        </div>
        ${this.commandRejection
          ? html`
              <div class="panel-card__alert" role="status" aria-live="polite">
                <span class="panel-card__label">${this.commandRejection.label}</span>
                <span class="panel-card__value">${this.commandRejection.reason}</span>
                ${this.commandRejection.meta
                  ? html`<span class="panel-card__meta">${this.commandRejection.meta}</span>`
                  : null}
              </div>
            `
          : null}
        <pre class="panel-card__log">${logText || "Booting client…"}</pre>
      </sl-card>
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
      <sl-card class="panel-card">
        <h2>World controls</h2>
        <form class="world-controls__form" @submit=${this.handleSubmit}>
          <sl-input
            name="seed"
            label="World seed"
            placeholder="Leave empty for random seed"
          ></sl-input>
          <sl-button type="submit">Reset world</sl-button>
        </form>
      </sl-card>
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
      <sl-card class="panel-card">
        <h2>Inventory</h2>
        <div class="inventory-grid">
          ${this.items.map((item) => {
            return html`
              <div class="inventory-slot" role="listitem">
                <span class="inventory-item-icon" aria-hidden="true">${item.icon}</span>
                <div class="inventory-item-details">
                  <span class="inventory-item-name">${item.name}</span>
                  <span class="inventory-item-quantity">x${item.quantity}</span>
                </div>
              </div>
            `;
          })}
        </div>
      </sl-card>
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
      <sl-card class="hud-network">
        <div class="hud-network__grid">
          <span>Status: ${this.formatConnectionStatusLabel()}</span>
          <span>Heartbeat: ${this.heartbeat}</span>
          <span>Server time: ${this.serverTime}</span>
          <span>Player: ${this.playerId ? this.playerId : "—"}</span>
          ${this.connectionError
            ? html`<span class="hud-network__error">${this.connectionError}</span>`
            : null}
        </div>
      </sl-card>
    `;
  }
}

customElements.define("game-client-app", GameClientApp);
customElements.define("app-shell", AppShell);
customElements.define("game-canvas", GameCanvas);
customElements.define("debug-panel", DebugPanel);
customElements.define("world-controls", WorldControls);
customElements.define("inventory-panel", InventoryPanel);
customElements.define("hud-network", HudNetwork);
