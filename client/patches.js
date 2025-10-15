const PATCH_KIND_PLAYER_POS = "player_pos";
const PATCH_KIND_PLAYER_FACING = "player_facing";
const PATCH_KIND_PLAYER_INTENT = "player_intent";
const PATCH_KIND_PLAYER_HEALTH = "player_health";
const PATCH_KIND_PLAYER_INVENTORY = "player_inventory";
const PATCH_KIND_PLAYER_REMOVED = "player_removed";
const PATCH_KIND_NPC_POS = "npc_pos";
const PATCH_KIND_NPC_FACING = "npc_facing";
const PATCH_KIND_NPC_HEALTH = "npc_health";
const PATCH_KIND_NPC_INVENTORY = "npc_inventory";
const PATCH_KIND_GROUND_ITEM_POS = "ground_item_pos";
const PATCH_KIND_GROUND_ITEM_QTY = "ground_item_qty";

const KNOWN_PATCH_KINDS = new Set([
  PATCH_KIND_PLAYER_POS,
  PATCH_KIND_PLAYER_FACING,
  PATCH_KIND_PLAYER_INTENT,
  PATCH_KIND_PLAYER_HEALTH,
  PATCH_KIND_PLAYER_INVENTORY,
  PATCH_KIND_PLAYER_REMOVED,
  PATCH_KIND_NPC_POS,
  PATCH_KIND_NPC_FACING,
  PATCH_KIND_NPC_HEALTH,
  PATCH_KIND_NPC_INVENTORY,
  PATCH_KIND_GROUND_ITEM_POS,
  PATCH_KIND_GROUND_ITEM_QTY,
]);

const VALID_FACINGS = new Set(["up", "down", "left", "right"]);
const DEFAULT_FACING = "down";
const DEFAULT_ERROR_LIMIT = 20;
const DEFAULT_PATCH_HISTORY_LIMIT = 128;
const DEFAULT_KEYFRAME_CACHE_LIMIT = 8;
const KEYFRAME_RECOVERY_MAX_ATTEMPTS = 3;
const KEYFRAME_RECOVERY_BASE_DELAY = 200;
const KEYFRAME_RECOVERY_MAX_STEP = 2000;
const RESOLVED_SEQUENCE_LIMIT = 16;

function toFiniteNumber(value, fallback = 0) {
  const num = Number(value);
  return Number.isFinite(num) ? num : fallback;
}

function toFiniteInt(value, fallback = 0) {
  const num = Number(value);
  if (!Number.isFinite(num)) {
    return fallback;
  }
  return Math.trunc(num);
}

function normalizeFacingValue(value) {
  if (typeof value === "string" && VALID_FACINGS.has(value)) {
    return value;
  }
  return DEFAULT_FACING;
}

function readFungibilityKey(itemSource) {
  if (!itemSource || typeof itemSource !== "object") {
    return null;
  }
  if (Object.hasOwn(itemSource, "fungibility_key")) {
    const value = itemSource.fungibility_key;
    return typeof value === "string" ? value : "";
  }
  if (Object.hasOwn(itemSource, "fungibilityKey")) {
    const value = itemSource.fungibilityKey;
    return typeof value === "string" ? value : "";
  }
  return null;
}

function cloneInventorySlots(slots) {
  if (!Array.isArray(slots) || slots.length === 0) {
    return [];
  }
  const cloned = [];
  for (const slot of slots) {
    if (!slot || typeof slot !== "object") {
      continue;
    }
    const slotIndex = toFiniteInt(slot.slot, null);
    if (slotIndex === null) {
      continue;
    }
    const itemSource = slot.item && typeof slot.item === "object" ? slot.item : {};
    const type = typeof itemSource.type === "string" ? itemSource.type : "";
    const quantity = toFiniteInt(itemSource.quantity, 0);
    const fungibilityKey = readFungibilityKey(itemSource);
    const item = { type, quantity };
    if (fungibilityKey !== null) {
      item.fungibility_key = fungibilityKey;
    }
    cloned.push({
      slot: slotIndex,
      item,
    });
  }
  return cloned;
}

function createPlayerView(player) {
  if (!player || typeof player !== "object") {
    return null;
  }
  const id = normalizeEntityId(player.id);
  if (!id) {
    return null;
  }
  const inventorySource =
    player.inventory && typeof player.inventory === "object"
      ? player.inventory
      : { slots: [] };
  const intentDX =
    player.intentDX ?? player.intentDx ?? player.intent_x ?? player.intentx ?? 0;
  const intentDY =
    player.intentDY ?? player.intentDy ?? player.intent_y ?? player.intenty ?? 0;

  return {
    id,
    x: toFiniteNumber(player.x, 0),
    y: toFiniteNumber(player.y, 0),
    facing: normalizeFacingValue(player.facing),
    health: toFiniteNumber(player.health, 0),
    maxHealth: toFiniteNumber(player.maxHealth, 0),
    intentDX: toFiniteNumber(intentDX, 0),
    intentDY: toFiniteNumber(intentDY, 0),
    inventory: { slots: cloneInventorySlots(inventorySource.slots) },
  };
}

function clonePlayerView(view) {
  if (!view || typeof view !== "object") {
    return null;
  }
  const id = normalizeEntityId(view.id);
  if (!id) {
    return null;
  }
  return {
    id,
    x: toFiniteNumber(view.x, 0),
    y: toFiniteNumber(view.y, 0),
    facing: normalizeFacingValue(view.facing),
    health: toFiniteNumber(view.health, 0),
    maxHealth: toFiniteNumber(view.maxHealth, 0),
    intentDX: toFiniteNumber(view.intentDX, 0),
    intentDY: toFiniteNumber(view.intentDY, 0),
    inventory: {
      slots: cloneInventorySlots(
        view.inventory && typeof view.inventory === "object"
          ? view.inventory.slots
          : [],
      ),
    },
  };
}

function clonePlayersMap(source) {
  const next = Object.create(null);
  if (!source || typeof source !== "object") {
    return next;
  }
  for (const view of Object.values(source)) {
    const cloned = clonePlayerView(view);
    if (!cloned || !cloned.id) {
      continue;
    }
    next[cloned.id] = cloned;
  }
  return next;
}

function createNPCView(npc) {
  if (!npc || typeof npc !== "object") {
    return null;
  }
  const id = normalizeEntityId(npc.id);
  if (!id) {
    return null;
  }
  const inventorySource =
    npc.inventory && typeof npc.inventory === "object"
      ? npc.inventory
      : { slots: [] };
  return {
    id,
    x: toFiniteNumber(npc.x, 0),
    y: toFiniteNumber(npc.y, 0),
    facing: normalizeFacingValue(npc.facing),
    health: toFiniteNumber(npc.health, 0),
    maxHealth: toFiniteNumber(npc.maxHealth, 0),
    type: typeof npc.type === "string" ? npc.type : "",
    aiControlled: npc.aiControlled === true,
    experienceReward: toFiniteInt(npc.experienceReward, 0),
    inventory: { slots: cloneInventorySlots(inventorySource.slots) },
  };
}

