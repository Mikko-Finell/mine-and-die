import { isLifecycleClientManaged } from "./effect-lifecycle-metadata";
import type {
  EffectContractID,
  EffectContractMap,
  EffectInstance,
  InstanceEndPayload,
  InstanceUpdatePayload,
} from "./generated/effect-contracts";

type LifecycleEventKind = "spawn" | "update" | "end";

type SpawnPayload = EffectContractMap[EffectContractID]["spawn"];
type UpdatePayload = EffectContractMap[EffectContractID]["update"];
type EndPayload = EffectContractMap[EffectContractID]["end"];
type ContractInstance = SpawnPayload["instance"];

export interface ContractLifecycleSpawnEvent {
  readonly seq: number;
  readonly tick?: number | null;
  readonly instance: EffectInstance;
}

export interface ContractLifecycleUpdateEvent {
  readonly seq: number;
  readonly tick?: number | null;
  readonly id: string;
  readonly deliveryState?: InstanceUpdatePayload["deliveryState"];
  readonly behaviorState?: InstanceUpdatePayload["behaviorState"];
  readonly params?: InstanceUpdatePayload["params"];
}

export interface ContractLifecycleEndEvent {
  readonly seq: number;
  readonly tick?: number | null;
  readonly id: string;
  readonly reason?: InstanceEndPayload["reason"];
}

export interface ContractLifecycleBatch {
  readonly spawns?: readonly ContractLifecycleSpawnEvent[];
  readonly updates?: readonly ContractLifecycleUpdateEvent[];
  readonly ends?: readonly ContractLifecycleEndEvent[];
  readonly cursors?: Readonly<Record<string, number>> | Map<string, number>;
}

export interface ContractLifecycleSummary {
  readonly spawns: readonly string[];
  readonly updates: readonly string[];
  readonly ends: readonly string[];
  readonly retained: readonly string[];
  readonly droppedSpawns: readonly string[];
  readonly droppedUpdates: readonly string[];
  readonly droppedEnds: readonly string[];
  readonly unknownUpdates: readonly ContractLifecycleUpdateEvent[];
}

export interface ContractLifecycleEntry<
  TContract extends EffectContractID = EffectContractID,
> {
  readonly id: string;
  readonly contractId: TContract;
  readonly entryId: string | null;
  readonly seq: number;
  readonly tick: number | null;
  readonly spawn: EffectContractMap[TContract]["spawn"];
  readonly lastUpdate: EffectContractMap[TContract]["update"] | null;
  readonly end: EffectContractMap[TContract]["end"] | null;
  readonly instance: EffectContractMap[TContract]["spawn"]["instance"];
  readonly lastEventKind: LifecycleEventKind;
  readonly managedByClient: boolean;
  readonly retained: boolean;
}

export interface ContractLifecycleView {
  readonly version: number;
  readonly lastBatchTick: number | null;
  readonly entries: ReadonlyMap<string, ContractLifecycleEntry>;
  readonly recentlyEnded: ReadonlyMap<string, ContractLifecycleEntry>;
  readonly getEntry: (id: string) => ContractLifecycleEntry | null;
}

interface InternalLifecycleEntry {
  id: string;
  contractId: EffectContractID;
  entryId: string | null;
  seq: number;
  tick: number | null;
  lastEventKind: LifecycleEventKind;
  managedByClient: boolean;
  retained: boolean;
  spawn: SpawnPayload;
  lastUpdate: UpdatePayload | null;
  end: EndPayload | null;
  instance: ContractInstance;
}

const cloneValue = <T>(value: T): T => {
  if (value === null || typeof value !== "object") {
    return value;
  }
  if (Array.isArray(value)) {
    return value.map((item) => cloneValue(item)) as unknown as T;
  }
  const result: Record<string, unknown> = {};
  for (const [key, entry] of Object.entries(value as Record<string, unknown>)) {
    result[key] = cloneValue(entry);
  }
  return result as T;
};

const normalizeTick = (tick: number | null | undefined): number | null =>
  typeof tick === "number" && Number.isFinite(tick) ? Math.floor(tick) : null;

const normalizeSeq = (seq: number | null | undefined): number | null =>
  typeof seq === "number" && Number.isFinite(seq) ? Math.floor(seq) : null;

const createSpawnPayload = (instance: EffectInstance): SpawnPayload => ({
  instance: cloneValue(instance),
});

