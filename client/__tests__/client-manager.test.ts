import { beforeEach, describe, expect, test, vi } from "vitest";

import { GameClientOrchestrator, type ClientManagerConfiguration } from "../client-manager";
import { setEffectCatalog } from "../effect-catalog";
import {
  effectCatalog as generatedEffectCatalog,
  type EffectCatalogEntry,
} from "../generated/effect-contracts";
import type {
  JoinResponse,
  NetworkClient,
  NetworkClientConfiguration,
  NetworkEventHandlers,
  NetworkMessageEnvelope,
} from "../network";
import type {
  RenderBatch,
  RenderContextProvider,
  RenderDimensions,
  RenderLayer,
  Renderer,
  RendererConfiguration,
} from "../render";
import {
  type ContractLifecycleSpawnEvent,
  type ContractLifecycleBatch,
} from "../effect-lifecycle-store";
import { InMemoryWorldStateStore } from "../world-state";

class TestNetworkClient implements NetworkClient {
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

class TestRenderer implements Renderer {
  public readonly configuration: RendererConfiguration;
  public readonly batches: RenderBatch[] = [];

  constructor(configuration?: Partial<RendererConfiguration>) {
    const dimensions: RenderDimensions = configuration?.dimensions ?? {
      width: 800,
      height: 600,
    };
    const layers: RenderLayer[] = configuration?.layers
      ? configuration.layers.map((layer) => ({ ...layer }))
      : [
          { id: "effect-area", zIndex: 1 },
          { id: "effect-visual", zIndex: 2 },
        ];

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

const createJoinResponse = (catalog: Record<string, EffectCatalogEntry>): JoinResponse => ({
  id: "player-test",
  seed: "seed",
  protocolVersion: 1,
  effectCatalog: catalog,
});

const orchestratorConfig: ClientManagerConfiguration = {
  autoConnect: true,
  reconcileIntervalMs: 0,
};

describe("GameClientOrchestrator", () => {
  beforeEach(() => {
    setEffectCatalog(null);
  });

  test("hydrates lifecycle store and renders batches from catalog metadata", async () => {
    const joinResponse = createJoinResponse(generatedEffectCatalog);
    const network = new TestNetworkClient(
      {
        joinUrl: "/join",
        websocketUrl: "ws://localhost",
        heartbeatIntervalMs: 1000,
        protocolVersion: 1,
      },
      joinResponse,
    );
    const renderer = new TestRenderer();
    const worldState = new InMemoryWorldStateStore();
    const orchestrator = new GameClientOrchestrator(orchestratorConfig, {
      network,
      renderer,
      worldState,
    });

    const ready = vi.fn();
    await orchestrator.boot({ onReady: ready });
    expect(ready).toHaveBeenCalledTimes(1);

    const attackEntry = generatedEffectCatalog.attack;
    const spawn: ContractLifecycleSpawnEvent = {
      seq: 1,
      tick: 10,
      instance: {
        id: "effect-attack",
        entryId: "attack",
        definitionId: attackEntry.contractId,
        definition: attackEntry.definition,
        startTick: 10,
        deliveryState: {
          geometry: {
            shape: "rect",
            width: 40,
            height: 24,
          },
          motion: {
            positionX: 128,
            positionY: 256,
            velocityX: 0,
            velocityY: 0,
          },
        },
        behaviorState: {
          ticksRemaining: 6,
        },
        params: attackEntry.blocks.parameters as Readonly<Record<string, number>>,
        replication: attackEntry.definition.client,
        end: attackEntry.definition.end,
      },
    };

    const batch: ContractLifecycleBatch = {
      spawns: [spawn],
      cursors: { "effect-attack": 1 },
    };

    network.emit({
      type: "state",
      payload: {
        effect_spawned: batch.spawns,
        effect_seq_cursors: batch.cursors,
        t: 10,
      },
      receivedAt: 500,
    });

    expect(renderer.batches.length).toBeGreaterThanOrEqual(2);
    const rendered = renderer.batches.at(-1);
    expect(rendered).toBeDefined();
    const finalBatch = rendered!;
    expect(finalBatch.animations.length).toBeGreaterThan(0);
    const animation = finalBatch.animations[0];
    expect(animation.metadata).toMatchObject({
      entryId: "attack",
      managedByClient: attackEntry.managedByClient,
    });
    expect(animation.metadata.catalog).toMatchObject({
      contractId: attackEntry.contractId,
    });
    expect(finalBatch.staticGeometry[0]?.layer.id).toBe("effect-area");
    expect(finalBatch.staticGeometry[0]?.vertices.length).toBe(4);

    network.emit({
      type: "state",
      payload: {
        resync: true,
      },
      receivedAt: 750,
    });

    const cleared = renderer.batches.at(-1);
    expect(cleared).toBeDefined();
    expect(cleared!.animations.length).toBe(0);
  });
});
