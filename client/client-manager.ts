import {
  ContractLifecycleStore,
  type ContractLifecycleBatch,
  type ContractLifecycleEntry,
  type ContractLifecycleEndEvent,
  type ContractLifecycleSpawnEvent,
  type ContractLifecycleUpdateEvent,
  type ContractLifecycleView,
} from "./effect-lifecycle-store";
import {
  getEffectCatalogEntry,
  normalizeEffectCatalog,
  setEffectCatalog,
  type EffectCatalogEntryMetadata,
} from "./effect-catalog";
import type {
  JoinResponse,
  NetworkClient,
  NetworkEventHandlers,
  NetworkMessageEnvelope,
} from "./network";
import type { RenderBatch, Renderer, StaticGeometry, AnimationFrame, RenderLayer } from "./render";
import type { WorldPatchBatch, WorldStateStore, WorldKeyframe } from "./world-state";
import type { DeliveryKind } from "./generated/effect-contracts";

export interface ClientManagerConfiguration {
  readonly autoConnect: boolean;
  readonly reconcileIntervalMs: number;
}

export interface ClientLifecycleHandlers {
  readonly onReady?: () => void;
  readonly onError?: (error: Error) => void;
}

export interface ClientOrchestrator {
  readonly configuration: ClientManagerConfiguration;
  readonly boot: (handlers: ClientLifecycleHandlers) => Promise<void>;
  readonly shutdown: () => Promise<void>;
  readonly handleKeyframe: (keyframe: WorldKeyframe) => void;
  readonly handlePatchBatch: (patch: WorldPatchBatch) => void;
  readonly requestRender: (batch: RenderBatch) => void;
}

export class GameClientOrchestrator implements ClientOrchestrator {
  private readonly network: NetworkClient;
  private readonly renderer: Renderer;
  private readonly worldState: WorldStateStore;
  private readonly lifecycleStore = new ContractLifecycleStore();
  private readonly layerCache = new Map<string, RenderLayer>();
  private readonly fallbackLayer: RenderLayer;
  private lifecycleHandlers: ClientLifecycleHandlers | null = null;
  private lastRenderVersion = -1;
  private lastRenderTime = -1;
  private joinResponse: JoinResponse | null = null;

  constructor(
    public readonly configuration: ClientManagerConfiguration,
    dependencies: {
      network: NetworkClient;
      renderer: Renderer;
      worldState: WorldStateStore;
    },
  ) {
    this.network = dependencies.network;
    this.renderer = dependencies.renderer;
    this.worldState = dependencies.worldState;
    const [firstLayer] = this.renderer.configuration.layers;
    this.fallbackLayer = firstLayer ?? { id: "effects", zIndex: 0 };
  }

  async boot(handlers: ClientLifecycleHandlers): Promise<void> {
    this.lifecycleHandlers = handlers;
    try {
      const joinResponse = await this.network.join();
      this.joinResponse = joinResponse;
      this.prepareForSession(joinResponse);
      await this.network.connect(this.createNetworkHandlers());
    } catch (error) {
      this.reportError(error);
    }
  }

  async shutdown(): Promise<void> {
    try {
      await this.network.disconnect();
    } finally {
      this.handleDisconnect();
      this.lifecycleHandlers = null;
    }
  }

  handleKeyframe(keyframe: WorldKeyframe): void {
    this.worldState.applyKeyframe(keyframe);
  }

  handlePatchBatch(patch: WorldPatchBatch): void {
    this.worldState.applyPatchBatch(patch);
  }

  requestRender(batch: RenderBatch): void {
    this.renderer.renderBatch(batch);
  }

  private createNetworkHandlers(): NetworkEventHandlers {
    return {
      onJoin: (join) => {
        this.joinResponse = join;
        this.prepareForSession(join);
        this.lifecycleHandlers?.onReady?.();
      },
      onMessage: (message) => {
        this.handleNetworkMessage(message);
      },
      onDisconnect: (code, reason) => {
        this.handleDisconnect();
        const reasonText = reason ? ` (${reason})` : "";
        this.reportError(new Error(`Disconnected from server${reasonText || ""}${code ? ` [${code}]` : ""}`));
      },
      onError: (error) => {
        this.reportError(error);
      },
    };
  }

  private handleNetworkMessage(message: NetworkMessageEnvelope): void {
    if (!message.payload || typeof message.payload !== "object") {
      return;
    }

    const payload = message.payload as Record<string, unknown>;
    if (message.type === "keyframeNack") {
      this.handleKeyframeNackPayload(payload);
      return;
    }
    if (message.type === "keyframe") {
      this.handleKeyframePayload(payload);
      return;
    }

    if (message.type !== "state") {
      return;
    }

    this.handleStatePayload(payload, message.receivedAt);
  }

