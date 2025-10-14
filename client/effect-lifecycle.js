const LIFECYCLE_STATE_KEY = "__effectLifecycleState";

function isPlainObject(value) {
  return value != null && typeof value === "object" && !Array.isArray(value);
}

function createLifecycleState() {
  return {
    instances: new Map(),
    lastSeqById: new Map(),
    lastBatchTick: null,
    version: 0,
    recentlyEnded: new Map(),
    recentlyEndedVersion: 0,
  };
}

function isLifecycleState(value) {
  return (
    isPlainObject(value) &&
    value.instances instanceof Map &&
    value.lastSeqById instanceof Map &&
    typeof value.version === "number" &&
    value.recentlyEnded instanceof Map &&
    typeof value.recentlyEndedVersion === "number"
  );
}

function cloneStructured(value) {
  if (value === null || value === undefined) {
    return null;
  }
  if (typeof structuredClone === "function") {
    try {
      return structuredClone(value);
    } catch (err) {
      // Fallback below.
    }
  }
  try {
    const serialized = JSON.stringify(value);
    if (typeof serialized !== "string") {
      return null;
    }
    return JSON.parse(serialized);
  } catch (err) {
    return null;
  }
}

function coerceSequence(value) {
  if (typeof value === "number" && Number.isFinite(value)) {
    const normalized = Math.floor(value);
    return normalized >= 0 ? normalized : null;
  }
  if (typeof value === "string" && value.trim().length > 0) {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) {
      const normalized = Math.floor(parsed);
      return normalized >= 0 ? normalized : null;
    }
  }
  return null;
}

function coerceTick(value) {
  if (typeof value === "number" && Number.isFinite(value)) {
    const normalized = Math.floor(value);
    return normalized >= 0 ? normalized : null;
  }
  return null;
}

function readString(value) {
  if (typeof value === "string") {
    const trimmed = value.trim();
    return trimmed.length > 0 ? trimmed : null;
  }
  return null;
}

function normalizeSpawnEvent(raw) {
  if (!isPlainObject(raw)) {
    return null;
  }
  const seq = coerceSequence(raw.seq);
  if (seq === null) {
    return null;
  }
  const instance = cloneStructured(raw.instance);
  if (!isPlainObject(instance)) {
    return null;
  }
  const id = readString(instance.id);
  if (!id) {
    return null;
  }
  const tick = coerceTick(raw.tick);
  return {
    id,
    seq,
    tick,
    instance,
  };
}

function normalizeUpdateEvent(raw) {
  if (!isPlainObject(raw)) {
    return null;
  }
  const id = readString(raw.id);
  if (!id) {
    return null;
  }
  const seq = coerceSequence(raw.seq);
  if (seq === null) {
    return null;
  }
  const tick = coerceTick(raw.tick);
  const deliveryState = cloneStructured(raw.deliveryState);
  const behaviorState = cloneStructured(raw.behaviorState);
  const params = cloneStructured(raw.params);
  return {
    id,
    seq,
    tick,
    deliveryState: isPlainObject(deliveryState) ? deliveryState : null,
    behaviorState: isPlainObject(behaviorState) ? behaviorState : null,
    params: isPlainObject(params) ? params : null,
  };
}

function normalizeEndEvent(raw) {
  if (!isPlainObject(raw)) {
    return null;
  }
  const id = readString(raw.id);
  if (!id) {
    return null;
  }
  const seq = coerceSequence(raw.seq);
  if (seq === null) {
    return null;
  }
  const tick = coerceTick(raw.tick);
  const reason = readString(raw.reason);
  return {
    id,
    seq,
    tick,
    reason,
  };
}

function normalizeCursorMap(raw) {
  const cursors = new Map();
  if (!raw || typeof raw !== "object") {
    return cursors;
  }
  const entries = raw instanceof Map ? raw.entries() : Object.entries(raw);
  for (const [rawId, rawSeq] of entries) {
    const id = readString(rawId);
    const seq = coerceSequence(rawSeq);
    if (!id || seq === null) {
      continue;
    }
    cursors.set(id, seq);
  }
  return cursors;
}

