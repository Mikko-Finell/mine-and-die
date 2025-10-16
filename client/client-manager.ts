import type { NetworkClient, NetworkEventHandlers } from "./network";
import type { RenderBatch, Renderer } from "./render";
import type { WorldPatchBatch, WorldStateStore, WorldKeyframe } from "./world-state";

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

  async boot(_handlers: ClientLifecycleHandlers): Promise<void> {
    throw new Error("Client orchestrator boot is not implemented.");
  }

  async shutdown(): Promise<void> {
    throw new Error("Client orchestrator shutdown is not implemented.");
  }

  handleKeyframe(_keyframe: WorldKeyframe): void {
    throw new Error("Client orchestrator handleKeyframe is not implemented.");
  }

  handlePatchBatch(_patch: WorldPatchBatch): void {
    throw new Error("Client orchestrator handlePatchBatch is not implemented.");
  }

  requestRender(_batch: RenderBatch): void {
    throw new Error("Client orchestrator requestRender is not implemented.");
  }

  private createNetworkHandlers(): NetworkEventHandlers {
    throw new Error("Client orchestrator network handlers are not implemented.");
  }
}
