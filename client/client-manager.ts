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
import {
  normalizeGroundItemSnapshots,
  normalizeNPCSnapshots,
  normalizeNetworkPatches,
  normalizeObstacleSnapshots,
  normalizePlayerSnapshots,
  type GroundItemSnapshot,
  type HeartbeatAck,
  type JoinResponse,
  type NetworkClient,
  type NetworkEventHandlers,
  type NetworkMessageEnvelope,
  type NetworkPatch,
  type NPCSnapshot,
  type ObstacleSnapshot,
  type PlayerSnapshot,
  type WorldConfigurationSnapshot,
} from "./network";
import {
  NetworkInputActionDispatcher,
  type CommandAcknowledgementDetails,
  type CommandKind,
  type CommandRejectionDetails,
  type InputActionDispatcher,
  type PathCommandState,
  type PathTarget,
  type PlayerIntent,
} from "./input";
import { translateRenderAnimation } from "./effect-runtime-adapter";
import type {
  RenderBatch,
  Renderer,
  StaticGeometry,
  AnimationFrame,
  RenderLayer,
  RuntimeEffectFrame,
  RenderDimensions,
} from "./render";
import type {
  WorldEntityState,
  WorldPatchBatch,
  WorldPatchOperation,
  WorldStateSnapshot,
  WorldStateStore,
  WorldKeyframe,
} from "./world-state";
import type { DeliveryKind } from "./generated/effect-contracts";

const WORLD_BACKGROUND_LAYER: RenderLayer = { id: "world-background", zIndex: -200 };
const WORLD_GRID_LAYER: RenderLayer = { id: "world-grid", zIndex: -150 };
const WORLD_OBSTACLE_LAYER: RenderLayer = { id: "world-obstacles", zIndex: -80 };
const WORLD_NPC_LAYER: RenderLayer = { id: "world-npcs", zIndex: -40 };
const WORLD_PLAYER_LAYER: RenderLayer = { id: "world-players", zIndex: -30 };
const WORLD_GROUND_ITEM_LAYER: RenderLayer = { id: "world-ground-items", zIndex: -35 };

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
  readonly onLog?: (message: string) => void;
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
  readonly onCommandRejectionChanged?: (rejection: CommandRejectionDetails | null) => void;
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

interface WorldSnapshotPayload {
  readonly players: readonly PlayerSnapshot[];
  readonly npcs: readonly NPCSnapshot[];
  readonly obstacles: readonly ObstacleSnapshot[];
  readonly groundItems: readonly GroundItemSnapshot[];
}

const cloneCommandRejectionDetails = (rejection: CommandRejectionDetails): CommandRejectionDetails => ({
  sequence: rejection.sequence,
  reason: rejection.reason,
  retry: rejection.retry,
  tick: rejection.tick,
  kind: rejection.kind,
});

export class GameClientOrchestrator implements ClientOrchestrator {
  private readonly network: NetworkClient;
  private readonly renderer: Renderer;
  private readonly worldState: WorldStateStore;
  private readonly lifecycleStore = new ContractLifecycleStore();
  private readonly layerCache = new Map<string, RenderLayer>();
  private lifecycleHandlers: ClientLifecycleHandlers | null = null;
  private lastRenderVersion = -1;
  private lastRenderTime = -1;
  private lastRenderPathSignature = "";
  private joinResponse: JoinResponse | null = null;
  private latestWorldSnapshot: WorldStateSnapshot | null = null;
  private worldRenderVersion = 0;
  private lastRenderedWorldVersion = -1;
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
  private inputDispatcher: InputActionDispatcher | null = null;
  private pathCommandState: PathCommandState = { active: false, target: null };
  private inputDispatcherHooks: InputDispatcherHooks | null = null;
  private lastCommandRejection: CommandRejectionDetails | null = null;
  private lastSyncedRendererDimensions: RenderDimensions | null = null;

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
    if (this.lastCommandRejection) {
      hooks.onCommandRejectionChanged?.(cloneCommandRejectionDetails(this.lastCommandRejection));
    }
    const dispatcher = new NetworkInputActionDispatcher({
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
      onCommandRejected: (rejection) => {
        this.handleCommandRejection(rejection);
      },
      onCommandAcknowledged: (ack) => {
        this.handleCommandAcknowledgement(ack);
      },
      onCommandsReset: () => {
        this.handleCommandReset();
      },
    });
    this.inputDispatcher = dispatcher;
    return dispatcher;
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

    if (message.type === "commandAck") {
      this.handleCommandAckPayload(payload);
      return;
    }