function normalizeLifecycleBatch(payload) {
  const spawns = Array.isArray(payload?.effect_spawned)
    ? payload.effect_spawned
    : Array.isArray(payload?.effectSpawned)
      ? payload.effectSpawned
      : [];
  const updates = Array.isArray(payload?.effect_update)
    ? payload.effect_update
    : Array.isArray(payload?.effectUpdate)
      ? payload.effectUpdate
      : [];
  const ends = Array.isArray(payload?.effect_ended)
    ? payload.effect_ended
    : Array.isArray(payload?.effectEnded)
      ? payload.effectEnded
      : [];
  const cursors = normalizeCursorMap(
    payload?.effect_seq_cursors ?? payload?.effectSeqCursors ?? null,
  );

  return {
    spawns: spawns.map((event) => normalizeSpawnEvent(event)).filter(Boolean),
    updates: updates.map((event) => normalizeUpdateEvent(event)).filter(Boolean),
    ends: ends.map((event) => normalizeEndEvent(event)).filter(Boolean),
    cursors,
  };
}

function ensureInstance(entry) {
  if (!entry.instance || !isPlainObject(entry.instance)) {
    entry.instance = {};
  }
  return entry.instance;
}

function applyUpdate(entry, event) {
  if (!entry || !event) {
    return;
  }
  const instance = ensureInstance(entry);
  if (event.deliveryState) {
    instance.deliveryState = event.deliveryState;
  }
  if (event.behaviorState) {
    instance.behaviorState = event.behaviorState;
  }
  if (event.params) {
    const existing = isPlainObject(instance.params) ? instance.params : {};
    instance.params = { ...existing, ...event.params };
  }
  entry.seq = event.seq;
  entry.tick = event.tick;
  entry.lastEventKind = "update";
  entry.lastEvent = event;
}

export function ensureEffectLifecycleState(store) {
  if (!store || typeof store !== "object") {
    return createLifecycleState();
  }
  const existing = store[LIFECYCLE_STATE_KEY];
  if (isLifecycleState(existing)) {
    return existing;
  }
  const created = createLifecycleState();
  store[LIFECYCLE_STATE_KEY] = created;
  return created;
}

export function peekEffectLifecycleState(store) {
  if (!store || typeof store !== "object") {
    return null;
  }
  const state = store[LIFECYCLE_STATE_KEY];
  return isLifecycleState(state) ? state : null;
}

export function resetEffectLifecycleState(store) {
  if (!store || typeof store !== "object") {
    return;
  }
  store[LIFECYCLE_STATE_KEY] = createLifecycleState();
  if (store.__effectLifecycleView) {
    delete store.__effectLifecycleView;
  }
  if (store.__effectLifecycleViewVersion) {
    delete store.__effectLifecycleViewVersion;
  }
  if (store.__effectLifecycleViewState) {
    delete store.__effectLifecycleViewState;
  }
  if (Object.prototype.hasOwnProperty.call(store, "__consumedEndedVersion")) {
    delete store.__consumedEndedVersion;
  }
}

export function getEffectLifecycleEntry(store, effectId) {
  if (typeof effectId !== "string" || effectId.length === 0) {
    return null;
  }
  const state = peekEffectLifecycleState(store);
  if (!state) {
    return null;
  }
  return state.instances.get(effectId) ?? null;
}