function cloneNPCView(view) {
  if (!view || typeof view !== "object") {
    return null;
  }
  const id = normalizeEntityId(view.id);
  if (!id) {
    return null;
  }
  return {
    id,
    x: toFiniteNumber(view.x, 0),
    y: toFiniteNumber(view.y, 0),
    facing: normalizeFacingValue(view.facing),
    health: toFiniteNumber(view.health, 0),
    maxHealth: toFiniteNumber(view.maxHealth, 0),
    type: typeof view.type === "string" ? view.type : "",
    aiControlled: view.aiControlled === true,
    experienceReward: toFiniteInt(view.experienceReward, 0),
    inventory: {
      slots: cloneInventorySlots(
        view.inventory && typeof view.inventory === "object"
          ? view.inventory.slots
          : [],
      ),
    },
  };
}

function cloneNPCsMap(source) {
  const next = Object.create(null);
  if (!source || typeof source !== "object") {
    return next;
  }
  for (const view of Object.values(source)) {
    const cloned = cloneNPCView(view);
    if (!cloned || !cloned.id) {
      continue;
    }
    next[cloned.id] = cloned;
  }
  return next;
}

function createGroundItemView(item) {
  if (!item || typeof item !== "object") {
    return null;
  }
  const id = normalizeEntityId(item.id);
  if (!id) {
    return null;
  }
  return {
    id,
    type: typeof item.type === "string" ? item.type : "",
    x: toFiniteNumber(item.x, 0),
    y: toFiniteNumber(item.y, 0),
    qty: Math.max(0, toFiniteInt(item.qty, 0)),
  };
}

function cloneGroundItemView(view) {
  if (!view || typeof view !== "object") {
    return null;
  }
  const id = normalizeEntityId(view.id);
  if (!id) {
    return null;
  }
  return {
    id,
    type: typeof view.type === "string" ? view.type : "",
    x: toFiniteNumber(view.x, 0),
    y: toFiniteNumber(view.y, 0),
    qty: Math.max(0, toFiniteInt(view.qty, 0)),
  };
}

function cloneGroundItemsMap(source) {
  const next = Object.create(null);
  if (!source || typeof source !== "object") {
    return next;
  }
  for (const view of Object.values(source)) {
    const cloned = cloneGroundItemView(view);
    if (!cloned || !cloned.id) {
      continue;
    }
    next[cloned.id] = cloned;
  }
  return next;
}

function buildBaselineFromSnapshot(payload) {
  const players = Object.create(null);
  if (Array.isArray(payload?.players)) {
    for (const entry of payload.players) {
      const view = createPlayerView(entry);
      if (!view) {
        continue;
      }
      players[view.id] = view;
    }
  }
  const npcs = Object.create(null);
  if (Array.isArray(payload?.npcs)) {
    for (const entry of payload.npcs) {
      const view = createNPCView(entry);
      if (!view) {
        continue;
      }
      npcs[view.id] = view;
    }
  }
  const groundItems = Object.create(null);
  if (Array.isArray(payload?.groundItems)) {
    for (const entry of payload.groundItems) {
      const view = createGroundItemView(entry);
      if (!view) {
        continue;
      }
      groundItems[view.id] = view;
    }
  }
  let tick = null;
  if (payload && typeof payload === "object" && Object.hasOwn(payload, "t")) {
    const tickValue = toFiniteNumber(payload.t, null);
    if (tickValue !== null && tickValue >= 0) {
      tick = Math.floor(tickValue);
    }
  }
  const sequence = readBatchSequence(payload);
  return {
    tick,
    sequence,
    players,
    npcs,
    groundItems,
  };
}

function coerceTick(value) {
  if (value === null || value === undefined) {
    return null;
  }
  const tick = toFiniteInt(value, null);
  if (tick === null || tick < 0) {
    return null;
  }
  return tick;
}

function readBatchSequence(payload) {
  if (!payload || typeof payload !== "object") {
    return null;
  }
  const candidates = [payload.sequence, payload.sequenceNumber];
  for (const candidate of candidates) {
    const seq = coerceTick(candidate);
    if (seq !== null) {
      return seq;
    }
  }
  if (Object.hasOwn(payload, "t")) {
    const tickCandidate = coerceTick(payload.t);
    if (tickCandidate !== null) {
      return tickCandidate;
    }
  }
  return null;
}

function readPatchSequence(patch) {
  if (!patch || typeof patch !== "object") {
    return null;
  }
  const payload = patch.payload && typeof patch.payload === "object" ? patch.payload : null;
  const candidates = [
    patch.sequence,
    patch.tick,
    patch.t,
    payload?.sequence,
    payload?.tick,
    payload?.t,
    payload?.version,
  ];
  for (const candidate of candidates) {
    const tick = coerceTick(candidate);
    if (tick !== null) {
      return tick;
    }
  }
  return null;
}

function createPatchHistory(limit = DEFAULT_PATCH_HISTORY_LIMIT) {
  const normalizedLimit = Number.isFinite(limit) && limit > 0
    ? Math.floor(limit)
    : DEFAULT_PATCH_HISTORY_LIMIT;
  return {
    limit: normalizedLimit,
    map: new Map(),
  };
}

function clonePatchHistory(history) {
  if (!history || typeof history !== "object") {
    return createPatchHistory();
  }
  const limit = Number.isFinite(history.limit) && history.limit > 0
    ? Math.floor(history.limit)
    : DEFAULT_PATCH_HISTORY_LIMIT;
  const map = history.map instanceof Map ? new Map(history.map) : new Map();
  return { limit, map };
}

function shouldSkipPatch(history, key, tick) {
  if (!history || !(history.map instanceof Map)) {
    return false;
  }
  if (tick === null) {
    return false;
  }
  const lastTick = history.map.get(key);
  return typeof lastTick === "number" && lastTick >= tick;
}

function rememberPatch(history, key, tick) {
  if (!history || !(history.map instanceof Map)) {
    return;
  }
  if (tick === null) {
    return;
  }
  if (history.map.has(key)) {
    history.map.delete(key);
  }
  history.map.set(key, tick);
  const limit = Number.isFinite(history.limit) && history.limit > 0
    ? Math.floor(history.limit)
    : DEFAULT_PATCH_HISTORY_LIMIT;
  while (history.map.size > limit) {
    const iterator = history.map.keys();
    const first = iterator.next();
    if (first.done) {
      break;
    }
    history.map.delete(first.value);
  }
}

