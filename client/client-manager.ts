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
  HeartbeatAck,
  JoinResponse,
  NetworkClient,
  NetworkEventHandlers,
  NetworkMessageEnvelope,
} from "./network";
import {
  NetworkInputActionDispatcher,
  type InputActionDispatcher,
  type PathCommandState,
  type PathTarget,
  type PlayerIntent,
} from "./input";
import type { RenderBatch, Renderer, StaticGeometry, AnimationFrame, RenderLayer } from "./render";
import type { WorldPatchBatch, WorldStateStore, WorldKeyframe } from "./world-state";
import type { DeliveryKind } from "./generated/effect-contracts";

export interface ClientManagerConfiguration {
  readonly autoConnect: boolean;
  readonly reconcileIntervalMs: number;
  readonly keyframeRetryDelayMs: number;
  readonly keyframeRetryPolicy?: {
    readonly baseMs?: number;
    readonly maxMs?: number;
    readonly multiplier?: number;
    readonly jitterMs?: number;
  };
}

export interface ClientLifecycleHandlers {
  readonly onReady?: () => void;
  readonly onError?: (error: Error) => void;
  readonly onHeartbeat?: (telemetry: ClientHeartbeatTelemetry) => void;
}

export interface ClientOrchestrator {
  readonly configuration: ClientManagerConfiguration;
  readonly boot: (handlers: ClientLifecycleHandlers) => Promise<void>;
  readonly shutdown: () => Promise<void>;
  readonly handleKeyframe: (keyframe: WorldKeyframe) => void;
  readonly handlePatchBatch: (patch: WorldPatchBatch) => void;
  readonly requestRender: (batch: RenderBatch) => void;
  readonly getJoinResponse: () => JoinResponse | null;
}

export interface InputDispatcherHooks {
  readonly onIntentDispatched?: (intent: PlayerIntent) => void;
  readonly onPathCommand?: (state: PathCommandState) => void;
}

export interface ClientHeartbeatTelemetry {
  readonly serverTime: number | null;
  readonly clientTime: number | null;
  readonly roundTripTimeMs: number | null;
  readonly receivedAt: number;
}

interface PendingKeyframeRetry {
  sequence: number | null;
  earliestRetryAt: number;
  awaitingResync: boolean;
  attempt?: number;
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
  private lastRenderPathSignature = "";
  private joinResponse: JoinResponse | null = null;
  private latestKeyframeSequence: number | null = null;
  private keyframeRequestInFlight: number | null = null;
  private pendingKeyframeRetry: PendingKeyframeRetry | null = null;
  private pendingKeyframeRetryTimer: ReturnType<typeof setTimeout> | null = null;
  private lastKeyframeRequestAt = 0;
  private lastPatchSequence: number | null = null;
  private maxPatchSequenceSeen: number | null = null;
  private pendingGapSequence: number | null = null;
  private latestAcknowledgedTick: number | null = null;
  private inputDispatchPaused = true;
  private pathCommandState: PathCommandState = { active: false, target: null };
  private inputDispatcherHooks: InputDispatcherHooks | null = null;

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

  getJoinResponse(): JoinResponse | null {
    return this.joinResponse;
  }

  get playerId(): string | null {
    return this.joinResponse?.id ?? null;
  }