export function applyEffectLifecycleBatch(store, payload, options = {}) {
  const state = ensureEffectLifecycleState(store);
  const summary = {
    spawns: [],
    updates: [],
    ends: [],
    droppedSpawns: [],
    droppedUpdates: [],
    droppedEnds: [],
    unknownUpdates: [],
  };

  if (!payload || typeof payload !== "object") {
    return summary;
  }

  const batch = normalizeLifecycleBatch(payload);
  let latestTick = state.lastBatchTick;
  let mutated = false;
  if (!(state.recentlyEnded instanceof Map)) {
    state.recentlyEnded = new Map();
  } else {
    state.recentlyEnded.clear();
  }
  let recordedEndedEntries = false;

  for (const spawn of batch.spawns) {
    const lastSeq = state.lastSeqById.get(spawn.id);
    if (lastSeq !== undefined && spawn.seq <= lastSeq) {
      summary.droppedSpawns.push(spawn.id);
      if (typeof console !== "undefined" && typeof console.error === "function") {
        console.error(
          `Rejected effect spawn for "${spawn.id}" because sequence ${spawn.seq} is not greater than ${lastSeq}.`,
        );
      }
      continue;
    }
    const entry = {
      id: spawn.id,
      seq: spawn.seq,
      tick: spawn.tick,
      instance: spawn.instance,
      lastEventKind: "spawn",
      lastEvent: spawn,
    };
    state.instances.set(spawn.id, entry);
    state.lastSeqById.set(spawn.id, spawn.seq);
    summary.spawns.push(spawn.id);
    mutated = true;
    if (spawn.tick !== null) {
      latestTick = latestTick === null ? spawn.tick : Math.max(latestTick, spawn.tick);
    }
  }

  const pendingUpdates = [];
  for (const update of batch.updates) {
    const lastSeq = state.lastSeqById.get(update.id);
    if (lastSeq !== undefined && update.seq <= lastSeq) {
      summary.droppedUpdates.push(update.id);
      continue;
    }
    const entry = state.instances.get(update.id);
    if (!entry) {
      pendingUpdates.push(update);
      continue;
    }
    applyUpdate(entry, update);
    state.lastSeqById.set(update.id, update.seq);
    summary.updates.push(update.id);
    mutated = true;
    if (update.tick !== null) {
      latestTick = latestTick === null ? update.tick : Math.max(latestTick, update.tick);
    }
  }

  if (pendingUpdates.length > 0) {
    for (const update of pendingUpdates) {
      const lastSeq = state.lastSeqById.get(update.id);
      if (lastSeq !== undefined && update.seq <= lastSeq) {
        summary.droppedUpdates.push(update.id);
        continue;
      }
      const entry = state.instances.get(update.id);
      if (!entry) {
        summary.unknownUpdates.push(update);
        continue;
      }
      applyUpdate(entry, update);
      state.lastSeqById.set(update.id, update.seq);
      summary.updates.push(update.id);
      mutated = true;
      if (update.tick !== null) {
        latestTick = latestTick === null ? update.tick : Math.max(latestTick, update.tick);
      }
    }
  }

  for (const end of batch.ends) {
    const lastSeq = state.lastSeqById.get(end.id);
    if (lastSeq !== undefined && end.seq <= lastSeq) {
      summary.droppedEnds.push(end.id);
      continue;
    }
    const entry = state.instances.get(end.id);
    if (!entry) {
      summary.droppedEnds.push(end.id);
      state.lastSeqById.set(end.id, end.seq);
      mutated = true;
      continue;
    }
    entry.seq = end.seq;
    entry.tick = end.tick;
    entry.lastEventKind = "end";
    entry.lastEvent = end;
    entry.endReason = end.reason ?? null;
    state.lastSeqById.set(end.id, end.seq);
    state.recentlyEnded.set(end.id, entry);
    recordedEndedEntries = true;
    state.instances.delete(end.id);
    summary.ends.push(end.id);
    mutated = true;
    if (end.tick !== null) {
      latestTick = latestTick === null ? end.tick : Math.max(latestTick, end.tick);
    }
  }

  for (const [id, seq] of batch.cursors.entries()) {
    const lastSeq = state.lastSeqById.get(id);
    if (lastSeq === undefined || seq > lastSeq) {
      state.lastSeqById.set(id, seq);
      mutated = true;
    }
  }

  if (latestTick !== null) {
    const previousTick = state.lastBatchTick;
    if (previousTick === null || latestTick > previousTick) {
      state.lastBatchTick = latestTick;
      mutated = true;
    }
  }

  if (mutated) {
    state.version = (state.version + 1) >>> 0;
  }

  if (recordedEndedEntries) {
    state.recentlyEndedVersion = (state.recentlyEndedVersion + 1) >>> 0;
  }

  if (Array.isArray(summary.unknownUpdates) && summary.unknownUpdates.length > 0) {
    const { onUnknownUpdate } = options;
    if (typeof onUnknownUpdate === "function") {
      for (const unknown of summary.unknownUpdates) {
        onUnknownUpdate(unknown);
      }
    }
  }

  return summary;
}