  private handleStatePayload(payload: Record<string, unknown>, receivedAt: number): void {
    const effectCatalogPayload = this.extractEffectCatalogPayload(payload["config"]);
    if (!this.hydrateEffectCatalog(effectCatalogPayload)) {
      return;
    }

    if (payload["resync"] === true) {
      this.handleResync();
    }

    const batch = this.extractLifecycleBatch(payload);
    if (batch) {
      this.lifecycleStore.applyBatch(batch);
    }

    const tickValue = payload["t"];
    const tick = typeof tickValue === "number" ? tickValue : null;
    const frameTime = tick !== null ? tick * TICK_DURATION_MS : receivedAt;
    this.renderLifecycleView(frameTime);
  }

  private handleKeyframePayload(payload: Record<string, unknown>): void {
    const effectCatalogPayload = this.extractEffectCatalogPayload(payload["config"]);
    this.hydrateEffectCatalog(effectCatalogPayload);
  }

  private handleKeyframeNackPayload(payload: Record<string, unknown>): void {
    const effectCatalogPayload = this.extractEffectCatalogPayload(payload["config"]);
    this.handleResync();
    this.hydrateEffectCatalog(effectCatalogPayload);
    this.renderLifecycleView();
  }

  private extractEffectCatalogPayload(config: unknown): unknown {
    if (!config || typeof config !== "object") {
      return undefined;
    }
    return (config as { readonly effectCatalog?: unknown }).effectCatalog;
  }

  private hydrateEffectCatalog(effectCatalogPayload: unknown): boolean {
    if (effectCatalogPayload === undefined) {
      return true;
    }
    try {
      const catalogSnapshot = normalizeEffectCatalog(effectCatalogPayload);
      setEffectCatalog(catalogSnapshot);
      return true;
    } catch (error) {
      this.reportError(error);
      return false;
    }
  }

  private extractLifecycleBatch(payload: Record<string, unknown>): ContractLifecycleBatch | null {
    const spawns = payload["effect_spawned"] as
      | readonly ContractLifecycleSpawnEvent[]
      | undefined;
    const updates = payload["effect_update"] as
      | readonly ContractLifecycleUpdateEvent[]
      | undefined;
    const ends = payload["effect_ended"] as
      | readonly ContractLifecycleEndEvent[]
      | undefined;
    const cursors = payload["effect_seq_cursors"] as
      | Readonly<Record<string, number>>
      | Map<string, number>
      | undefined;

    const cursorCount = (() => {
      if (!cursors) {
        return 0;
      }
      if (cursors instanceof Map) {
        return cursors.size;
      }
      if (typeof cursors === "object") {
        return Object.keys(cursors).length;
      }
      return 0;
    })();

    const hasLifecycleEvents =
      (Array.isArray(spawns) && spawns.length > 0) ||
      (Array.isArray(updates) && updates.length > 0) ||
      (Array.isArray(ends) && ends.length > 0) ||
      cursorCount > 0;

    if (!hasLifecycleEvents) {
      return null;
    }

    return {
      spawns,
      updates,
      ends,
      cursors,
    };
  }

  private prepareForSession(join: JoinResponse): void {
    setEffectCatalog(join.effectCatalog);
    this.worldState.reset();
    this.lifecycleStore.reset();
    this.lastRenderVersion = -1;
    this.lastRenderTime = -1;
    this.renderLifecycleView();
  }

  private handleResync(): void {
    this.worldState.reset();
    this.lifecycleStore.reset();
    this.lastRenderVersion = -1;
    this.lastRenderTime = -1;
  }

  private handleDisconnect(): void {
    this.handleResync();
    this.joinResponse = null;
    this.renderLifecycleView();
  }

  private renderLifecycleView(frameTime?: number): void {
    const snapshot = this.lifecycleStore.snapshot();
    const timestamp = frameTime ?? (snapshot.lastBatchTick !== null
      ? snapshot.lastBatchTick * TICK_DURATION_MS
      : Date.now());

    if (snapshot.version === this.lastRenderVersion && timestamp === this.lastRenderTime) {
      return;
    }

    this.lastRenderVersion = snapshot.version;
    this.lastRenderTime = timestamp;
    const batch = this.buildRenderBatch(snapshot, timestamp);
    this.renderer.renderBatch(batch);
  }

  private buildRenderBatch(view: ContractLifecycleView, time: number): RenderBatch {
    const staticGeometry: StaticGeometry[] = [];
    const animations: AnimationFrame[] = [];

    for (const entry of view.entries.values()) {
      const catalogEntry = this.resolveCatalogEntry(entry);
      if (!catalogEntry) {
        continue;
      }
      const geometry = this.createStaticGeometry(entry, catalogEntry);
      if (geometry) {
        staticGeometry.push(geometry);
      }
      const animation = this.createAnimationFrame(entry, catalogEntry, "active", time);
      if (animation) {
        animations.push(animation);
      }
    }

    for (const entry of view.recentlyEnded.values()) {
      const catalogEntry = this.resolveCatalogEntry(entry);
      if (!catalogEntry) {
        continue;
      }
      const animation = this.createAnimationFrame(entry, catalogEntry, "ended", time);
      if (animation) {
        animations.push(animation);
      }
    }

    const keyframeId = view.lastBatchTick !== null ? `tick-${view.lastBatchTick}` : `lifecycle-${view.version}`;
    return {
      keyframeId,
      time,
      staticGeometry,
      animations,
    };
  }