    if (message.type === "commandReject") {
      this.handleCommandRejectPayload(payload);
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
    const tick = this.extractTick(tickValue);
    this.recordAcknowledgedTick(tick);

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

    let worldStateChanged = false;
    let worldFrameTime: number | undefined;
    let worldKeyframeId: string | null = null;
    const frameTime = tick !== null ? tick * TICK_DURATION_MS : receivedAt;

    const worldSnapshot = this.extractWorldSnapshotFromStatePayload(payload);
    if (worldSnapshot) {
      const worldConfig = this.extractWorldDimensions(payload["config"]);
      const keyframeId = this.resolveWorldKeyframeId("state", keyframeSequence, patchSequence, tick);
      const metadata = this.createWorldMetadata({
        source: "state",
        world: worldConfig,
        tick,
        sequence: patchSequence,
        keyframeSequence,
      });
      const keyframe = this.createWorldKeyframe({
        id: keyframeId,
        timestamp: frameTime,
        snapshot: worldSnapshot,
        metadata,
      });
      this.worldState.applyKeyframe(keyframe);
      this.logWorldSnapshot("state", worldSnapshot, {
        world: worldConfig,
        tick,
        sequence: patchSequence,
        keyframeSequence,
      });
      worldStateChanged = true;
      worldFrameTime = frameTime;
      worldKeyframeId = keyframeId;
    }

    if (this.payloadHasField(payload, "patches")) {
      try {
        const patches = normalizeNetworkPatches(payload["patches"], "state.patches");
        const batch = this.createWorldPatchBatch(patches, {
          keyframeId: this.resolvePatchKeyframeId(keyframeSequence, patchSequence, worldKeyframeId, "state"),
          timestamp: frameTime,
        });
        if (batch) {
          this.worldState.applyPatchBatch(batch);
          this.logPatchBatch("state", patches.length, batch.operations.length, {
            tick,
            sequence: patchSequence,
            keyframeSequence,
          });
          worldStateChanged = true;
          worldFrameTime = worldFrameTime ?? frameTime;
        }
      } catch (error) {
        this.reportError(error);
      }
    }

    const batch = this.extractLifecycleBatch(payload);
    if (batch) {
      this.lifecycleStore.applyBatch(batch);
    }

    if (worldStateChanged) {
      this.updateWorldSnapshot(worldFrameTime ?? frameTime, false);
    }

    this.renderLifecycleView(frameTime);

    this.inputDispatchPaused = false;
    this.inputDispatcher?.handleDispatchResume();

    if (isResync) {
      this.applyPendingKeyframeRetry();
    }
  }

  private handleCommandAckPayload(payload: Record<string, unknown>): void {
    const sequence = this.extractSequence(payload["seq"]);
    if (sequence === null) {
      return;
    }
    const tickValue = payload["tick"];
    let tick: number | null = null;
    if (typeof tickValue === "number" && Number.isFinite(tickValue) && tickValue >= 0) {
      tick = Math.floor(tickValue);
    }
    this.recordAcknowledgedTick(tick);
    this.inputDispatcher?.handleCommandAck({ sequence, tick });
  }

  private handleCommandRejectPayload(payload: Record<string, unknown>): void {
    const sequence = this.extractSequence(payload["seq"]);
    if (sequence === null) {
      return;
    }
    const reasonValue = payload["reason"];
    const reason = typeof reasonValue === "string" && reasonValue.length > 0 ? reasonValue : "unknown";
    const retry = payload["retry"] === true;
    const tickValue = payload["tick"];
    let tick: number | null = null;
    if (typeof tickValue === "number" && Number.isFinite(tickValue) && tickValue >= 0) {
      tick = Math.floor(tickValue);
    }
    this.recordAcknowledgedTick(tick);
    this.inputDispatcher?.handleCommandReject({ sequence, reason, retry, tick });
  }

  private handleKeyframePayload(payload: Record<string, unknown>): void {
    const effectCatalogPayload = this.extractEffectCatalogPayload(payload["config"]);
    this.hydrateEffectCatalog(effectCatalogPayload);
    const sequence = this.extractSequence(payload["sequence"]);
    if (sequence !== null) {
      this.latestKeyframeSequence = sequence;
    }
    const tick = this.extractTick(payload["t"]);
    let worldFrameTime: number | undefined;
    const worldSnapshot = this.extractWorldSnapshotFromKeyframePayload(payload);
    if (worldSnapshot) {
      const worldConfig = this.extractWorldDimensions(payload["config"]);
      const timestamp = tick !== null ? tick * TICK_DURATION_MS : Date.now();
      const keyframeId = this.resolveWorldKeyframeId("keyframe", sequence, null, tick);
      const metadata = this.createWorldMetadata({
        source: "keyframe",
        world: worldConfig,
        tick,
        sequence,
        keyframeSequence: sequence,
      });
      const keyframe = this.createWorldKeyframe({
        id: keyframeId,
        timestamp,
        snapshot: worldSnapshot,
        metadata,
      });
      this.worldState.applyKeyframe(keyframe);
      this.updateWorldSnapshot(timestamp, false);
      worldFrameTime = timestamp;
    }
    this.keyframeRequestInFlight = null;
    this.pendingKeyframeRetry = null;
    this.clearPendingKeyframeRetryTimer();
    this.resetPatchSequenceTracking();
    this.renderLifecycleView(worldFrameTime);
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
    this.renderer.reset();
    this.lifecycleStore.reset();
    this.lastRenderVersion = -1;
    this.lastRenderTime = -1;
    this.lastRenderPathSignature = "";
    this.latestWorldSnapshot = null;
    this.worldRenderVersion = 0;
    this.lastRenderedWorldVersion = -1;
    this.latestKeyframeSequence = null;
    this.keyframeRequestInFlight = null;
    this.pendingKeyframeRetry = null;
    this.lastKeyframeRequestAt = 0;
    this.resetPatchSequenceTracking();
    this.clearPendingKeyframeRetryTimer();
    this.latestAcknowledgedTick = null;
    this.inputDispatchPaused = true;
    this.inputDispatcher?.handleResync();
    this.applyPathCommandState({ active: false, target: null }, { notifyHooks: true });
    this.clearCommandRejection();
    this.lastSyncedRendererDimensions = null;
    this.hydrateWorldFromJoin(join);
    this.renderLifecycleView();
  }