const createUpdatePayload = (
  update: ContractLifecycleUpdateEvent,
): UpdatePayload => {
  const payload: InstanceUpdatePayload = { id: update.id };
  if (update.deliveryState) {
    payload.deliveryState = cloneValue(update.deliveryState);
  }
  if (update.behaviorState) {
    payload.behaviorState = cloneValue(update.behaviorState);
  }
  if (update.params) {
    payload.params = { ...update.params };
  }
  return payload;
};

const createEndPayload = (end: ContractLifecycleEndEvent): EndPayload => ({
  id: end.id,
  reason: end.reason ?? "expired",
});

const viewForEntry = (
  entry: InternalLifecycleEntry,
): ContractLifecycleEntry => ({
  id: entry.id,
  contractId: entry.contractId,
  entryId: entry.entryId,
  seq: entry.seq,
  tick: entry.tick,
  spawn: entry.spawn,
  lastUpdate: entry.lastUpdate,
  end: entry.end,
  instance: entry.instance,
  lastEventKind: entry.lastEventKind,
  managedByClient: entry.managedByClient,
  retained: entry.retained,
});

export class ContractLifecycleStore {
  private readonly entries = new Map<string, InternalLifecycleEntry>();
  private readonly recentlyEnded = new Map<string, InternalLifecycleEntry>();
  private readonly lastSeqById = new Map<string, number>();
  private lastBatchTick: number | null = null;
  private version = 0;

  reset(): void {
    this.entries.clear();
    this.recentlyEnded.clear();
    this.lastSeqById.clear();
    this.lastBatchTick = null;
    this.version = 0;
  }

  snapshot(): ContractLifecycleView {
    const entryView = new Map<string, ContractLifecycleEntry>();
    for (const [id, entry] of this.entries.entries()) {
      entryView.set(id, viewForEntry(entry));
    }

    const recentlyEndedView = new Map<string, ContractLifecycleEntry>();
    for (const [id, entry] of this.recentlyEnded.entries()) {
      recentlyEndedView.set(id, viewForEntry(entry));
    }

    return {
      version: this.version,
      lastBatchTick: this.lastBatchTick,
      entries: entryView,
      recentlyEnded: recentlyEndedView,
      getEntry: (id: string): ContractLifecycleEntry | null => {
        if (typeof id !== "string" || id.length === 0) {
          return null;
        }
        return entryView.get(id) ?? recentlyEndedView.get(id) ?? null;
      },
    };
  }