  private resolveCatalogEntry(entry: ContractLifecycleEntry): EffectCatalogEntryMetadata | null {
    const entryId = entry.entryId ?? entry.contractId;
    if (!entryId) {
      return null;
    }
    return getEffectCatalogEntry(entryId);
  }

  private createStaticGeometry(
    entry: ContractLifecycleEntry,
    catalogEntry: EffectCatalogEntryMetadata,
  ): StaticGeometry | null {
    const { deliveryState } = entry.instance;
    const motion = deliveryState.motion;
    const geometry = deliveryState.geometry;
    if (!motion) {
      return null;
    }

    const centerX = motion.positionX + (geometry.offsetX ?? 0);
    const centerY = motion.positionY + (geometry.offsetY ?? 0);
    const layer = this.selectLayer(catalogEntry.definition.delivery);
    const vertices: [number, number][] = [];

    if (geometry.shape === "rect") {
      const width = geometry.width ?? geometry.length ?? 0;
      const height = geometry.height ?? geometry.width ?? 0;
      const halfWidth = width / 2;
      const halfHeight = height / 2;
      vertices.push(
        [centerX - halfWidth, centerY - halfHeight],
        [centerX + halfWidth, centerY - halfHeight],
        [centerX + halfWidth, centerY + halfHeight],
        [centerX - halfWidth, centerY + halfHeight],
      );
    } else {
      const radius = geometry.radius ?? Math.max(geometry.width ?? 0, geometry.height ?? 0, geometry.length ?? 0) / 2;
      const steps = 12;
      for (let index = 0; index < steps; index += 1) {
        const theta = (index / steps) * Math.PI * 2;
        vertices.push([centerX + radius * Math.cos(theta), centerY + radius * Math.sin(theta)]);
      }
    }

    if (vertices.length === 0) {
      return null;
    }

    return {
      id: entry.id,
      layer,
      vertices,
      style: {
        shape: geometry.shape,
        managedByClient: catalogEntry.managedByClient,
        entryId: entry.entryId,
      },
    };
  }

  private createAnimationFrame(
    entry: ContractLifecycleEntry,
    catalogEntry: EffectCatalogEntryMetadata,
    phase: "active" | "ended",
    frameTime: number,
  ): AnimationFrame | null {
    const motion = entry.instance.deliveryState.motion;
    if (!motion) {
      return null;
    }

    const startTick = entry.instance.startTick ?? entry.seq;
    const startedAt = startTick * TICK_DURATION_MS;
    const behaviorState = entry.instance.behaviorState;
    const ticksRemaining = behaviorState?.ticksRemaining ?? catalogEntry.definition.lifetimeTicks;
    const durationTicks = phase === "ended" ? 0 : Math.max(ticksRemaining, 0);
    const geometry = entry.instance.deliveryState.geometry;
    const radius = geometry.radius ?? Math.max(geometry.width ?? 0, geometry.height ?? 0) / 2;

    return {
      effectId: entry.id,
      startedAt,
      durationMs: durationTicks * TICK_DURATION_MS,
      metadata: {
        state: phase,
        contractId: entry.contractId,
        entryId: entry.entryId,
        managedByClient: catalogEntry.managedByClient,
        lastEventKind: entry.lastEventKind,
        retained: entry.retained,
        catalog: catalogEntry,
        instance: entry.instance,
        position: { x: motion.positionX, y: motion.positionY },
        radius: radius > 0 ? radius : undefined,
        fill: catalogEntry.managedByClient ? "rgba(96, 204, 255, 0.6)" : "rgba(255, 196, 72, 0.5)",
        stroke: catalogEntry.managedByClient ? "rgba(96, 204, 255, 0.9)" : "rgba(255, 196, 72, 0.8)",
        blocks: catalogEntry.blocks,
        renderedAt: frameTime,
      },
    };
  }

  private selectLayer(delivery: DeliveryKind): RenderLayer {
    const cached = this.layerCache.get(delivery);
    if (cached) {
      return cached;
    }

    const candidateIds = [delivery, `effect-${delivery}`, `effects-${delivery}`];
    const resolved =
      this.renderer.configuration.layers.find((layer) => candidateIds.includes(layer.id)) ??
      this.fallbackLayer;
    this.layerCache.set(delivery, resolved);
    return resolved;
  }

  private reportError(cause: unknown): void {
    const error = cause instanceof Error ? cause : new Error(String(cause));
    this.lifecycleHandlers?.onError?.(error);
  }
}

const TICK_DURATION_MS = 16;
