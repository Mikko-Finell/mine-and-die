export interface WorldEntityState {
  readonly id: string;
  readonly type: string;
  readonly position: readonly [number, number];
  readonly [key: string]: unknown;
}

export interface WorldKeyframe {
  readonly id: string;
  readonly timestamp: number;
  readonly entities: readonly WorldEntityState[];
  readonly metadata: Record<string, unknown>;
}

export interface WorldPatchOperation {
  readonly entityId: string;
  readonly path: readonly (string | number)[];
  readonly value: unknown;
}

export interface WorldPatchBatch {
  readonly keyframeId: string;
  readonly timestamp: number;
  readonly operations: readonly WorldPatchOperation[];
}

export interface WorldStateSnapshot {
  readonly keyframe: WorldKeyframe | null;
  readonly entities: Map<string, WorldEntityState>;
}

export interface WorldStateStore {
  readonly snapshot: () => WorldStateSnapshot;
  readonly applyKeyframe: (keyframe: WorldKeyframe) => void;
  readonly applyPatchBatch: (patch: WorldPatchBatch) => void;
  readonly reset: () => void;
}

export class InMemoryWorldStateStore implements WorldStateStore {
  snapshot(): WorldStateSnapshot {
    throw new Error("WorldState snapshot is not implemented.");
  }

  applyKeyframe(_keyframe: WorldKeyframe): void {
    throw new Error("WorldState applyKeyframe is not implemented.");
  }

  applyPatchBatch(_patch: WorldPatchBatch): void {
    throw new Error("WorldState applyPatchBatch is not implemented.");
  }

  reset(): void {
    throw new Error("WorldState reset is not implemented.");
  }
}
