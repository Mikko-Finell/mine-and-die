import { LitElement, html } from "lit";
import { classMap } from "lit/directives/class-map.js";

const HEALTH_CHECK_URL = "/health";
const JOIN_URL = "/join";

type PanelKey = "telemetry" | "world" | "inventory";

interface LogEntry {
  timestamp: string;
  message: string;
}

class GameClientApp extends LitElement {
  static properties = {
    healthStatus: { state: true },
    serverTime: { state: true },
    heartbeat: { state: true },
    logs: { state: true },
    activeTab: { state: true },
    playerId: { state: true },
  } as const;

  private clockInterval: number | undefined;

  playerId: string | null;

  constructor() {
    super();
    this.healthStatus = "Checking…";
    this.serverTime = "--";
    this.heartbeat = "--";
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
    void this.joinWorld();
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
    this.heartbeat = `${Math.round(Math.random() * 1000)} ms`;
  }

  private handleRefreshRequested(): void {
    void this.fetchHealth();
  }

  private async joinWorld(): Promise<void> {
    this.addLog("Joining world…");
    try {
      const response = await fetch(JOIN_URL, {
        method: "POST",
        cache: "no-cache",
      });
      if (!response.ok) {
        throw new Error(`join failed with ${response.status}`);
      }
      const data = (await response.json()) as { id: string };
      this.playerId = data.id;
      this.addLog(`Joined world as ${data.id}.`);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      this.playerId = null;
      this.addLog(`Failed to join world: ${message}`);
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

  render() {
    return html`
      <app-shell
        heading="Mine &amp; Die"
        subtitle="Multiplayer sandbox in active development."
        .healthStatus=${this.healthStatus}
        .logs=${this.logs}
        .serverTime=${this.serverTime}
        .heartbeat=${this.heartbeat}
        .activeTab=${this.activeTab}
        @refresh-requested=${this.handleRefreshRequested}
        @tab-change=${this.handleTabChange}
        @world-reset-requested=${this.handleWorldReset}
      ></app-shell>
      <hud-network
        .serverTime=${this.serverTime}
        .heartbeat=${this.heartbeat}
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
    heartbeat: { type: String },
    activeTab: { attribute: false },
  } as const;

  heading!: string;
  subtitle!: string;
  healthStatus!: string;
  logs!: LogEntry[];
  serverTime!: string;
  heartbeat!: string;
  activeTab!: PanelKey;

  constructor() {
    super();
    this.heading = "";
    this.subtitle = "";
    this.healthStatus = "--";
    this.logs = [];
    this.serverTime = "--";
    this.heartbeat = "--";
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
          .heartbeat=${this.heartbeat}
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
    heartbeat: { type: String },
  } as const;

  private canvasElement: HTMLCanvasElement | null = null;

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

  protected firstUpdated(): void {
    this.canvasElement = this.querySelector("canvas");
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
                <span class="debug-metric__label">Heartbeat RTT</span>
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
  } as const;

  serverTime!: string;
  heartbeat!: string;
  playerId!: string;

  constructor() {
    super();
    this.serverTime = "--";
    this.heartbeat = "--";
    this.playerId = "";
  }

  createRenderRoot(): Element | ShadowRoot {
    return this;
  }

  render() {
    return html`
      <div class="hud-network">
        <span class="hud-network__item">Server time: ${this.serverTime}</span>
        <span class="hud-network__item">Heartbeat: ${this.heartbeat}</span>
        <span class="hud-network__item">
          Player: ${this.playerId ? this.playerId : "—"}
        </span>
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