  private handleResync(): void {
    this.worldState.reset();
    this.renderer.reset();
    this.lifecycleStore.reset();
    this.lastRenderVersion = -1;
    this.lastRenderTime = -1;
    this.lastRenderPathSignature = "";
    this.latestWorldSnapshot = null;
    this.worldRenderVersion = 0;
    this.lastRenderedWorldVersion = -1;
    this.resetPatchSequenceTracking();
    this.clearPendingKeyframeRetryTimer();
    this.latestAcknowledgedTick = null;
    this.inputDispatchPaused = true;
    this.inputDispatcher?.handleResync();
    this.applyPathCommandState({ active: false, target: null }, { notifyHooks: true });
    this.clearCommandRejection();
    this.lastSyncedRendererDimensions = null;
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
    this.clearCommandRejection();
    this.lastSyncedRendererDimensions = null;
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
    const lifecycleSnapshot = this.lifecycleStore.snapshot();
    const timestamp = frameTime ?? (lifecycleSnapshot.lastBatchTick !== null
      ? lifecycleSnapshot.lastBatchTick * TICK_DURATION_MS
      : Date.now());
    const pathSignature = this.serializePathCommandState(this.pathCommandState);
    const worldVersion = this.worldRenderVersion;
    const worldSnapshot = this.latestWorldSnapshot;

    if (
      lifecycleSnapshot.version === this.lastRenderVersion &&
      timestamp === this.lastRenderTime &&
      pathSignature === this.lastRenderPathSignature &&
      worldVersion === this.lastRenderedWorldVersion
    ) {
      return;
    }

    this.lastRenderVersion = lifecycleSnapshot.version;
    this.lastRenderTime = timestamp;
    this.lastRenderPathSignature = pathSignature;
    this.lastRenderedWorldVersion = worldVersion;
    const batch = this.buildRenderBatch(lifecycleSnapshot, timestamp, worldSnapshot);
    this.renderer.renderBatch(batch);
  }