function cloneBaselineSnapshot(baseline) {
  if (!baseline || typeof baseline !== "object") {
    return null;
  }
  const tick = coerceTick(baseline.tick);
  const sequence = coerceTick(baseline.sequence);
  return {
    tick,
    sequence,
    players: clonePlayersMap(baseline.players),
    npcs: cloneNPCsMap(baseline.npcs),
    groundItems: cloneGroundItemsMap(baseline.groundItems),
  };
}

function rememberKeyframeSnapshot(cache, baseline) {
  if (!cache || !(cache.map instanceof Map)) {
    return null;
  }
  const snapshot = cloneBaselineSnapshot(baseline);
  if (!snapshot) {
    return null;
  }
  const sequence = snapshot.sequence;
  if (sequence === null || sequence === undefined) {
    return null;
  }
  if (cache.map.has(sequence)) {
    cache.map.delete(sequence);
  }
  cache.map.set(sequence, snapshot);
  const limit = Number.isFinite(cache.limit) && cache.limit > 0
    ? Math.floor(cache.limit)
    : DEFAULT_KEYFRAME_CACHE_LIMIT;
  cache.limit = limit;
  while (cache.map.size > limit) {
    const iterator = cache.map.keys();
    const first = iterator.next();
    if (first.done) {
      break;
    }
    cache.map.delete(first.value);
  }
  return snapshot;
}

function getCachedKeyframe(cache, sequence) {
  if (!cache || !(cache.map instanceof Map)) {
    return null;
  }
  if (!Number.isFinite(sequence) || sequence <= 0) {
    return null;
  }
  const snapshot = cache.map.get(sequence);
  if (!snapshot) {
    return null;
  }
  return cloneBaselineSnapshot(snapshot);
}

function hasExplicitEntityArrays(payload) {
  if (!payload || typeof payload !== "object") {
    return false;
  }
  const fields = ["players", "npcs", "groundItems"];
  return fields.some((field) => Array.isArray(payload[field]));
}

function readKeyframeSequence(payload) {
  if (!payload || typeof payload !== "object") {
    return null;
  }
  const candidates = [
    payload.keyframeSeq,
    payload.kfSeq,
    payload.keyframeSequence,
    payload.keyframe_sequence,
    payload.baselineSeq,
  ];
  for (const candidate of candidates) {
    const seq = coerceTick(candidate);
    if (seq !== null) {
      return seq;
    }
  }
  return readBatchSequence(payload);
}

function readKeyframeTick(payload) {
  if (!payload || typeof payload !== "object") {
    return null;
  }
  const candidates = [
    payload.keyframeTick,
    payload.kfTick,
    payload.keyframe_t,
    payload.keyframe?.t,
  ];
  for (const candidate of candidates) {
    const tick = coerceTick(candidate);
    if (tick !== null) {
      return tick;
    }
  }
  return null;
}

function clonePendingRequests(requests) {
  if (!(requests instanceof Map)) {
    return new Map();
  }
  const cloned = new Map();
  for (const [key, value] of requests.entries()) {
    const seq = Number.isFinite(key) ? Math.floor(key) : null;
    if (seq === null || seq <= 0) {
      continue;
    }
    const attempts = Number.isFinite(value?.attempts)
      ? Math.max(0, Math.floor(value.attempts))
      : 0;
    const nextRetryAt = Number.isFinite(value?.nextRetryAt)
      ? Math.max(0, Math.floor(value.nextRetryAt))
      : null;
    const firstRequestedAt = Number.isFinite(value?.firstRequestedAt)
      ? Math.max(0, Math.floor(value.firstRequestedAt))
      : null;
    cloned.set(seq, {
      attempts,
      nextRetryAt,
      firstRequestedAt,
    });
  }
  return cloned;
}

function cloneResolvedSequences(sequences) {
  if (!(sequences instanceof Set)) {
    return new Set();
  }
  const cloned = new Set();
  for (const value of sequences.values()) {
    const seq = Number.isFinite(value) ? Math.floor(value) : null;
    if (seq === null || seq <= 0) {
      continue;
    }
    cloned.add(seq);
  }
  return cloned;
}

function clonePendingReplays(replays) {
  if (!Array.isArray(replays) || replays.length === 0) {
    return [];
  }
  const normalized = [];
  for (const entry of replays) {
    if (!entry || typeof entry !== "object") {
      continue;
    }
    const deferredCount = Number.isFinite(entry.deferredCount) && entry.deferredCount > 0
      ? Math.floor(entry.deferredCount)
      : 0;
    const deferredAt = Number.isFinite(entry.deferredAt) && entry.deferredAt >= 0
      ? Math.floor(entry.deferredAt)
      : null;
    normalized.push({
      ...entry,
      deferredCount,
      deferredAt,
    });
  }
  return normalized;
}

function cloneNackCounts(counts) {
  const result = {};
  if (!counts || typeof counts !== "object") {
    return result;
  }
  for (const [key, value] of Object.entries(counts)) {
    const normalizedKey = typeof key === "string" ? key : String(key);
    const numeric = Number(value);
    if (Number.isFinite(numeric) && numeric >= 0) {
      result[normalizedKey] = Math.floor(numeric);
    }
  }
  return result;
}

function incrementNackCount(counts, reason) {
  if (!reason) {
    return;
  }
  const key = String(reason);
  const current = Number(counts[key]);
  const nextValue = Number.isFinite(current) && current >= 0 ? current + 1 : 1;
  counts[key] = nextValue;
}

function createKeyframeCache(limit = DEFAULT_KEYFRAME_CACHE_LIMIT) {
  const normalizedLimit = Number.isFinite(limit) && limit > 0
    ? Math.floor(limit)
    : DEFAULT_KEYFRAME_CACHE_LIMIT;
  return {
    limit: normalizedLimit,
    map: new Map(),
  };
}

function cloneKeyframeCache(cache) {
  if (!cache || typeof cache !== "object") {
    return createKeyframeCache();
  }
  const limit = Number.isFinite(cache.limit) && cache.limit > 0
    ? Math.floor(cache.limit)
    : DEFAULT_KEYFRAME_CACHE_LIMIT;
  const map = cache.map instanceof Map ? new Map(cache.map) : new Map();
  return { limit, map };
}

function trimRecoveryLog(log, limit = 10) {
  if (!Array.isArray(log)) {
    return [];
  }
  if (!Number.isFinite(limit) || limit <= 0) {
    return log.slice();
  }
  if (log.length <= limit) {
    return log.slice();
  }
  return log.slice(log.length - limit);
}

function normalizePatchKind(value) {
  if (typeof value !== "string") {
    return null;
  }
  const trimmed = value.trim();
  if (trimmed.length === 0) {
    return null;
  }
  const lower = trimmed.toLowerCase();
  if (KNOWN_PATCH_KINDS.has(lower)) {
    return lower;
  }
  return trimmed;
}

