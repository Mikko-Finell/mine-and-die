import {
  GameClientOrchestrator,
  type ClientManagerConfiguration,
} from "../../client-manager";
import { normalizeEffectCatalog } from "../../effect-catalog";
import type { EffectCatalogEntry } from "../../generated/effect-contracts";
import type {
  ContractLifecycleBatch,
  ContractLifecycleEndEvent,
  ContractLifecycleSpawnEvent,
  ContractLifecycleUpdateEvent,
} from "../../effect-lifecycle-store";
import type {
  JoinResponse,
  NetworkClient,
  NetworkClientConfiguration,
  NetworkEventHandlers,
  NetworkMessageEnvelope,
  WorldConfigurationSnapshot,
} from "../../network";
import type {
  RenderBatch,
  RenderContextProvider,
  RenderDimensions,
  RenderLayer,
  Renderer,
  RendererConfiguration,
} from "../../render";
import { InMemoryWorldStateStore } from "../../world-state";

export const defaultOrchestratorConfiguration: ClientManagerConfiguration = {
  autoConnect: true,
  reconcileIntervalMs: 0,
  keyframeRetryDelayMs: 1000,
};

export const defaultNetworkConfiguration: NetworkClientConfiguration = {
  joinUrl: "/join",
  websocketUrl: "ws://localhost",
  heartbeatIntervalMs: 1000,
  protocolVersion: 1,
};

export const defaultRendererConfiguration: RendererConfiguration = {
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

export const createJoinResponse = (
  catalog: Record<string, EffectCatalogEntry>,
  overrides: Partial<Omit<JoinResponse, "effectCatalog">> = {},
): JoinResponse => ({
  id: "player",
  seed: "seed",
  protocolVersion: 1,
  effectCatalog: normalizeEffectCatalog(catalog),
  world: (overrides.world as WorldConfigurationSnapshot | undefined) ?? {
    width: defaultRendererConfiguration.dimensions.width,
    height: defaultRendererConfiguration.dimensions.height,
  },
  ...overrides,
});

export class HeadlessNetworkClient implements NetworkClient {
  private handlers: NetworkEventHandlers | null = null;
  public readonly sentMessages: unknown[] = [];

  constructor(
    public readonly configuration: NetworkClientConfiguration,
    private readonly joinResponse: JoinResponse,
  ) {}

  async join(): Promise<JoinResponse> {
    return this.joinResponse;
  }

  async connect(handlers: NetworkEventHandlers): Promise<void> {
    this.handlers = handlers;
    handlers.onJoin?.(this.joinResponse);
  }

  async disconnect(): Promise<void> {
    this.handlers = null;
  }

  send(data: unknown): void {
    this.sentMessages.push(data);
  }

  emit(message: NetworkMessageEnvelope): void {
    this.handlers?.onMessage?.(message);
  }

  simulateDisconnect(code?: number, reason?: string): void {
    const handlers = this.handlers;
    this.handlers = null;
    handlers?.onDisconnect?.(code, reason);
  }
}

export class HeadlessRenderer implements Renderer {
  public readonly configuration: RendererConfiguration;
  public readonly batches: RenderBatch[] = [];

  constructor(configuration?: Partial<RendererConfiguration>) {
    const dimensions: RenderDimensions = configuration?.dimensions ?? {
      ...defaultRendererConfiguration.dimensions,
    };
    const layers: RenderLayer[] = configuration?.layers
      ? configuration.layers.map((layer) => ({ ...layer }))
      : defaultRendererConfiguration.layers.map((layer) => ({ ...layer }));

    this.configuration = {
      dimensions,
      layers,
    };
  }

  mount(_provider: RenderContextProvider): void {}

  unmount(): void {}

  renderBatch(batch: RenderBatch): void {
    this.batches.push(batch);
  }

  resize(_dimensions: RenderDimensions): void {}

  reset(): void {}
}

export interface HeadlessHarnessOptions {
  catalog: Record<string, EffectCatalogEntry>;
  joinResponseOverrides?: Partial<Omit<JoinResponse, "effectCatalog">>;
  networkConfiguration?: Partial<NetworkClientConfiguration>;
  orchestratorConfiguration?: Partial<ClientManagerConfiguration>;
  rendererConfiguration?: Partial<RendererConfiguration>;
}

export interface HeadlessHarness {
  joinResponse: JoinResponse;
  network: HeadlessNetworkClient;
  renderer: HeadlessRenderer;
  orchestrator: GameClientOrchestrator;
  worldState: InMemoryWorldStateStore;
  emitLifecycleState: (options: HeadlessLifecycleStateOptions) => void;
}

export interface HeadlessLifecycleStateOptions {
  readonly spawns?: readonly ContractLifecycleSpawnEvent[];
  readonly updates?: readonly ContractLifecycleUpdateEvent[];
  readonly ends?: readonly ContractLifecycleEndEvent[];
  readonly cursors?: ContractLifecycleBatch["cursors"];
  readonly tick?: number | null;
  readonly sequence?: number | null;
  readonly keyframeSequence?: number | null;
  readonly resync?: boolean;
  readonly config?: Record<string, unknown>;
  readonly payload?: Record<string, unknown>;
  readonly receivedAt?: number;
}

export const createHeadlessHarness = ({
  catalog,
  joinResponseOverrides,
  networkConfiguration,
  orchestratorConfiguration,
  rendererConfiguration,
}: HeadlessHarnessOptions): HeadlessHarness => {
  const joinResponse = createJoinResponse(catalog, joinResponseOverrides);
  const network = new HeadlessNetworkClient(
    {
      ...defaultNetworkConfiguration,
      ...networkConfiguration,
    },
    joinResponse,
  );
  const renderer = new HeadlessRenderer(rendererConfiguration);
  const worldState = new InMemoryWorldStateStore();
  const orchestrator = new GameClientOrchestrator(
    {
      ...defaultOrchestratorConfiguration,
      ...orchestratorConfiguration,
    },
    {
      network,
      renderer,
      worldState,
    },
  );

  const emitLifecycleState = ({
    spawns,
    updates,
    ends,
    cursors,
    tick,
    sequence,
    keyframeSequence,
    resync,
    config,
    payload,
    receivedAt,
  }: HeadlessLifecycleStateOptions): void => {
    const messagePayload: Record<string, unknown> = payload ? { ...payload } : {};

    if (Array.isArray(spawns) && spawns.length > 0) {
      messagePayload.effect_spawned = spawns;
    }
    if (Array.isArray(updates) && updates.length > 0) {
      messagePayload.effect_update = updates;
    }
    if (Array.isArray(ends) && ends.length > 0) {
      messagePayload.effect_ended = ends;
    }
    if (cursors) {
      const hasEntries =
        cursors instanceof Map ? cursors.size > 0 : Object.keys(cursors as Record<string, number>).length > 0;
      if (hasEntries) {
        messagePayload.effect_seq_cursors = cursors;
      }
    }
    if (typeof tick === "number") {
      messagePayload.t = tick;
    }
    if (typeof sequence === "number") {
      messagePayload.sequence = sequence;
    }
    if (typeof keyframeSequence === "number") {
      messagePayload.keyframeSeq = keyframeSequence;
    }
    if (resync) {
      messagePayload.resync = true;
    }
    if (config) {
      messagePayload.config = config;
    }

    network.emit({
      type: "state",
      payload: messagePayload,
      receivedAt: receivedAt ?? Date.now(),
    });
  };

  return { joinResponse, network, renderer, orchestrator, worldState, emitLifecycleState };
};