  private buildRenderBatch(
    view: ContractLifecycleView,
    time: number,
    worldSnapshot: WorldStateSnapshot | null,
  ): RenderBatch {
    const staticGeometry: StaticGeometry[] = [];
    staticGeometry.push(...this.buildWorldStaticGeometry(worldSnapshot));
    const animations: AnimationFrame[] = [];
    const runtimeEffects: RuntimeEffectFrame[] = [];
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
        runtimeEffects.push(this.createRuntimeEffect(animation));
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
        runtimeEffects.push(this.createRuntimeEffect(animation));
      }
    }

    const keyframeId = view.lastBatchTick !== null ? `tick-${view.lastBatchTick}` : `lifecycle-${view.version}`;
    return {
      keyframeId,
      time,
      staticGeometry,
      animations,
      pathTarget,
      runtimeEffects,
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

  private createRuntimeEffect(animation: AnimationFrame): RuntimeEffectFrame {
    return {
      effectId: animation.effectId,
      intent: translateRenderAnimation(animation),
    };
  }

  private hydrateWorldFromJoin(join: JoinResponse): void {
    const snapshot: WorldSnapshotPayload = {
      players: join.players ?? [],
      npcs: join.npcs ?? [],
      obstacles: join.obstacles ?? [],
      groundItems: join.groundItems ?? [],
    };
    const timestamp = Date.now();
    const keyframeId = `join-${join.id}`;
    const keyframe = this.createWorldKeyframe({
      id: keyframeId,
      timestamp,
      snapshot,
      metadata: this.createWorldMetadata({
        source: "join",
        world: join.world,
        additional: { seed: join.seed },
      }),
    });
    this.worldState.applyKeyframe(keyframe);
    this.logWorldSnapshot("join", snapshot, {
      world: join.world,
      seed: join.seed,
    });

    if (join.patches.length > 0) {
      const batch = this.createWorldPatchBatch(join.patches, {
        keyframeId,
        timestamp,
      });
      if (batch) {
        this.worldState.applyPatchBatch(batch);
        this.logPatchBatch("join", join.patches.length, batch.operations.length, {});
      }
    }

    this.updateWorldSnapshot(timestamp, false);
  }

  private createWorldKeyframe(options: {
    readonly id: string;
    readonly timestamp: number;
    readonly snapshot: WorldSnapshotPayload;
    readonly metadata: Record<string, unknown>;
  }): WorldKeyframe {
    const entities = this.createWorldEntities(options.snapshot);
    return {
      id: options.id,
      timestamp: options.timestamp,
      entities,
      metadata: { ...options.metadata },
    };
  }

  private createWorldEntities(snapshot: WorldSnapshotPayload): WorldEntityState[] {
    const entities: WorldEntityState[] = [];

    for (const player of snapshot.players) {
      const entity = this.translatePlayerEntity(player);
      if (entity) {
        entities.push(entity);
      }
    }

    for (const npc of snapshot.npcs) {
      const entity = this.translateNPCEntity(npc);
      if (entity) {
        entities.push(entity);
      }
    }

    for (const obstacle of snapshot.obstacles) {
      const entity = this.translateObstacleEntity(obstacle);
      if (entity) {
        entities.push(entity);
      }
    }

    for (const item of snapshot.groundItems) {
      const entity = this.translateGroundItemEntity(item);
      if (entity) {
        entities.push(entity);
      }
    }

    return entities;
  }

  private buildWorldStaticGeometry(worldSnapshot: WorldStateSnapshot | null): StaticGeometry[] {
    const geometry: StaticGeometry[] = [];
    const dimensions = this.resolveWorldDimensions(worldSnapshot);
    if (dimensions) {
      this.syncRendererDimensions(dimensions);
      const background = this.createWorldBackgroundGeometry(dimensions);
      if (background) {
        geometry.push(background);
      }
      const grid = this.createWorldGridGeometry(dimensions);
      if (grid) {
        geometry.push(grid);
      }
    }

    if (!worldSnapshot) {
      return geometry;
    }

    for (const entity of worldSnapshot.entities.values()) {
      const entries = this.createGeometryForEntity(entity);
      for (const entry of entries) {
        geometry.push(entry);
      }
    }

    return geometry;
  }

  private syncRendererDimensions(dimensions: WorldConfigurationSnapshot | null): void {
    if (!dimensions) {
      return;
    }
    const { width, height } = dimensions;
    if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) {
      return;
    }
    const next: RenderDimensions = { width, height };
    const last = this.lastSyncedRendererDimensions;
    if (last && last.width === next.width && last.height === next.height) {
      return;
    }
    this.lastSyncedRendererDimensions = { ...next };
    this.renderer.resize({ ...next });
  }

  private resolveWorldDimensions(worldSnapshot: WorldStateSnapshot | null): WorldConfigurationSnapshot | null {
    const fromSnapshot = this.extractWorldDimensionsFromSnapshot(worldSnapshot);
    if (fromSnapshot) {
      return fromSnapshot;
    }
    const joinWorld = this.joinResponse?.world;
    if (!joinWorld) {
      return null;
    }
    return { width: joinWorld.width, height: joinWorld.height };
  }

  private extractWorldDimensionsFromSnapshot(
    worldSnapshot: WorldStateSnapshot | null,
  ): WorldConfigurationSnapshot | null {
    if (!worldSnapshot?.keyframe?.metadata) {
      return null;
    }
    const metadata = worldSnapshot.keyframe.metadata;
    if (!metadata || typeof metadata !== "object") {
      return null;
    }
    const world = (metadata as Record<string, unknown>)["world"];
    if (!world || typeof world !== "object") {
      return null;
    }
    const record = world as Record<string, unknown>;
    const width = typeof record.width === "number" && Number.isFinite(record.width) ? record.width : null;
    const height = typeof record.height === "number" && Number.isFinite(record.height) ? record.height : null;
    if (width === null || height === null) {
      return null;
    }
    return { width, height };
  }

  private createWorldBackgroundGeometry(dimensions: WorldConfigurationSnapshot): StaticGeometry | null {
    const { width, height } = dimensions;
    if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) {
      return null;
    }
    const vertices: [number, number][] = [
      [0, 0],
      [width, 0],
      [width, height],
      [0, height],
    ];
    return {
      id: "world/background",
      layer: WORLD_BACKGROUND_LAYER,
      vertices,
      style: {
        kind: "world-background",
        width,
        height,
        origin: [0, 0],
        fill: "#0f1218",
        stroke: "#06080b",
      },
    };
  }

  private createWorldGridGeometry(dimensions: WorldConfigurationSnapshot): StaticGeometry | null {
    const { width, height } = dimensions;
    if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) {
      return null;
    }
    const vertices: [number, number][] = [
      [0, 0],
      [width, 0],
      [width, height],
      [0, height],
    ];
    return {
      id: "world/grid",
      layer: WORLD_GRID_LAYER,
      vertices,
      style: {
        kind: "world-grid",
        columns: Math.max(1, Math.floor(width)),
        rows: Math.max(1, Math.floor(height)),
        spacing: 1,
        stroke: "rgba(255, 255, 255, 0.06)",
      },
    };
  }

  private createGeometryForEntity(entity: WorldEntityState): readonly StaticGeometry[] {
    const position = this.extractEntityPosition(entity);
    if (!position) {
      return [];
    }

    switch (entity.type) {
      case "player": {
        const playerGeometry = this.createPlayerGeometry(entity, position);
        return playerGeometry ? [playerGeometry] : [];
      }
      case "npc": {
        const npcGeometry = this.createNPCGeometry(entity, position);
        return npcGeometry ? [npcGeometry] : [];
      }
      case "obstacle": {
        const obstacleGeometry = this.createObstacleGeometry(entity, position);
        return obstacleGeometry ? [obstacleGeometry] : [];
      }
      case "groundItem": {
        const itemGeometry = this.createGroundItemGeometry(entity, position);
        return itemGeometry ? [itemGeometry] : [];
      }
      default:
        return [];
    }
  }

  private createPlayerGeometry(
    entity: WorldEntityState,
    position: readonly [number, number],
  ): StaticGeometry | null {
    const radius = 0.4;
    const vertices = this.createCircleVertices(position, radius, 14);
    if (vertices.length === 0) {
      return null;
    }
    const style: Record<string, unknown> = {
      kind: "player",
      playerId: entity.id,
      anchor: "center",
      radius,
      fill: "rgba(80, 200, 255, 0.85)",
      stroke: "rgba(24, 136, 212, 1)",
    };
    if (typeof (entity as Record<string, unknown>).facing === "string") {
      style.facing = (entity as Record<string, unknown>).facing;
    }
    if (typeof (entity as Record<string, unknown>).health === "number") {
      style.health = (entity as Record<string, unknown>).health;
    }
    if (typeof (entity as Record<string, unknown>).maxHealth === "number") {
      style.maxHealth = (entity as Record<string, unknown>).maxHealth;
    }
    if ((entity as Record<string, unknown>).intent && typeof (entity as Record<string, unknown>).intent === "object") {
      style.intent = (entity as Record<string, unknown>).intent;
    }
    return {
      id: `world/player/${entity.id}`,
      layer: WORLD_PLAYER_LAYER,
      vertices,
      style,
    };
  }

  private createNPCGeometry(
    entity: WorldEntityState,
    position: readonly [number, number],
  ): StaticGeometry | null {
    const radius = 0.4;
    const vertices = this.createCircleVertices(position, radius, 14);
    if (vertices.length === 0) {
      return null;
    }
    const style: Record<string, unknown> = {
      kind: "npc",
      npcId: entity.id,
      anchor: "center",
      radius,
      fill: "rgba(255, 168, 92, 0.85)",
      stroke: "rgba(204, 108, 28, 1)",
    };
    if (typeof (entity as Record<string, unknown>).npcType === "string") {
      style.npcType = (entity as Record<string, unknown>).npcType;
    }
    if (typeof (entity as Record<string, unknown>).facing === "string") {
      style.facing = (entity as Record<string, unknown>).facing;
    }
    if (typeof (entity as Record<string, unknown>).health === "number") {
      style.health = (entity as Record<string, unknown>).health;
    }
    if (typeof (entity as Record<string, unknown>).maxHealth === "number") {
      style.maxHealth = (entity as Record<string, unknown>).maxHealth;
    }
    return {
      id: `world/npc/${entity.id}`,
      layer: WORLD_NPC_LAYER,
      vertices,
      style,
    };
  }

  private createObstacleGeometry(
    entity: WorldEntityState,
    position: readonly [number, number],
  ): StaticGeometry | null {
    const width = typeof (entity as Record<string, unknown>).width === "number"
      ? (entity as Record<string, unknown>).width
      : null;
    const height = typeof (entity as Record<string, unknown>).height === "number"
      ? (entity as Record<string, unknown>).height
      : null;
    if (width === null || height === null || width <= 0 || height <= 0) {
      return null;
    }
    const vertices = this.createRectangleVertices(position, width, height);
    if (vertices.length === 0) {
      return null;
    }
    const style: Record<string, unknown> = {
      kind: "obstacle",
      obstacleId: entity.id,
      width,
      height,
      anchor: "center",
      fill: "rgba(92, 104, 120, 0.9)",
      stroke: "rgba(54, 62, 74, 1)",
    };
    if (typeof (entity as Record<string, unknown>).obstacleType === "string") {
      style.obstacleType = (entity as Record<string, unknown>).obstacleType;
    }
    return {
      id: `world/obstacle/${entity.id}`,
      layer: WORLD_OBSTACLE_LAYER,
      vertices,
      style,
    };
  }

  private createGroundItemGeometry(
    entity: WorldEntityState,
    position: readonly [number, number],
  ): StaticGeometry | null {
    const radius = 0.3;
    const vertices = this.createDiamondVertices(position, radius);
    if (vertices.length === 0) {
      return null;
    }
    const style: Record<string, unknown> = {
      kind: "ground-item",
      itemId: entity.id,
      anchor: "center",
      radius,
      fill: "rgba(192, 255, 160, 0.9)",
      stroke: "rgba(126, 190, 94, 1)",
    };
    if (typeof (entity as Record<string, unknown>).itemType === "string") {
      style.itemType = (entity as Record<string, unknown>).itemType;
    }
    if (typeof (entity as Record<string, unknown>).qty === "number") {
      style.quantity = (entity as Record<string, unknown>).qty;
    }
    if (typeof (entity as Record<string, unknown>).fungibilityKey === "string") {
      style.fungibilityKey = (entity as Record<string, unknown>).fungibilityKey;
    }
    return {
      id: `world/ground-item/${entity.id}`,
      layer: WORLD_GROUND_ITEM_LAYER,
      vertices,
      style,
    };
  }

  private extractEntityPosition(entity: WorldEntityState): readonly [number, number] | null {
    if (!Array.isArray(entity.position) || entity.position.length < 2) {
      return null;
    }
    const [x, y] = entity.position;
    if (typeof x !== "number" || !Number.isFinite(x) || typeof y !== "number" || !Number.isFinite(y)) {
      return null;
    }
    return [x, y];
  }

  private createCircleVertices(
    center: readonly [number, number],
    radius: number,
    steps: number,
  ): [number, number][] {
    if (!Number.isFinite(radius) || radius <= 0 || !Number.isFinite(center[0]) || !Number.isFinite(center[1])) {
      return [];
    }
    const clampedSteps = Math.max(8, Math.floor(steps));
    const vertices: [number, number][] = [];
    for (let index = 0; index < clampedSteps; index += 1) {
      const theta = (index / clampedSteps) * Math.PI * 2;
      const x = center[0] + Math.cos(theta) * radius;
      const y = center[1] + Math.sin(theta) * radius;
      vertices.push([x, y]);
    }
    return vertices;
  }

  private createRectangleVertices(
    center: readonly [number, number],
    width: number,
    height: number,
  ): [number, number][] {
    if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) {
      return [];
    }
    const [cx, cy] = center;
    const halfWidth = width / 2;
    const halfHeight = height / 2;
    return [
      [cx - halfWidth, cy - halfHeight],
      [cx + halfWidth, cy - halfHeight],
      [cx + halfWidth, cy + halfHeight],
      [cx - halfWidth, cy + halfHeight],
    ];
  }

  private createDiamondVertices(
    center: readonly [number, number],
    radius: number,
  ): [number, number][] {
    if (!Number.isFinite(radius) || radius <= 0) {
      return [];
    }
    const [cx, cy] = center;
    return [
      [cx, cy - radius],
      [cx + radius, cy],
      [cx, cy + radius],
      [cx - radius, cy],
    ];
  }

  private translatePlayerEntity(player: PlayerSnapshot): WorldEntityState | null {
    const id = typeof player.id === "string" && player.id.length > 0 ? player.id : null;
    const x = typeof player.x === "number" && Number.isFinite(player.x) ? player.x : null;
    const y = typeof player.y === "number" && Number.isFinite(player.y) ? player.y : null;
    if (!id || x === null || y === null) {
      return null;
    }

    const entity: WorldEntityState = {
      id,
      type: "player",
      position: [x, y],
    };

    if (typeof player.facing === "string" && player.facing.length > 0) {
      entity.facing = player.facing;
    }
    if (typeof player.health === "number" && Number.isFinite(player.health)) {
      entity.health = player.health;
    }
    if (typeof player.maxHealth === "number" && Number.isFinite(player.maxHealth)) {
      entity.maxHealth = player.maxHealth;
    }
    if (player.inventory && typeof player.inventory === "object") {
      entity.inventory = player.inventory;
    }
    if (player.equipment && typeof player.equipment === "object") {
      entity.equipment = player.equipment;
    }

    return entity;
  }

  private translateNPCEntity(npc: NPCSnapshot): WorldEntityState | null {
    const id = typeof npc.id === "string" && npc.id.length > 0 ? npc.id : null;
    const x = typeof npc.x === "number" && Number.isFinite(npc.x) ? npc.x : null;
    const y = typeof npc.y === "number" && Number.isFinite(npc.y) ? npc.y : null;
    if (!id || x === null || y === null) {
      return null;
    }

    const entity: WorldEntityState = {
      id,
      type: "npc",
      position: [x, y],
    };

    if (typeof npc.facing === "string" && npc.facing.length > 0) {
      entity.facing = npc.facing;
    }
    if (typeof npc.health === "number" && Number.isFinite(npc.health)) {
      entity.health = npc.health;
    }
    if (typeof npc.maxHealth === "number" && Number.isFinite(npc.maxHealth)) {
      entity.maxHealth = npc.maxHealth;
    }
    if (npc.inventory && typeof npc.inventory === "object") {
      entity.inventory = npc.inventory;
    }
    if (npc.equipment && typeof npc.equipment === "object") {
      entity.equipment = npc.equipment;
    }
    if (typeof npc.type === "string" && npc.type.length > 0) {
      entity.npcType = npc.type;
    }
    if (typeof npc.aiControlled === "boolean") {
      entity.aiControlled = npc.aiControlled;
    }
    if (typeof npc.experienceReward === "number" && Number.isFinite(npc.experienceReward)) {
      entity.experienceReward = npc.experienceReward;
    }

    return entity;
  }

  private translateObstacleEntity(obstacle: ObstacleSnapshot): WorldEntityState | null {
    const id = typeof obstacle.id === "string" && obstacle.id.length > 0 ? obstacle.id : null;
    const x = typeof obstacle.x === "number" && Number.isFinite(obstacle.x) ? obstacle.x : null;
    const y = typeof obstacle.y === "number" && Number.isFinite(obstacle.y) ? obstacle.y : null;
    const width = typeof obstacle.width === "number" && Number.isFinite(obstacle.width) ? obstacle.width : null;
    const height = typeof obstacle.height === "number" && Number.isFinite(obstacle.height) ? obstacle.height : null;
    if (!id || x === null || y === null || width === null || height === null) {
      return null;
    }

    const entity: WorldEntityState = {
      id,
      type: "obstacle",
      position: [x, y],
      width,
      height,
    };

    if (typeof obstacle.type === "string" && obstacle.type.length > 0) {
      entity.obstacleType = obstacle.type;
    }

    return entity;
  }

  private translateGroundItemEntity(item: GroundItemSnapshot): WorldEntityState | null {
    const id = typeof item.id === "string" && item.id.length > 0 ? item.id : null;
    const x = typeof item.x === "number" && Number.isFinite(item.x) ? item.x : null;
    const y = typeof item.y === "number" && Number.isFinite(item.y) ? item.y : null;
    const qty = typeof item.qty === "number" && Number.isFinite(item.qty) ? Math.floor(item.qty) : null;
    if (!id || x === null || y === null || qty === null) {
      return null;
    }

    const entity: WorldEntityState = {
      id,
      type: "groundItem",
      position: [x, y],
      qty,
      itemType: item.type,
    };

    if (typeof item.fungibility_key === "string" && item.fungibility_key.length > 0) {
      entity.fungibilityKey = item.fungibility_key;
    }

    return entity;
  }

  private createWorldPatchBatch(
    patches: readonly NetworkPatch[],
    options: { readonly keyframeId: string; readonly timestamp: number },
  ): WorldPatchBatch | null {
    if (!Array.isArray(patches) || patches.length === 0) {
      return null;
    }

    const operations: WorldPatchOperation[] = [];
    for (const patch of patches) {
      const translated = this.translateNetworkPatch(patch);
      if (translated.length > 0) {
        operations.push(...translated);
      }
    }

    if (operations.length === 0) {
      return null;
    }

    return {
      keyframeId: options.keyframeId,
      timestamp: options.timestamp,
      operations,
    };
  }

  private translateNetworkPatch(patch: NetworkPatch): readonly WorldPatchOperation[] {
    if (!patch || typeof patch !== "object") {
      return [];
    }

    const { kind, entityId, payload } = patch;
    if (typeof entityId !== "string" || entityId.length === 0) {
      return [];
    }

    switch (kind) {
      case "player_pos":
      case "npc_pos":
      case "ground_item_pos":
        return this.translatePositionPatch(entityId, payload);
      case "player_facing":
      case "npc_facing":
        return this.translateFacingPatch(entityId, payload);
      case "player_intent":
        return this.translateIntentPatch(entityId, payload);
      case "player_health":
      case "npc_health":
        return this.translateHealthPatch(entityId, payload);
      case "player_inventory":
      case "npc_inventory":
        return this.translateObjectPatch(entityId, "inventory", payload);
      case "player_equipment":
      case "npc_equipment":
        return this.translateObjectPatch(entityId, "equipment", payload);
      case "ground_item_qty":
        return this.translateQuantityPatch(entityId, payload);
      case "player_removed":
        return [{ entityId, path: [], value: null }];
      default:
        return [];
    }
  }

  private translatePositionPatch(entityId: string, payload: unknown): readonly WorldPatchOperation[] {
    if (!payload || typeof payload !== "object") {
      return [];
    }
    const record = payload as Record<string, unknown>;
    const x = typeof record.x === "number" && Number.isFinite(record.x) ? record.x : null;
    const y = typeof record.y === "number" && Number.isFinite(record.y) ? record.y : null;
    if (x === null || y === null) {
      return [];
    }
    return [{ entityId, path: ["position"], value: [x, y] }];
  }

  private translateFacingPatch(entityId: string, payload: unknown): readonly WorldPatchOperation[] {
    if (!payload || typeof payload !== "object") {
      return [];
    }
    const record = payload as Record<string, unknown>;
    const facing = typeof record.facing === "string" && record.facing.length > 0 ? record.facing : null;
    if (!facing) {
      return [];
    }
    return [{ entityId, path: ["facing"], value: facing }];
  }

  private translateIntentPatch(entityId: string, payload: unknown): readonly WorldPatchOperation[] {
    if (!payload || typeof payload !== "object") {
      return [];
    }
    const record = payload as Record<string, unknown>;
    const dx = typeof record.dx === "number" && Number.isFinite(record.dx) ? record.dx : null;
    const dy = typeof record.dy === "number" && Number.isFinite(record.dy) ? record.dy : null;
    if (dx === null || dy === null) {
      return [];
    }
    return [{ entityId, path: ["intent"], value: { dx, dy } }];
  }

  private translateHealthPatch(entityId: string, payload: unknown): readonly WorldPatchOperation[] {
    if (!payload || typeof payload !== "object") {
      return [];
    }
    const record = payload as Record<string, unknown>;
    const operations: WorldPatchOperation[] = [];
    if (typeof record.health === "number" && Number.isFinite(record.health)) {
      operations.push({ entityId, path: ["health"], value: record.health });
    }
    if (typeof record.maxHealth === "number" && Number.isFinite(record.maxHealth)) {
      operations.push({ entityId, path: ["maxHealth"], value: record.maxHealth });
    }
    return operations;
  }

  private translateObjectPatch(
    entityId: string,
    key: string,
    payload: unknown,
  ): readonly WorldPatchOperation[] {
    if (!payload || typeof payload !== "object") {
      return [];
    }
    return [{ entityId, path: [key], value: payload }];
  }

  private translateQuantityPatch(entityId: string, payload: unknown): readonly WorldPatchOperation[] {
    if (!payload || typeof payload !== "object") {
      return [];
    }
    const record = payload as Record<string, unknown>;
    const qty = typeof record.qty === "number" && Number.isFinite(record.qty) ? Math.floor(record.qty) : null;
    if (qty === null) {
      return [];
    }
    return [{ entityId, path: ["qty"], value: qty }];
  }

  private updateWorldSnapshot(frameTime?: number, immediate = true): void {
    this.latestWorldSnapshot = this.worldState.snapshot();
    this.worldRenderVersion += 1;
    if (immediate) {
      this.renderLifecycleView(frameTime);
    }
  }

  private emitDebugLog(message: string): void {
    this.lifecycleHandlers?.onLog?.(message);
  }

  private logWorldSnapshot(
    source: string,
    snapshot: WorldSnapshotPayload,
    options: {
      readonly world?: WorldConfigurationSnapshot | null;
      readonly tick?: number | null;
      readonly sequence?: number | null;
      readonly keyframeSequence?: number | null;
      readonly seed?: string | null;
    } = {},
  ): void {
    const world = options.world ?? null;
    const dimensions = world ? `${world.width}×${world.height}` : "unknown";
    const summaryParts = [
      `size=${dimensions}`,
      `players=${snapshot.players.length}`,
      `npcs=${snapshot.npcs.length}`,
      `obstacles=${snapshot.obstacles.length}`,
      `groundItems=${snapshot.groundItems.length}`,
    ];
    const contextParts: string[] = [];
    if (options.seed) {
      contextParts.push(`seed ${options.seed}`);
    }
    if (options.tick !== undefined && options.tick !== null) {
      contextParts.push(`tick ${options.tick}`);
    }
    if (options.sequence !== undefined && options.sequence !== null) {
      contextParts.push(`seq ${options.sequence}`);
    }
    if (options.keyframeSequence !== undefined && options.keyframeSequence !== null) {
      contextParts.push(`keyframe ${options.keyframeSequence}`);
    }
    const suffix = contextParts.length > 0 ? ` (${contextParts.join(", ")})` : "";
    this.emitDebugLog(`World snapshot [${source}] ${summaryParts.join(" · ")}${suffix}`);
  }

  private logPatchBatch(
    source: string,
    patchCount: number,
    operationCount: number,
    options: {
      readonly tick?: number | null;
      readonly sequence?: number | null;
      readonly keyframeSequence?: number | null;
    } = {},
  ): void {
    if (operationCount <= 0) {
      return;
    }
    const summaryParts = [`ops=${operationCount}`, `patches=${patchCount}`];
    const contextParts: string[] = [];
    if (options.tick !== undefined && options.tick !== null) {
      contextParts.push(`tick ${options.tick}`);
    }
    if (options.sequence !== undefined && options.sequence !== null) {
      contextParts.push(`seq ${options.sequence}`);
    }
    if (options.keyframeSequence !== undefined && options.keyframeSequence !== null) {
      contextParts.push(`keyframe ${options.keyframeSequence}`);
    }
    const suffix = contextParts.length > 0 ? ` (${contextParts.join(", ")})` : "";
    this.emitDebugLog(`Patch batch [${source}] ${summaryParts.join(" · ")}${suffix}`);
  }

  private createWorldMetadata(options: {
    readonly source: string;
    readonly world?: WorldConfigurationSnapshot | null;
    readonly tick?: number | null;
    readonly sequence?: number | null;
    readonly keyframeSequence?: number | null;
    readonly additional?: Record<string, unknown>;
  }): Record<string, unknown> {
    const metadata: Record<string, unknown> = { source: options.source };
    if (options.world) {
      metadata.world = {
        width: options.world.width,
        height: options.world.height,
      };
    }
    if (options.tick !== undefined && options.tick !== null) {
      metadata.tick = options.tick;
    }
    if (options.sequence !== undefined && options.sequence !== null) {
      metadata.sequence = options.sequence;
    }
    if (options.keyframeSequence !== undefined && options.keyframeSequence !== null) {
      metadata.keyframeSequence = options.keyframeSequence;
    }
    if (options.additional) {
      for (const [key, value] of Object.entries(options.additional)) {
        metadata[key] = value;
      }
    }
    return metadata;
  }

  private extractWorldSnapshotFromStatePayload(
    payload: Record<string, unknown>,
  ): WorldSnapshotPayload | null {
    return this.extractWorldSnapshotFromPayload(payload, "state");
  }

  private extractWorldSnapshotFromKeyframePayload(
    payload: Record<string, unknown>,
  ): WorldSnapshotPayload | null {
    return this.extractWorldSnapshotFromPayload(payload, "keyframe");
  }

  private extractWorldSnapshotFromPayload(
    payload: Record<string, unknown>,
    context: string,
  ): WorldSnapshotPayload | null {
    const hasPlayers = this.payloadHasField(payload, "players");
    const hasNPCs = this.payloadHasField(payload, "npcs");
    const hasObstacles = this.payloadHasField(payload, "obstacles");
    const hasGroundItems = this.payloadHasField(payload, "groundItems");

    if (!hasPlayers && !hasNPCs && !hasObstacles && !hasGroundItems) {
      return null;
    }

    const players = hasPlayers
      ? normalizePlayerSnapshots(payload["players"], `${context}.players`)
      : [];
    const npcs = hasNPCs
      ? normalizeNPCSnapshots(payload["npcs"], `${context}.npcs`)
      : [];
    const obstacles = hasObstacles
      ? normalizeObstacleSnapshots(payload["obstacles"], `${context}.obstacles`)
      : [];
    const groundItems = hasGroundItems
      ? normalizeGroundItemSnapshots(payload["groundItems"], `${context}.groundItems`)
      : [];

    return { players, npcs, obstacles, groundItems };
  }

  private payloadHasField(payload: Record<string, unknown>, field: string): boolean {
    return Object.prototype.hasOwnProperty.call(payload, field);
  }

  private extractWorldDimensions(config: unknown): WorldConfigurationSnapshot | null {
    if (!config || typeof config !== "object") {
      return null;
    }
    const record = config as Record<string, unknown>;
    const width = typeof record.width === "number" && Number.isFinite(record.width) ? record.width : null;
    const height = typeof record.height === "number" && Number.isFinite(record.height) ? record.height : null;
    if (width === null || height === null) {
      return null;
    }
    return { width, height };
  }

  private extractTick(value: unknown): number | null {
    if (typeof value !== "number" || !Number.isFinite(value) || value < 0) {
      return null;
    }
    const normalized = Math.floor(value);
    return normalized >= 0 ? normalized : null;
  }

  private resolveWorldKeyframeId(
    source: string,
    keyframeSequence: number | null,
    sequence: number | null,
    tick: number | null,
  ): string {
    if (keyframeSequence !== null) {
      return `${source}-keyframe-${keyframeSequence}`;
    }
    if (sequence !== null) {
      return `${source}-sequence-${sequence}`;
    }
    if (tick !== null) {
      return `${source}-tick-${tick}`;
    }
    return `${source}-${Date.now()}`;
  }

  private resolvePatchKeyframeId(
    keyframeSequence: number | null,
    sequence: number | null,
    fallback: string | null,
    source: string,
  ): string {
    if (keyframeSequence !== null) {
      return `${source}-keyframe-${keyframeSequence}`;
    }
    if (sequence !== null) {
      return `${source}-sequence-${sequence}`;
    }
    if (fallback) {
      return fallback;
    }
    return `${source}-patch-${Date.now()}`;
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
    const resolved = this.renderer.configuration.layers.find((layer) => candidateIds.includes(layer.id));
    if (!resolved) {
      throw new Error(`Renderer configuration missing layer for ${delivery} delivery effects.`);
    }
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

  private recordAcknowledgedTick(tick: number | null): void {
    if (tick === null) {
      return;
    }
    if (typeof tick !== "number" || !Number.isFinite(tick) || tick < 0) {
      return;
    }
    const normalized = Math.floor(tick);
    if (normalized < 0) {
      return;
    }
    const previous = this.latestAcknowledgedTick;
    if (previous !== null && normalized <= previous) {
      return;
    }
    this.latestAcknowledgedTick = normalized;
  }

  private handleIntentDispatched(_intent: PlayerIntent): void {
    // Reserved for future intent instrumentation.
  }

  private handlePathCommand(state: PathCommandState): void {
    this.applyPathCommandState(state, { notifyHooks: false });
  }

  private handleCommandRejection(rejection: CommandRejectionDetails): void {
    if (rejection.retry) {
      return;
    }
    const snapshot = cloneCommandRejectionDetails(rejection);
    this.lastCommandRejection = snapshot;
    this.inputDispatcherHooks?.onCommandRejectionChanged?.(cloneCommandRejectionDetails(snapshot));
  }

  private handleCommandAcknowledgement(ack: CommandAcknowledgementDetails): void {
    this.clearCommandRejection(ack.kind);
  }

  private handleCommandReset(): void {
    this.clearCommandRejection();
  }

  private clearCommandRejection(kind?: CommandKind): void {
    if (!this.lastCommandRejection) {
      return;
    }
    if (kind && this.lastCommandRejection.kind !== kind) {
      return;
    }
    this.lastCommandRejection = null;
    this.inputDispatcherHooks?.onCommandRejectionChanged?.(null);
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