function normalizeEntityId(value) {
  if (typeof value !== "string") {
    return null;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : null;
}

function normalizePatchEnvelope(raw) {
  if (!raw || typeof raw !== "object") {
    return null;
  }
  const kind = normalizePatchKind(raw.kind);
  const entityId = normalizeEntityId(raw.entityId);
  if (!kind || !entityId) {
    return null;
  }
  const payload = raw.payload && typeof raw.payload === "object" ? raw.payload : {};
  return { kind, entityId, payload };
}

function makePatchError(kind, entityId, message) {
  return {
    kind: typeof kind === "string" ? kind : null,
    entityId: typeof entityId === "string" ? entityId : null,
    message,
  };
}

function applyPlayerPosition(view, payload) {
  const x = toFiniteNumber(payload?.x, null);
  const y = toFiniteNumber(payload?.y, null);
  if (x === null || y === null) {
    return { applied: false, error: "invalid position payload" };
  }
  view.x = x;
  view.y = y;
  return { applied: true };
}

function applyPlayerFacing(view, payload) {
  if (!payload || typeof payload !== "object") {
    return { applied: false, error: "invalid facing payload" };
  }
  view.facing = normalizeFacingValue(payload.facing);
  return { applied: true };
}

function applyPlayerIntent(view, payload) {
  if (!payload || typeof payload !== "object") {
    return { applied: false, error: "invalid intent payload" };
  }
  const dx = toFiniteNumber(payload.dx, null);
  const dy = toFiniteNumber(payload.dy, null);
  if (dx === null || dy === null) {
    return { applied: false, error: "invalid intent payload" };
  }
  view.intentDX = dx;
  view.intentDY = dy;
  return { applied: true };
}

function applyPlayerHealth(view, payload) {
  if (!payload || typeof payload !== "object") {
    return { applied: false, error: "invalid health payload" };
  }
  const health = toFiniteNumber(payload.health, null);
  if (health === null) {
    return { applied: false, error: "invalid health payload" };
  }
  view.health = health;
  const maxHealth = toFiniteNumber(payload.maxHealth, null);
  if (maxHealth !== null && maxHealth > 0) {
    view.maxHealth = maxHealth;
  }
  return { applied: true };
}

function applyPlayerInventory(view, payload) {
  if (!payload || typeof payload !== "object") {
    return { applied: false, error: "invalid inventory payload" };
  }
  const slots = cloneInventorySlots(payload.slots);
  view.inventory = { slots };
  return { applied: true };
}

const applyNPCPosition = applyPlayerPosition;
const applyNPCFacing = applyPlayerFacing;
const applyNPCHealth = applyPlayerHealth;
const applyNPCInventory = applyPlayerInventory;
const applyGroundItemPosition = applyPlayerPosition;

function applyGroundItemQuantity(view, payload) {
  if (!payload || typeof payload !== "object") {
    return { applied: false, error: "invalid quantity payload" };
  }
  const qty = toFiniteInt(payload.qty, null);
  if (qty === null || qty < 0) {
    return { applied: false, error: "invalid quantity payload" };
  }
  view.qty = qty;
  return { applied: true };
}

const PATCH_HANDLERS = {
  [PATCH_KIND_PLAYER_POS]: { target: "players", apply: applyPlayerPosition },
  [PATCH_KIND_PLAYER_FACING]: { target: "players", apply: applyPlayerFacing },
  [PATCH_KIND_PLAYER_INTENT]: { target: "players", apply: applyPlayerIntent },
  [PATCH_KIND_PLAYER_HEALTH]: { target: "players", apply: applyPlayerHealth },
  [PATCH_KIND_PLAYER_INVENTORY]: { target: "players", apply: applyPlayerInventory },
  [PATCH_KIND_PLAYER_REMOVED]: { target: "players", remove: true },
  [PATCH_KIND_NPC_POS]: { target: "npcs", apply: applyNPCPosition },
  [PATCH_KIND_NPC_FACING]: { target: "npcs", apply: applyNPCFacing },
  [PATCH_KIND_NPC_HEALTH]: { target: "npcs", apply: applyNPCHealth },
  [PATCH_KIND_NPC_INVENTORY]: { target: "npcs", apply: applyNPCInventory },
  [PATCH_KIND_GROUND_ITEM_POS]: { target: "groundItems", apply: applyGroundItemPosition },
  [PATCH_KIND_GROUND_ITEM_QTY]: { target: "groundItems", apply: applyGroundItemQuantity },
};

function applyPatchesToSnapshot(baseSnapshot, patches, options = {}) {
  const basePlayers =
    baseSnapshot && typeof baseSnapshot === "object" ? baseSnapshot.players : null;
  const baseNPCs =
    baseSnapshot && typeof baseSnapshot === "object" ? baseSnapshot.npcs : null;
  const baseGroundItems =
    baseSnapshot && typeof baseSnapshot === "object"
      ? baseSnapshot.groundItems
      : null;

  const players = clonePlayersMap(basePlayers);
  const npcs = cloneNPCsMap(baseNPCs);
  const groundItems = cloneGroundItemsMap(baseGroundItems);

  const viewMaps = { players, npcs, groundItems };
  const errors = [];
  let appliedCount = 0;
  const history = options.history && typeof options.history === "object"
    ? options.history
    : null;
  const batchTick = coerceTick(options.batchTick);
  const batchSequence = coerceTick(options.batchSequence);

  if (!Array.isArray(patches) || patches.length === 0) {
    return { players, npcs, groundItems, errors, appliedCount };
  }

  for (const rawPatch of patches) {
    const patch = normalizePatchEnvelope(rawPatch);
    if (!patch) {
      errors.push(makePatchError(null, null, "invalid patch envelope"));
      continue;
    }
    const handlerEntry = PATCH_HANDLERS[patch.kind];
    if (!handlerEntry) {
      errors.push(
        makePatchError(
          patch.kind,
          patch.entityId,
          `unsupported patch kind: ${patch.kind}`,
        ),
      );
      continue;
    }
    const viewMap = viewMaps[handlerEntry.target];
    if (!viewMap) {
      errors.push(
        makePatchError(patch.kind, patch.entityId, "no target map for patch"),
      );
      continue;
    }
    const patchTick = readPatchSequence(rawPatch);
    const dedupeValue =
      patchTick !== null
        ? patchTick
        : batchSequence !== null
            ? batchSequence
            : batchTick;
    const dedupeKey = `${patch.kind}:${patch.entityId}`;
    const isDuplicate = shouldSkipPatch(history, dedupeKey, dedupeValue);
    if (handlerEntry.remove === true) {
      const existed = Object.prototype.hasOwnProperty.call(viewMap, patch.entityId);
      delete viewMap[patch.entityId];
      if (!isDuplicate) {
        rememberPatch(history, dedupeKey, dedupeValue);
        if (existed) {
          appliedCount += 1;
        }
      }
      continue;
    }
    const view = viewMap[patch.entityId];
    if (!view) {
      errors.push(
        makePatchError(patch.kind, patch.entityId, "unknown entity for patch"),
      );
      continue;
    }
    const result = handlerEntry.apply(view, patch.payload);
    if (!result || result.applied !== true) {
      const message = result?.error || "failed to apply patch";
      errors.push(makePatchError(patch.kind, patch.entityId, message));
      continue;
    }
    if (!isDuplicate) {
      appliedCount += 1;
      rememberPatch(history, dedupeKey, dedupeValue);
    }
  }

  return { players, npcs, groundItems, errors, appliedCount };
}

function trimErrors(errors, limit) {
  if (!Number.isFinite(limit) || limit <= 0) {
    return errors;
  }
  if (errors.length <= limit) {
    return errors;
  }
  return errors.slice(errors.length - limit);
}

export function createPatchState() {
  return {
    baseline: {
      tick: null,
      sequence: null,
      players: Object.create(null),
      npcs: Object.create(null),
      groundItems: Object.create(null),
    },
    patched: {
      tick: null,
      sequence: null,
      players: Object.create(null),
      npcs: Object.create(null),
      groundItems: Object.create(null),
    },
    lastAppliedPatchCount: 0,
    lastError: null,
    errors: [],
    lastUpdateSource: null,
    lastTick: null,
    lastSequence: null,
    patchHistory: createPatchHistory(),
    keyframes: createKeyframeCache(),
    pendingKeyframeRequests: new Map(),
    pendingReplays: [],
    resolvedKeyframeSequences: new Set(),
    lastRecovery: null,
    recoveryLog: [],
    keyframeNackCounts: {},
    resyncRequested: false,
    deferredPatchCount: 0,
    totalDeferredPatchCount: 0,
    lastDeferredReplayLatencyMs: null,
  };
}

export function updatePatchState(previousState, payload, options = {}) {
  const state = previousState && typeof previousState === "object"
    ? previousState
    : createPatchState();
  const source =
    typeof options.source === "string" && options.source.length > 0
      ? options.source
      : "snapshot";
  const errorLimit = Number.isFinite(options.errorLimit)
    ? Math.max(0, Math.floor(options.errorLimit))
    : DEFAULT_ERROR_LIMIT;
  const now = Number.isFinite(options.now) ? Math.floor(options.now) : Date.now();
  const keyframeLimit = Number.isFinite(options.keyframeLimit) && options.keyframeLimit > 0
    ? Math.floor(options.keyframeLimit)
    : DEFAULT_KEYFRAME_CACHE_LIMIT;
  const requestKeyframe = typeof options.requestKeyframe === "function"
    ? options.requestKeyframe
    : null;
  const replaying = options.replaying === true;

  const payloadResetFlag =
    payload && typeof payload === "object" && (
      payload.resync === true ||
      payload.patchReset === true ||
      payload.full === true ||
      payload.reset === true
    );
  const shouldResetHistory =
    options.resetHistory === true || source === "join" || payloadResetFlag;

  const previousHistory = state.patchHistory;
  const history = shouldResetHistory
    ? createPatchHistory(previousHistory?.limit)
    : clonePatchHistory(previousHistory);

  const baselineFromPayload = buildBaselineFromSnapshot(payload || {});
  let patchList = Array.isArray(payload?.patches) ? payload.patches : [];
  const previousTick = Number.isFinite(state.lastTick) && state.lastTick >= 0
    ? Math.floor(state.lastTick)
    : null;
  const previousSequence = Number.isFinite(state.lastSequence) && state.lastSequence >= 0
    ? Math.floor(state.lastSequence)
    : null;

  const keyframes = cloneKeyframeCache(state.keyframes);
  keyframes.limit = keyframeLimit;
  const pendingRequests = clonePendingRequests(state.pendingKeyframeRequests);
  const pendingReplays = clonePendingReplays(state.pendingReplays);
  const resolvedSequences = cloneResolvedSequences(state.resolvedKeyframeSequences);
  const historyEntries = Array.isArray(state.errors) ? state.errors.slice() : [];
  const recoveryLog = Array.isArray(state.recoveryLog) ? state.recoveryLog.slice() : [];
  const nackCounts = cloneNackCounts(state.keyframeNackCounts);
  const previousLastError = state.lastError && typeof state.lastError === "object"
    ? state.lastError
    : null;
  let lastError = previousLastError;
  let lastRecovery = state.lastRecovery && typeof state.lastRecovery === "object"
    ? { ...state.lastRecovery }
    : null;
  let resyncRequested = false;
  const previousTotalDeferred = Number.isFinite(state.totalDeferredPatchCount) && state.totalDeferredPatchCount >= 0
    ? Math.floor(state.totalDeferredPatchCount)
    : 0;
  const previousDeferredLatency = Number.isFinite(state.lastDeferredReplayLatencyMs) &&
      state.lastDeferredReplayLatencyMs >= 0
    ? Math.floor(state.lastDeferredReplayLatencyMs)
    : null;
  let deferredPatchCountForUpdate = 0;
  let totalDeferredPatches = previousTotalDeferred;
  let deferredReplayLatencyMs = previousDeferredLatency;

  const keyframeSeq = readKeyframeSequence(payload);
  const keyframeTick = readKeyframeTick(payload);
  const hasSnapshot = hasExplicitEntityArrays(payload);
  const messageType = typeof payload?.type === "string" ? payload.type : null;

  let baseline = baselineFromPayload;
  if (hasSnapshot && keyframeSeq !== null) {
    baseline.sequence = keyframeSeq;
  } else if (baseline.sequence === null && keyframeSeq !== null) {
    baseline.sequence = keyframeSeq;
  }
  if (baseline.tick === null && keyframeTick !== null) {
    baseline.tick = keyframeTick;
  }

  const normalizedKeyframeSeq =
    typeof keyframeSeq === "number" && Number.isFinite(keyframeSeq)
      ? Math.floor(keyframeSeq)
      : null;

  if (
    hasSnapshot &&
    messageType === "keyframe" &&
    normalizedKeyframeSeq !== null &&
    resolvedSequences.has(normalizedKeyframeSeq)
  ) {
    const trimmedErrors = trimErrors(historyEntries, errorLimit);
    const trimmedRecoveries = trimRecoveryLog(recoveryLog);
    return {
      baseline: state.baseline,
      patched: state.patched,
      lastAppliedPatchCount: state.lastAppliedPatchCount,
      lastError,
      errors: trimmedErrors,
      lastUpdateSource: state.lastUpdateSource,
      lastTick: previousTick,
      lastSequence: previousSequence,
      patchHistory: history,
      keyframes,
      pendingKeyframeRequests: pendingRequests,
      pendingReplays,
      lastRecovery,
      recoveryLog: trimmedRecoveries,
      keyframeNackCounts: nackCounts,
      resyncRequested,
      resolvedKeyframeSequences: resolvedSequences,
      deferredPatchCount: deferredPatchCountForUpdate,
      totalDeferredPatchCount: totalDeferredPatches,
      lastDeferredReplayLatencyMs: deferredReplayLatencyMs,
    };
  }

  if (hasSnapshot) {
    rememberKeyframeSnapshot(keyframes, baseline);
    if (baseline.sequence !== null && pendingRequests.has(baseline.sequence)) {
      const pendingEntry = pendingRequests.get(baseline.sequence);
      pendingRequests.delete(baseline.sequence);
      const requestedAt = Number.isFinite(pendingEntry?.firstRequestedAt)
        ? Math.floor(pendingEntry.firstRequestedAt)
        : lastRecovery && lastRecovery.sequence === baseline.sequence
          ? lastRecovery.requestedAt
          : null;
      const resolvedEntry = {
        sequence: baseline.sequence,
        tick: baseline.tick,
        status: "recovered",
        requestedAt,
        resolvedAt: now,
        latencyMs:
          typeof requestedAt === "number" && Number.isFinite(requestedAt)
            ? Math.max(0, now - requestedAt)
            : null,
      };
      lastRecovery = resolvedEntry;
      recoveryLog.push({ ...resolvedEntry });
    }
    if (messageType === "keyframe" && baseline.sequence !== null) {
      resolvedSequences.add(baseline.sequence);
      while (resolvedSequences.size > RESOLVED_SEQUENCE_LIMIT) {
        const oldest = resolvedSequences.values().next().value;
        if (oldest === undefined) {
          break;
        }
        resolvedSequences.delete(oldest);
      }
    }
  } else if (messageType === "keyframeNack" && normalizedKeyframeSeq !== null) {
    const reasonValue = typeof payload?.reason === "string" ? payload.reason.trim().toLowerCase() : "";
    const normalizedReason = reasonValue || "nack";
    incrementNackCount(nackCounts, normalizedReason);

    const replayIndex = pendingReplays.findIndex((entry) => entry && entry.sequence === normalizedKeyframeSeq);
    const replayEntry = replayIndex !== -1 ? pendingReplays[replayIndex] : null;
    const pendingEntry = pendingRequests.get(normalizedKeyframeSeq) || null;

    if (normalizedReason === "expired") {
      pendingRequests.delete(normalizedKeyframeSeq);
      if (replayIndex !== -1) {
        pendingReplays.splice(replayIndex, 1);
      }
      resyncRequested = true;
    } else if (normalizedReason === "rate_limited") {
      if (pendingEntry) {
        const attempts = Math.max(1, Math.floor(pendingEntry.attempts ?? 1));
        if (attempts < KEYFRAME_RECOVERY_MAX_ATTEMPTS) {
          const delay = Math.min(
            KEYFRAME_RECOVERY_MAX_STEP,
            Math.max(
              KEYFRAME_RECOVERY_BASE_DELAY,
              KEYFRAME_RECOVERY_BASE_DELAY * Math.pow(2, attempts - 1),
            ),
          );
          pendingRequests.set(normalizedKeyframeSeq, {
            attempts,
            nextRetryAt: now + delay,
            firstRequestedAt: Number.isFinite(pendingEntry.firstRequestedAt)
              ? Math.floor(pendingEntry.firstRequestedAt)
              : now,
          });
        } else {
          pendingRequests.delete(normalizedKeyframeSeq);
          if (replayIndex !== -1) {
            pendingReplays.splice(replayIndex, 1);
          }
          resyncRequested = true;
        }
      }
      if (replayEntry) {
        replayEntry.lastRateLimitedAt = now;
      }
    } else {
      pendingRequests.delete(normalizedKeyframeSeq);
    }

    const requestTimestamp = Number.isFinite(pendingEntry?.firstRequestedAt)
      ? Math.floor(pendingEntry.firstRequestedAt)
      : lastRecovery && lastRecovery.sequence === normalizedKeyframeSeq
        ? lastRecovery.requestedAt
        : null;

    const recoveryEntry = {
      sequence: normalizedKeyframeSeq,
      tick: keyframeTick ?? baseline.tick ?? null,
      status: normalizedReason,
      requestedAt: requestTimestamp,
      resolvedAt: now,
      latencyMs:
        (normalizedReason === "expired" || resyncRequested) &&
        typeof requestTimestamp === "number" &&
        Number.isFinite(requestTimestamp)
          ? Math.max(0, now - requestTimestamp)
          : null,
    };
    lastRecovery = recoveryEntry;
    recoveryLog.push({ ...recoveryEntry });

    if (normalizedReason === "expired" || resyncRequested) {
      const trimmedErrors = trimErrors(historyEntries, errorLimit);
      const trimmedRecoveries = trimRecoveryLog(recoveryLog);
      return {
        baseline: state.baseline,
        patched: state.patched,
        lastAppliedPatchCount: 0,
        lastError,
        errors: trimmedErrors,
        lastUpdateSource: source,
        lastTick: previousTick,
        lastSequence: previousSequence,
        patchHistory: history,
        keyframes,
        pendingKeyframeRequests: pendingRequests,
        pendingReplays,
        lastRecovery,
        recoveryLog: trimmedRecoveries,
        keyframeNackCounts: nackCounts,
        resyncRequested,
        resolvedKeyframeSequences: resolvedSequences,
        deferredPatchCount: deferredPatchCountForUpdate,
        totalDeferredPatchCount: totalDeferredPatches,
        lastDeferredReplayLatencyMs: deferredReplayLatencyMs,
      };
    }

    const trimmedErrors = trimErrors(historyEntries, errorLimit);
    const trimmedRecoveries = trimRecoveryLog(recoveryLog);
    return {
      baseline: state.baseline,
      patched: state.patched,
      lastAppliedPatchCount: 0,
      lastError,
      errors: trimmedErrors,
      lastUpdateSource: source,
      lastTick: previousTick,
      lastSequence: previousSequence,
      patchHistory: history,
      keyframes,
      pendingKeyframeRequests: pendingRequests,
      pendingReplays,
      lastRecovery,
      recoveryLog: trimmedRecoveries,
      keyframeNackCounts: nackCounts,
      resyncRequested,
      resolvedKeyframeSequences: resolvedSequences,
      deferredPatchCount: deferredPatchCountForUpdate,
      totalDeferredPatchCount: totalDeferredPatches,
      lastDeferredReplayLatencyMs: deferredReplayLatencyMs,
    };
  } else if (normalizedKeyframeSeq !== null) {
    const cached = getCachedKeyframe(keyframes, keyframeSeq);
    if (cached) {
      const patchSequence = baseline.sequence;
      const patchTick = baseline.tick;
      const cumulative = cloneBaselineSnapshot(state.baseline);
      if (cumulative) {
        const mergeMaps = (target, source, cloneEntry) => {
          if (!target || typeof target !== "object") {
            return;
          }
          if (!source || typeof source !== "object") {
            return;
          }
          for (const entry of Object.values(source)) {
            const cloned = cloneEntry(entry);
            if (!cloned || !cloned.id) {
              continue;
            }
            if (!Object.hasOwn(target, cloned.id)) {
              target[cloned.id] = cloned;
            }
          }
        };
        mergeMaps(cumulative.players, cached.players, clonePlayerView);
        mergeMaps(cumulative.npcs, cached.npcs, cloneNPCView);
        mergeMaps(cumulative.groundItems, cached.groundItems, cloneGroundItemView);
        baseline = cumulative;
      } else {
        baseline = cached;
      }
      if (patchSequence !== null && patchSequence !== undefined) {
        baseline.sequence = patchSequence;
      }
      if (patchTick !== null && patchTick !== undefined) {
        const normalizedTick = coerceTick(patchTick);
        if (normalizedTick !== null) {
          if (
            baseline.tick === null ||
            baseline.tick === undefined ||
            !Number.isFinite(baseline.tick) ||
            baseline.tick < normalizedTick
          ) {
            baseline.tick = normalizedTick;
          }
        }
      }
    } else if (!replaying) {
      const patchedFallback = cloneBaselineSnapshot(state.patched);
      const baselineFallback = patchedFallback || cloneBaselineSnapshot(state.baseline);
      const fallbackBaseline = baselineFallback;
      if (!pendingRequests.has(normalizedKeyframeSeq)) {
        pendingRequests.set(normalizedKeyframeSeq, {
          attempts: 1,
          nextRetryAt: null,
          firstRequestedAt: now,
        });
        if (requestKeyframe) {
          try {
            requestKeyframe(normalizedKeyframeSeq, keyframeTick ?? baseline.tick ?? null, {
              incrementAttempt: false,
              firstRequestedAt: now,
            });
          } catch (err) {
            // Ignore transport errors from diagnostics helpers.
          }
        }
        const recoveryEntry = {
          sequence: normalizedKeyframeSeq,
          tick: keyframeTick ?? baseline.tick ?? null,
          status: "requested",
          requestedAt: now,
          resolvedAt: null,
          latencyMs: null,
        };
        lastRecovery = recoveryEntry;
        recoveryLog.push({ ...recoveryEntry });
      }
      const alreadyPendingReplay = pendingReplays.some(
        (entry) => entry && entry.sequence === normalizedKeyframeSeq,
      );
      if (!alreadyPendingReplay) {
        pendingReplays.push({
          sequence: normalizedKeyframeSeq,
          payload,
          source,
          keyframeTick: keyframeTick ?? baseline.tick ?? null,
          requests: 1,
          lastRateLimitedAt: null,
          deferredCount: 0,
          deferredAt: null,
        });
      }
      if (fallbackBaseline) {
        const fallbackPlayers = fallbackBaseline.players || Object.create(null);
        const fallbackNPCs = fallbackBaseline.npcs || Object.create(null);
        const fallbackGroundItems = fallbackBaseline.groundItems || Object.create(null);
        const fallbackViewMaps = {
          players: fallbackPlayers,
          npcs: fallbackNPCs,
          groundItems: fallbackGroundItems,
        };

        const replayablePatches = [];
        const deferredPatches = [];
        for (const rawPatch of patchList) {
          const envelope = normalizePatchEnvelope(rawPatch);
          if (!envelope) {
            replayablePatches.push(rawPatch);
            continue;
          }
          const handlerEntry = PATCH_HANDLERS[envelope.kind];
          if (!handlerEntry) {
            replayablePatches.push(rawPatch);
            continue;
          }
          const viewMap = fallbackViewMaps[handlerEntry.target];
          const entityKnown =
            viewMap && typeof viewMap === "object"
              ? Object.hasOwn(viewMap, envelope.entityId)
              : false;
          if (entityKnown) {
            replayablePatches.push(rawPatch);
          } else {
            deferredPatches.push(rawPatch);
          }
        }

        if (deferredPatches.length > 0) {
          deferredPatchCountForUpdate += deferredPatches.length;
          totalDeferredPatches += deferredPatches.length;
          const replayEntryIndex = pendingReplays.findIndex(
            (entry) => entry && entry.sequence === normalizedKeyframeSeq,
          );
          if (replayEntryIndex !== -1) {
            const replayEntry = pendingReplays[replayEntryIndex];
            replayEntry.deferredCount = deferredPatches.length;
            replayEntry.deferredAt = now;
          }
        }

        patchList = replayablePatches;

        if (previousTick !== null) {
          if (
            fallbackBaseline.tick === null ||
            !Number.isFinite(fallbackBaseline.tick) ||
            fallbackBaseline.tick < previousTick
          ) {
            fallbackBaseline.tick = previousTick;
          } else {
            fallbackBaseline.tick = Math.floor(fallbackBaseline.tick);
          }
        }
        if (previousSequence !== null) {
          if (
            fallbackBaseline.sequence === null ||
            !Number.isFinite(fallbackBaseline.sequence) ||
            fallbackBaseline.sequence < previousSequence
          ) {
            fallbackBaseline.sequence = previousSequence;
          } else {
            fallbackBaseline.sequence = Math.floor(fallbackBaseline.sequence);
          }
        }

        baseline = fallbackBaseline;
      } else {
        const trimmedErrors = trimErrors(historyEntries, errorLimit);
        const trimmedRecoveries = trimRecoveryLog(recoveryLog);
        return {
          baseline: state.baseline,
          patched: state.patched,
          lastAppliedPatchCount: 0,
          lastError,
          errors: trimmedErrors,
          lastUpdateSource: source,
          lastTick: previousTick,
          lastSequence: previousSequence,
          patchHistory: history,
          keyframes,
          pendingKeyframeRequests: pendingRequests,
          pendingReplays,
          lastRecovery,
          recoveryLog: trimmedRecoveries,
          keyframeNackCounts: nackCounts,
          resyncRequested,
          resolvedKeyframeSequences: resolvedSequences,
          deferredPatchCount: deferredPatchCountForUpdate,
          totalDeferredPatchCount: totalDeferredPatches,
          lastDeferredReplayLatencyMs: deferredReplayLatencyMs,
        };
      }
    }
  }

  if (hasSnapshot && keyframeSeq !== null) {
    const replayIndex = pendingReplays.findIndex((entry) => entry.sequence === keyframeSeq);
    if (replayIndex !== -1) {
      const replayEntry = pendingReplays.splice(replayIndex, 1)[0];
      if (
        replayEntry &&
        Number.isFinite(replayEntry.deferredCount) &&
        replayEntry.deferredCount > 0 &&
        Number.isFinite(replayEntry.deferredAt) &&
        replayEntry.deferredAt >= 0
      ) {
        const latency = Math.max(0, now - Math.floor(replayEntry.deferredAt));
        deferredReplayLatencyMs = latency;
      }
      const replayState = {
        ...state,
        patchHistory: history,
        keyframes,
        pendingKeyframeRequests: pendingRequests,
        pendingReplays,
        lastRecovery,
        recoveryLog,
        errors: historyEntries,
        lastError,
        resolvedKeyframeSequences: resolvedSequences,
        deferredPatchCount: deferredPatchCountForUpdate,
        totalDeferredPatchCount: totalDeferredPatches,
        lastDeferredReplayLatencyMs: deferredReplayLatencyMs,
      };
      return updatePatchState(replayState, replayEntry.payload, {
        ...options,
        source: replayEntry.source || replayEntry.payload?.type || source,
        replaying: true,
        keyframeLimit,
        now,
      });
    }
  }

  const nextTick = baseline.tick;
  const nextSequence = baseline.sequence;

  const appendError = (kind, entityId, message, tickValue, sequenceValue) => {
    const entry = {
      kind: typeof kind === "string" ? kind : null,
      entityId: typeof entityId === "string" ? entityId : null,
      message,
      tick: tickValue,
      sequence: sequenceValue,
      source,
    };
    historyEntries.push(entry);
    lastError = entry;
    return entry;
  };

  if (!shouldResetHistory && previousTick !== null && nextTick !== null && nextTick < previousTick) {
    appendError(null, null, `out-of-order patch tick ${nextTick} < ${previousTick}`, nextTick, nextSequence);
    const trimmedErrors = trimErrors(historyEntries, errorLimit);
    const trimmedRecoveries = trimRecoveryLog(recoveryLog);
    return {
      baseline: state.baseline,
      patched: state.patched,
      lastAppliedPatchCount: 0,
      lastError,
      errors: trimmedErrors,
      lastUpdateSource: source,
      lastTick: previousTick,
      lastSequence: previousSequence,
      patchHistory: history,
      keyframes,
      pendingKeyframeRequests: pendingRequests,
      pendingReplays,
      lastRecovery,
      recoveryLog: trimmedRecoveries,
      keyframeNackCounts: nackCounts,
      resyncRequested,
      resolvedKeyframeSequences: resolvedSequences,
      deferredPatchCount: deferredPatchCountForUpdate,
      totalDeferredPatchCount: totalDeferredPatches,
      lastDeferredReplayLatencyMs: deferredReplayLatencyMs,
    };
  }

  if (!shouldResetHistory && previousSequence !== null && nextSequence !== null && nextSequence < previousSequence) {
    appendError(
      null,
      null,
      `out-of-order patch sequence ${nextSequence} < ${previousSequence}`,
      nextTick,
      nextSequence,
    );
    const trimmedErrors = trimErrors(historyEntries, errorLimit);
    const trimmedRecoveries = trimRecoveryLog(recoveryLog);
    return {
      baseline: state.baseline,
      patched: state.patched,
      lastAppliedPatchCount: 0,
      lastError,
      errors: trimmedErrors,
      lastUpdateSource: source,
      lastTick: previousTick,
      lastSequence: previousSequence,
      patchHistory: history,
      keyframes,
      pendingKeyframeRequests: pendingRequests,
      pendingReplays,
      lastRecovery,
      recoveryLog: trimmedRecoveries,
      keyframeNackCounts: nackCounts,
      resyncRequested,
      deferredPatchCount: deferredPatchCountForUpdate,
      totalDeferredPatchCount: totalDeferredPatches,
      lastDeferredReplayLatencyMs: deferredReplayLatencyMs,
    };
  }

  const patchResult = applyPatchesToSnapshot(baseline, patchList, {
    history,
    batchTick: nextTick,
    batchSequence: nextSequence,
  });

  if (patchResult.errors.length > 0) {
    for (const error of patchResult.errors) {
      appendError(error.kind, error.entityId, error.message, baseline.tick, baseline.sequence);
    }
  }

  const trimmedErrors = trimErrors(historyEntries, errorLimit);
  const trimmedRecoveries = trimRecoveryLog(recoveryLog);
  const resolvedTick = nextTick !== null ? nextTick : previousTick;
  const resolvedSequence = nextSequence !== null ? nextSequence : previousSequence;

  baseline.players = clonePlayersMap(patchResult.players);
  baseline.npcs = cloneNPCsMap(patchResult.npcs);
  baseline.groundItems = cloneGroundItemsMap(patchResult.groundItems);
  if (resolvedTick !== null) {
    baseline.tick = resolvedTick;
  }
  if (resolvedSequence !== null) {
    baseline.sequence = resolvedSequence;
  }

  return {
    baseline: {
      tick: baseline.tick,
      sequence: baseline.sequence,
      players: baseline.players,
      npcs: baseline.npcs,
      groundItems: baseline.groundItems,
    },
    patched: {
      tick: baseline.tick,
      sequence: baseline.sequence,
      players: patchResult.players,
      npcs: patchResult.npcs,
      groundItems: patchResult.groundItems,
    },
    lastAppliedPatchCount: patchResult.appliedCount,
    lastError,
    errors: trimmedErrors,
    lastUpdateSource: source,
    lastTick: resolvedTick,
    lastSequence: resolvedSequence,
    patchHistory: history,
    keyframes,
    pendingKeyframeRequests: pendingRequests,
    pendingReplays,
    lastRecovery,
    recoveryLog: trimmedRecoveries,
    keyframeNackCounts: nackCounts,
    resyncRequested,
    resolvedKeyframeSequences: resolvedSequences,
    deferredPatchCount: deferredPatchCountForUpdate,
    totalDeferredPatchCount: totalDeferredPatches,
    lastDeferredReplayLatencyMs: deferredReplayLatencyMs,
  };
}

export {
  PATCH_KIND_PLAYER_POS,
  PATCH_KIND_PLAYER_FACING,
  PATCH_KIND_PLAYER_INTENT,
  PATCH_KIND_PLAYER_HEALTH,
  PATCH_KIND_PLAYER_INVENTORY,
  PATCH_KIND_PLAYER_REMOVED,
  PATCH_KIND_NPC_POS,
  PATCH_KIND_NPC_FACING,
  PATCH_KIND_NPC_HEALTH,
  PATCH_KIND_NPC_INVENTORY,
  PATCH_KIND_GROUND_ITEM_POS,
  PATCH_KIND_GROUND_ITEM_QTY,
  applyPatchesToSnapshot,
  applyPatchesToSnapshot as applyPatchesToPlayers,
  buildBaselineFromSnapshot,
};