  createInputDispatcher(hooks: InputDispatcherHooks = {}): InputActionDispatcher {
    this.inputDispatcherHooks = hooks;
    hooks.onPathCommand?.(this.clonePathCommandState(this.pathCommandState));
    return new NetworkInputActionDispatcher({
      getProtocolVersion: () => this.joinResponse?.protocolVersion ?? null,
      getAcknowledgedTick: () => this.latestAcknowledgedTick,
      isDispatchPaused: () => this.inputDispatchPaused,
      sendMessage: (payload) => {
        this.sendClientCommand(payload);
      },
      onIntentDispatched: (intent: PlayerIntent) => {
        hooks.onIntentDispatched?.(intent);
        this.handleIntentDispatched(intent);
      },
      onPathCommand: (state) => {
        hooks.onPathCommand?.(state);
        this.handlePathCommand(state);
      },
    });
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
      onHeartbeat: (ack) => {
        this.handleHeartbeatAck(ack);
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

    const keyframeSequence = this.extractSequence(payload["keyframeSeq"]);
    if (keyframeSequence !== null) {
      this.latestKeyframeSequence = keyframeSequence;
    }

    const tickValue = payload["t"];
    const tick = typeof tickValue === "number" && Number.isFinite(tickValue) ? Math.floor(tickValue) : null;
    if (tick !== null) {
      this.latestAcknowledgedTick = tick;
    }

    const isResync = payload["resync"] === true;
    if (isResync) {
      this.handleResync();
      if (this.pendingKeyframeRetry) {
        this.pendingKeyframeRetry.awaitingResync = false;
        if (keyframeSequence !== null) {
          this.pendingKeyframeRetry.sequence = keyframeSequence;
        } else if (this.pendingKeyframeRetry.sequence === null && this.latestKeyframeSequence !== null) {
          this.pendingKeyframeRetry.sequence = this.latestKeyframeSequence;
        }
      }
    }

    const patchSequence = this.extractSequence(payload["sequence"]);
    this.trackPatchSequenceProgress(patchSequence, keyframeSequence);

    const batch = this.extractLifecycleBatch(payload);
    if (batch) {
      this.lifecycleStore.applyBatch(batch);
    }

    const frameTime = tick !== null ? tick * TICK_DURATION_MS : receivedAt;
    this.renderLifecycleView(frameTime);

    this.inputDispatchPaused = false;

    if (isResync) {
      this.applyPendingKeyframeRetry();
    }
  }

  private handleKeyframePayload(payload: Record<string, unknown>): void {
    const effectCatalogPayload = this.extractEffectCatalogPayload(payload["config"]);
    this.hydrateEffectCatalog(effectCatalogPayload);
    const sequence = this.extractSequence(payload["sequence"]);
    if (sequence !== null) {
      this.latestKeyframeSequence = sequence;
    }
    this.keyframeRequestInFlight = null;
    this.pendingKeyframeRetry = null;
    this.clearPendingKeyframeRetryTimer();
    this.resetPatchSequenceTracking();
  }

  private handleKeyframeNackPayload(payload: Record<string, unknown>): void {
    const effectCatalogPayload = this.extractEffectCatalogPayload(payload["config"]);
    const sequence = this.extractSequence(payload["sequence"]);
    const resyncRequested = payload["resync"] === true;
    this.handleResync();
    this.hydrateEffectCatalog(effectCatalogPayload);
    this.keyframeRequestInFlight = null;
    const now = Date.now();
    const policy = resolveRetryPolicy(this.configuration);
    const attempt = (this.pendingKeyframeRetry?.attempt ?? 0) + 1;
    const retryDelay = computeRetryDelayMs(attempt, policy);
    const earliestRetryAt = Math.max(now + retryDelay, this.lastKeyframeRequestAt + retryDelay);
    this.pendingKeyframeRetry = {
      sequence,
      earliestRetryAt,
      awaitingResync: resyncRequested,
      attempt,
    };
    if (!resyncRequested) {
      this.applyPendingKeyframeRetry();
    }
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
    this.lastRenderPathSignature = "";
    this.latestKeyframeSequence = null;
    this.keyframeRequestInFlight = null;
    this.pendingKeyframeRetry = null;
    this.lastKeyframeRequestAt = 0;
    this.resetPatchSequenceTracking();
    this.clearPendingKeyframeRetryTimer();
    this.latestAcknowledgedTick = null;
    this.inputDispatchPaused = true;
    this.applyPathCommandState({ active: false, target: null }, { notifyHooks: true });
    this.renderLifecycleView();
  }

  private handleResync(): void {
    this.worldState.reset();
    this.lifecycleStore.reset();
    this.lastRenderVersion = -1;
    this.lastRenderTime = -1;
    this.lastRenderPathSignature = "";
    this.resetPatchSequenceTracking();
    this.clearPendingKeyframeRetryTimer();
    this.latestAcknowledgedTick = null;
    this.inputDispatchPaused = true;
    this.applyPathCommandState({ active: false, target: null }, { notifyHooks: true });
  }

  private handleDisconnect(): void {
    this.handleResync();
    this.joinResponse = null;
    this.latestKeyframeSequence = null;
    this.keyframeRequestInFlight = null;
    this.pendingKeyframeRetry = null;
    this.lastKeyframeRequestAt = 0;
    this.resetPatchSequenceTracking();
    this.applyPathCommandState({ active: false, target: null }, { notifyHooks: false });
    this.renderLifecycleView();
  }

  private handleHeartbeatAck(ack: HeartbeatAck): void {
    const telemetry: ClientHeartbeatTelemetry = {
      serverTime: Number.isFinite(ack.serverTime) ? ack.serverTime : null,
      clientTime: Number.isFinite(ack.clientTime) ? ack.clientTime : null,
      roundTripTimeMs: Number.isFinite(ack.roundTripTime) ? ack.roundTripTime : null,
      receivedAt: ack.receivedAt,
    };
    this.lifecycleHandlers?.onHeartbeat?.(telemetry);
  }

  private renderLifecycleView(frameTime?: number): void {
    const snapshot = this.lifecycleStore.snapshot();
    const timestamp = frameTime ?? (snapshot.lastBatchTick !== null
      ? snapshot.lastBatchTick * TICK_DURATION_MS
      : Date.now());
    const pathSignature = this.serializePathCommandState(this.pathCommandState);

    if (
      snapshot.version === this.lastRenderVersion &&
      timestamp === this.lastRenderTime &&
      pathSignature === this.lastRenderPathSignature
    ) {
      return;
    }

    this.lastRenderVersion = snapshot.version;
    this.lastRenderTime = timestamp;
    this.lastRenderPathSignature = pathSignature;
    const batch = this.buildRenderBatch(snapshot, timestamp);
    this.renderer.renderBatch(batch);
  }

  private buildRenderBatch(view: ContractLifecycleView, time: number): RenderBatch {
    const staticGeometry: StaticGeometry[] = [];
    const animations: AnimationFrame[] = [];
    const pathTarget = this.clonePathTarget(this.pathCommandState);

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
      pathTarget,
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

  private applyPendingKeyframeRetry(): void {
    const pending = this.pendingKeyframeRetry;
    if (!pending || pending.awaitingResync || this.keyframeRequestInFlight !== null) {
      return;
    }

    const normalized = this.extractSequence(pending.sequence ?? this.latestKeyframeSequence);
    if (normalized === null) {
      return;
    }

    const now = Date.now();
    if (now < pending.earliestRetryAt) {
      this.schedulePendingKeyframeRetry(pending.earliestRetryAt - now);
      return;
    }

    if (this.sendKeyframeRequest(normalized)) {
      this.pendingKeyframeRetry = null;
    } else {
      const policy = resolveRetryPolicy(this.configuration);
      const nextAttempt = (pending.attempt ?? 0) + 1;
      const nextDelay = computeRetryDelayMs(nextAttempt, policy);
      pending.attempt = nextAttempt;
      pending.earliestRetryAt = now + nextDelay;
      this.schedulePendingKeyframeRetry(nextDelay);
    }
  }

  private schedulePendingKeyframeRetry(delayMs: number): void {
    this.clearPendingKeyframeRetryTimer();
    const delay = Math.max(0, Math.floor(delayMs));
    this.pendingKeyframeRetryTimer = setTimeout(() => {
      this.pendingKeyframeRetryTimer = null;
      this.applyPendingKeyframeRetry();
    }, delay);
  }

  private clearPendingKeyframeRetryTimer(): void {
    if (this.pendingKeyframeRetryTimer !== null) {
      clearTimeout(this.pendingKeyframeRetryTimer);
      this.pendingKeyframeRetryTimer = null;
    }
  }

  private trackPatchSequenceProgress(
    patchSequence: number | null,
    keyframeSequence: number | null,
  ): void {
    if (patchSequence === null) {
      return;
    }

    this.maxPatchSequenceSeen =
      this.maxPatchSequenceSeen === null
        ? patchSequence
        : Math.max(this.maxPatchSequenceSeen, patchSequence);

    if (this.lastPatchSequence === null) {
      this.lastPatchSequence = patchSequence;
      return;
    }

    if (patchSequence <= this.lastPatchSequence) {
      return;
    }

    const expectedNext = this.lastPatchSequence + 1;
    if (patchSequence === expectedNext) {
      this.lastPatchSequence = patchSequence;
      if (this.pendingGapSequence !== null) {
        if (this.maxPatchSequenceSeen !== null && this.lastPatchSequence >= this.maxPatchSequenceSeen) {
          this.clearResolvedKeyframeGap();
        } else {
          this.pendingGapSequence = this.lastPatchSequence + 1;
        }
      }
      return;
    }

    const gapStart = expectedNext;
    if (this.pendingGapSequence === null || gapStart < this.pendingGapSequence) {
      this.pendingGapSequence = gapStart;
    }

    const sequenceToRequest = keyframeSequence ?? this.latestKeyframeSequence ?? patchSequence;
    this.ensureKeyframeRetry(sequenceToRequest);
  }

  private ensureKeyframeRetry(sequence: number | null): void {
    const normalized = this.extractSequence(sequence);
    if (normalized === null) {
      return;
    }

    const now = Date.now();
    const policy = resolveRetryPolicy(this.configuration);
    const minRetryAt =
      this.lastKeyframeRequestAt > 0 ? this.lastKeyframeRequestAt + policy.baseMs : now;
    const retryAt = Math.max(now, minRetryAt);

    if (this.pendingKeyframeRetry) {
      const pending = this.pendingKeyframeRetry;
      if (pending.sequence === null || normalized > pending.sequence) {
        pending.sequence = normalized;
        pending.attempt = 0;
      } else if (normalized < pending.sequence) {
        pending.sequence = normalized;
      }
      pending.awaitingResync = false;
      if (pending.earliestRetryAt > retryAt) {
        pending.earliestRetryAt = retryAt;
      }
      this.pendingKeyframeRetry = pending;
    } else {
      this.pendingKeyframeRetry = {
        sequence: normalized,
        earliestRetryAt: retryAt,
        awaitingResync: false,
        attempt: 0,
      };
    }

    this.applyPendingKeyframeRetry();
  }

  private clearResolvedKeyframeGap(): void {
    this.pendingGapSequence = null;
    const pending = this.pendingKeyframeRetry;
    if (pending && !pending.awaitingResync) {
      this.pendingKeyframeRetry = null;
      this.clearPendingKeyframeRetryTimer();
    }
  }

  private resetPatchSequenceTracking(): void {
    this.lastPatchSequence = null;
    this.maxPatchSequenceSeen = null;
    this.pendingGapSequence = null;
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

  private sendKeyframeRequest(sequence: number): boolean {
    if (this.keyframeRequestInFlight !== null) {
      return false;
    }

    if (!Number.isFinite(sequence) || sequence <= 0) {
      return false;
    }

    const normalized = Math.floor(sequence);
    const payload: Record<string, unknown> = {
      type: "keyframeRequest",
      keyframeSeq: normalized,
    };

    const version = this.joinResponse?.protocolVersion;
    if (typeof version === "number") {
      payload.ver = version;
    }

    try {
      this.network.send(payload);
      this.keyframeRequestInFlight = normalized;
      this.lastKeyframeRequestAt = Date.now();
      return true;
    } catch (error) {
      this.reportError(error);
      return false;
    }
  }

  private extractSequence(value: unknown): number | null {
    if (typeof value !== "number" || !Number.isFinite(value)) {
      return null;
    }
    const normalized = Math.floor(value);
    return normalized > 0 ? normalized : null;
  }

  private sendClientCommand(payload: Record<string, unknown>): void {
    try {
      this.network.send(payload);
    } catch (error) {
      this.reportError(error);
    }
  }

  private handleIntentDispatched(_intent: PlayerIntent): void {
    // Reserved for future intent instrumentation.
  }

  private handlePathCommand(state: PathCommandState): void {
    this.applyPathCommandState(state, { notifyHooks: false });
  }

  private applyPathCommandState(state: PathCommandState, options: { notifyHooks: boolean }): void {
    const normalized = this.normalizePathCommandState(state);
    const signatureBefore = this.serializePathCommandState(this.pathCommandState);
    this.pathCommandState = normalized;
    if (options.notifyHooks) {
      this.inputDispatcherHooks?.onPathCommand?.(this.clonePathCommandState(normalized));
    }
    const signatureAfter = this.serializePathCommandState(normalized);
    if (signatureBefore !== signatureAfter) {
      this.renderLifecycleView();
    }
  }

  private normalizePathCommandState(state: PathCommandState): PathCommandState {
    if (!state.active || !state.target) {
      return { active: false, target: null };
    }
    const { x, y } = state.target;
    if (!Number.isFinite(x) || !Number.isFinite(y)) {
      return { active: false, target: null };
    }
    return { active: true, target: { x, y } };
  }

  private clonePathCommandState(state: PathCommandState): PathCommandState {
    return state.active && state.target
      ? { active: true, target: { x: state.target.x, y: state.target.y } }
      : { active: false, target: null };
  }

  private clonePathTarget(state: PathCommandState): PathTarget | null {
    return state.active && state.target
      ? { x: state.target.x, y: state.target.y }
      : null;
  }

  private serializePathCommandState(state: PathCommandState): string {
    if (!state.active || !state.target) {
      return "inactive";
    }
    return `active:${state.target.x}:${state.target.y}`;
  }
}

interface ResolvedKeyframeRetryPolicy {
  readonly baseMs: number;
  readonly maxMs: number;
  readonly multiplier: number;
  readonly jitterMs: number;
}

function computeRetryDelayMs(attempt: number, cfg: ResolvedKeyframeRetryPolicy): number {
  if (attempt <= 1) {
    return cfg.baseMs;
  }

  let delay = cfg.baseMs * Math.pow(cfg.multiplier, attempt - 1);
  if (!Number.isFinite(delay)) {
    delay = cfg.maxMs;
  }
  delay = Math.min(delay, cfg.maxMs);
  const jitter = cfg.jitterMs > 0 ? Math.floor(Math.random() * cfg.jitterMs) : 0;
  return delay + jitter;
}

function resolveRetryPolicy(configuration: ClientManagerConfiguration): ResolvedKeyframeRetryPolicy {
  const base = Math.max(0, configuration.keyframeRetryPolicy?.baseMs ?? configuration.keyframeRetryDelayMs ?? 200);
  const max = Math.max(base, configuration.keyframeRetryPolicy?.maxMs ?? 2000);
  const multiplier = Math.max(1, configuration.keyframeRetryPolicy?.multiplier ?? 2);
  const jitter = Math.max(0, configuration.keyframeRetryPolicy?.jitterMs ?? 100);
  return { baseMs: base, maxMs: max, multiplier, jitterMs: jitter };
}

const TICK_DURATION_MS = 16;