  applyBatch(batch: ContractLifecycleBatch | null | undefined): ContractLifecycleSummary {
    const spawns: string[] = [];
    const updates: string[] = [];
    const ends: string[] = [];
    const retained: string[] = [];
    const droppedSpawns: string[] = [];
    const droppedUpdates: string[] = [];
    const droppedEnds: string[] = [];
    const unknownUpdates: ContractLifecycleUpdateEvent[] = [];

    if (this.recentlyEnded.size > 0) {
      this.recentlyEnded.clear();
    }

    if (!batch) {
      return {
        spawns,
        updates,
        ends,
        retained,
        droppedSpawns,
        droppedUpdates,
        droppedEnds,
        unknownUpdates,
      };
    }

    let mutated = false;
    let latestTick = this.lastBatchTick;

    const recordTick = (tick: number | null): void => {
      if (tick === null) {
        return;
      }
      if (latestTick === null || tick > latestTick) {
        latestTick = tick;
        mutated = true;
      }
    };

    const spawnEvents = batch.spawns ?? [];
    for (const spawn of spawnEvents) {
      if (!spawn || typeof spawn !== "object") {
        continue;
      }
      const seq = normalizeSeq(spawn.seq);
      const instance = spawn.instance;
      const id = instance?.id ?? null;
      if (!instance || typeof id !== "string" || id.length === 0 || seq === null) {
        continue;
      }

      const lastSeq = this.lastSeqById.get(id);
      if (lastSeq !== undefined && seq <= lastSeq) {
        droppedSpawns.push(id);
        continue;
      }

      const tick = normalizeTick(spawn.tick ?? null);
      const spawnPayload = createSpawnPayload(instance);
      const rawEntryId = spawnPayload.instance.entryId;
      const entryId =
        typeof rawEntryId === "string" && rawEntryId.length > 0 ? rawEntryId : null;
      const managedByClient = isLifecycleClientManaged({
        entryId,
        instance: spawnPayload.instance,
      });

      const entry: InternalLifecycleEntry = {
        id,
        contractId: spawnPayload.instance.definitionId as EffectContractID,
        entryId,
        seq,
        tick,
        lastEventKind: "spawn",
        managedByClient,
        retained: false,
        spawn: spawnPayload,
        lastUpdate: null,
        end: null,
        instance: spawnPayload.instance,
      };

      this.entries.set(id, entry);
      this.lastSeqById.set(id, seq);
      spawns.push(id);
      mutated = true;
      recordTick(tick);
    }

    const updateEvents = batch.updates ?? [];
    for (const update of updateEvents) {
      if (!update || typeof update !== "object") {
        continue;
      }
      const seq = normalizeSeq(update.seq);
      const id = typeof update.id === "string" ? update.id : null;
      if (!id || seq === null) {
        continue;
      }

      const lastSeq = this.lastSeqById.get(id);
      if (lastSeq !== undefined && seq <= lastSeq) {
        droppedUpdates.push(id);
        continue;
      }

      const entry = this.entries.get(id);
      if (!entry) {
        unknownUpdates.push(update);
        continue;
      }

      const tick = normalizeTick(update.tick ?? null);
      let instance = entry.instance;
      if (update.deliveryState) {
        instance = {
          ...instance,
          deliveryState: cloneValue(update.deliveryState),
        };
      }
      if (update.behaviorState) {
        instance = {
          ...instance,
          behaviorState: cloneValue(update.behaviorState),
        };
      }
      if (update.params) {
        const mergedParams = {
          ...(instance.params ?? {}),
          ...update.params,
        };
        instance = {
          ...instance,
          params: mergedParams,
        };
      }

      entry.seq = seq;
      entry.tick = tick;
      entry.lastEventKind = "update";
      entry.instance = instance;
      entry.lastUpdate = createUpdatePayload(update);
      entry.retained = false;
      this.entries.set(id, entry);
      this.lastSeqById.set(id, seq);
      updates.push(id);
      mutated = true;
      recordTick(tick);
    }

    const endEvents = batch.ends ?? [];
    for (const end of endEvents) {
      if (!end || typeof end !== "object") {
        continue;
      }
      const seq = normalizeSeq(end.seq);
      const id = typeof end.id === "string" ? end.id : null;
      if (!id || seq === null) {
        continue;
      }

      const lastSeq = this.lastSeqById.get(id);
      if (lastSeq !== undefined && seq <= lastSeq) {
        droppedEnds.push(id);
        continue;
      }

      const tick = normalizeTick(end.tick ?? null);
      const entry = this.entries.get(id);
      if (!entry) {
        this.lastSeqById.set(id, seq);
        droppedEnds.push(id);
        mutated = true;
        recordTick(tick);
        continue;
      }

      entry.seq = seq;
      entry.tick = tick;
      entry.lastEventKind = "end";
      entry.end = createEndPayload(end);
      entry.retained = entry.managedByClient;
      this.lastSeqById.set(id, seq);
      ends.push(id);
      mutated = true;
      recordTick(tick);

      if (entry.managedByClient) {
        retained.push(id);
        this.entries.set(id, entry);
      } else {
        this.entries.delete(id);
        this.recentlyEnded.set(id, entry);
      }
    }

    const cursors = batch.cursors;
    if (cursors) {
      const iterator: Iterable<[string, number]> =
        cursors instanceof Map
          ? cursors.entries()
          : Object.entries(cursors as Record<string, number>);
      for (const [id, seqValue] of iterator) {
        const seq = normalizeSeq(seqValue);
        if (typeof id === "string" && id.length > 0 && seq !== null) {
          const lastSeq = this.lastSeqById.get(id);
          if (lastSeq === undefined || seq > lastSeq) {
            this.lastSeqById.set(id, seq);
            mutated = true;
          }
        }
      }
    }

    if (mutated) {
      this.version = (this.version + 1) >>> 0;
    }

    if (latestTick !== this.lastBatchTick) {
      this.lastBatchTick = latestTick ?? null;
    }

    return {
      spawns,
      updates,
      ends,
      retained,
      droppedSpawns,
      droppedUpdates,
      droppedEnds,
      unknownUpdates,
    };
  }
}

