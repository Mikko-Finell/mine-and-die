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

const cloneValue = <T>(value: T): T => {
  if (value === null || typeof value !== "object") {
    return value;
  }

  if (Array.isArray(value)) {
    return value.map((entry) => cloneValue(entry)) as unknown as T;
  }

  const clone: Record<string, unknown> = {};
  for (const [key, entry] of Object.entries(value as Record<string, unknown>)) {
    clone[key] = cloneValue(entry);
  }
  return clone as T;
};

const cloneEntity = (entity: WorldEntityState): WorldEntityState =>
  cloneValue(entity) as WorldEntityState;

const cloneKeyframe = (keyframe: WorldKeyframe): WorldKeyframe => ({
  id: keyframe.id,
  timestamp: keyframe.timestamp,
  entities: keyframe.entities.map((entity) => cloneEntity(entity)),
  metadata: cloneValue(keyframe.metadata),
});

const isPlainObject = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null && !Array.isArray(value);

const setDeepValue = (
  target: Record<string, unknown>,
  path: readonly (string | number)[],
  value: unknown,
): void => {
  if (path.length === 0) {
    return;
  }

  let cursor: Record<string, unknown> | unknown[] = target;

  for (let index = 0; index < path.length - 1; index += 1) {
    const key = path[index];
    const nextKey = path[index + 1];

    if (Array.isArray(cursor)) {
      const numericKey = typeof key === "number" ? key : Number.parseInt(String(key), 10);
      if (!Number.isFinite(numericKey) || numericKey < 0) {
        return;
      }
      const existing = cursor[numericKey];
      if (existing && typeof existing === "object") {
        cursor[numericKey] = cloneValue(existing);
      } else {
        cursor[numericKey] = typeof nextKey === "number" ? [] : {};
      }
      cursor = cursor[numericKey] as Record<string, unknown> | unknown[];
      continue;
    }

    if (typeof key !== "string") {
      return;
    }

    const existing = cursor[key];
    if (existing && typeof existing === "object") {
      cursor[key] = cloneValue(existing);
    } else {
      cursor[key] = typeof nextKey === "number" ? [] : {};
    }
    cursor = cursor[key] as Record<string, unknown> | unknown[];
  }

  const lastKey = path[path.length - 1];
  const clonedValue = cloneValue(value);

  if (Array.isArray(cursor)) {
    const numericKey = typeof lastKey === "number" ? lastKey : Number.parseInt(String(lastKey), 10);
    if (!Number.isFinite(numericKey) || numericKey < 0) {
      return;
    }
    if (clonedValue === undefined) {
      cursor.splice(numericKey, 1);
    } else {
      cursor[numericKey] = clonedValue;
    }
    return;
  }

  if (typeof lastKey !== "string") {
    return;
  }

  if (clonedValue === undefined) {
    delete cursor[lastKey];
  } else {
    cursor[lastKey] = clonedValue;
  }
};

export class InMemoryWorldStateStore implements WorldStateStore {
  private keyframe: WorldKeyframe | null = null;
  private readonly entities = new Map<string, WorldEntityState>();

  snapshot(): WorldStateSnapshot {
    const entitySnapshot = new Map<string, WorldEntityState>();
    for (const [id, entity] of this.entities.entries()) {
      entitySnapshot.set(id, cloneEntity(entity));
    }

    return {
      keyframe: this.keyframe ? cloneKeyframe(this.keyframe) : null,
      entities: entitySnapshot,
    };
  }

  applyKeyframe(keyframe: WorldKeyframe): void {
    this.keyframe = cloneKeyframe(keyframe);
    this.entities.clear();
    for (const entity of keyframe.entities) {
      this.entities.set(entity.id, cloneEntity(entity));
    }
  }

  applyPatchBatch(patch: WorldPatchBatch): void {
    const operations = patch.operations ?? [];
    for (const operation of operations) {
      if (!operation || typeof operation !== "object") {
        continue;
      }
      const entityId = operation.entityId;
      if (typeof entityId !== "string" || entityId.length === 0) {
        continue;
      }

      const path = operation.path ?? [];
      if (!Array.isArray(path)) {
        continue;
      }

      if (path.length === 0) {
        if (operation.value === null || operation.value === undefined) {
          this.entities.delete(entityId);
          continue;
        }
        if (!isPlainObject(operation.value)) {
          continue;
        }
        this.entities.set(entityId, cloneValue(operation.value) as WorldEntityState);
        continue;
      }

      const existing = this.entities.get(entityId);
      if (!existing) {
        if (!isPlainObject(operation.value)) {
          continue;
        }
        this.entities.set(entityId, cloneValue(operation.value) as WorldEntityState);
        continue;
      }

      const updated = cloneEntity(existing);
      setDeepValue(updated as unknown as Record<string, unknown>, path, operation.value);
      this.entities.set(entityId, updated);
    }
  }

  reset(): void {
    this.keyframe = null;
    this.entities.clear();
  }
}
