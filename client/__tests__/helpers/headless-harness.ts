import { GameClientOrchestrator, type ClientManagerConfiguration } from "../../client-manager";
import type { EffectCatalogEntry } from "../../generated/effect-contracts";
import type {
  JoinResponse,
  NetworkClient,
  NetworkClientConfiguration,
  NetworkEventHandlers,
  NetworkMessageEnvelope,
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
  effectCatalog: catalog,
  ...overrides,
});

export class HeadlessNetworkClient implements NetworkClient {
  private handlers: NetworkEventHandlers | null = null;

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

  send(_data: unknown): void {}

  emit(message: NetworkMessageEnvelope): void {
    this.handlers?.onMessage?.(message);
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

  return { joinResponse, network, renderer, orchestrator, worldState };
};
