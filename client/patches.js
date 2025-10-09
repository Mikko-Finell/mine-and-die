const PATCH_KIND_PLAYER_POS = "player_pos";
const PATCH_KIND_PLAYER_FACING = "player_facing";
const PATCH_KIND_PLAYER_INTENT = "player_intent";
const PATCH_KIND_PLAYER_HEALTH = "player_health";
const PATCH_KIND_PLAYER_INVENTORY = "player_inventory";
const PATCH_KIND_NPC_POS = "npc_pos";
const PATCH_KIND_NPC_FACING = "npc_facing";
const PATCH_KIND_NPC_HEALTH = "npc_health";
const PATCH_KIND_NPC_INVENTORY = "npc_inventory";
const PATCH_KIND_EFFECT_POS = "effect_pos";
const PATCH_KIND_EFFECT_PARAMS = "effect_params";
const PATCH_KIND_GROUND_ITEM_POS = "ground_item_pos";
const PATCH_KIND_GROUND_ITEM_QTY = "ground_item_qty";

const VALID_FACINGS = new Set(["up", "down", "left", "right"]);
const DEFAULT_FACING = "down";
const DEFAULT_ERROR_LIMIT = 20;
const DEFAULT_PATCH_HISTORY_LIMIT = 128;

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
    cloned.push({
      slot: slotIndex,
      item: { type, quantity },
    });
  }
  return cloned;
}

function cloneEffectParams(params) {
  if (!params || typeof params !== "object") {
    return {};
  }
  const cloned = {};
  for (const [key, value] of Object.entries(params)) {
    if (typeof key !== "string" || key.length === 0) {
      continue;
    }
    const numeric = toFiniteNumber(value, null);
    if (numeric === null) {
      continue;
    }
    cloned[key] = numeric;
  }
  return cloned;
}

function createPlayerView(player) {
  if (!player || typeof player !== "object") {
    return null;
  }
  const id = typeof player.id === "string" ? player.id : null;
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
  return {
    id: typeof view.id === "string" ? view.id : null,
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
  for (const [id, view] of Object.entries(source)) {
    const cloned = clonePlayerView(view);
    if (!cloned || !cloned.id) {
      continue;
    }
    next[id] = cloned;
  }
  return next;
}

function createNPCView(npc) {
  if (!npc || typeof npc !== "object") {
    return null;
  }
  const id = typeof npc.id === "string" ? npc.id : null;
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
  return {
    id: typeof view.id === "string" ? view.id : null,
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
  for (const [id, view] of Object.entries(source)) {
    const cloned = cloneNPCView(view);
    if (!cloned || !cloned.id) {
      continue;
    }
    next[id] = cloned;
  }
  return next;
}

function createEffectView(effect) {
  if (!effect || typeof effect !== "object") {
    return null;
  }
  const id = typeof effect.id === "string" ? effect.id : null;
  if (!id) {
    return null;
  }
  return {
    id,
    type: typeof effect.type === "string" ? effect.type : "",
    owner: typeof effect.owner === "string" ? effect.owner : "",
    start: toFiniteInt(effect.start, 0),
    duration: toFiniteInt(effect.duration, 0),
    x: toFiniteNumber(effect.x, 0),
    y: toFiniteNumber(effect.y, 0),
    width: toFiniteNumber(effect.width, 0),
    height: toFiniteNumber(effect.height, 0),
    params: cloneEffectParams(effect.params),
  };
}

function cloneEffectView(view) {
  if (!view || typeof view !== "object") {
    return null;
  }
  return {
    id: typeof view.id === "string" ? view.id : null,
    type: typeof view.type === "string" ? view.type : "",
    owner: typeof view.owner === "string" ? view.owner : "",
    start: toFiniteInt(view.start, 0),
    duration: toFiniteInt(view.duration, 0),
    x: toFiniteNumber(view.x, 0),
    y: toFiniteNumber(view.y, 0),
    width: toFiniteNumber(view.width, 0),
    height: toFiniteNumber(view.height, 0),
    params: cloneEffectParams(view.params),
  };
}

function cloneEffectsMap(source) {
  const next = Object.create(null);
  if (!source || typeof source !== "object") {
    return next;
  }
  for (const [id, view] of Object.entries(source)) {
    const cloned = cloneEffectView(view);
    if (!cloned || !cloned.id) {
      continue;
    }
    next[id] = cloned;
  }
  return next;
}

function createGroundItemView(item) {
  if (!item || typeof item !== "object") {
    return null;
  }
  const id = typeof item.id === "string" ? item.id : null;
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
  return {
    id: typeof view.id === "string" ? view.id : null,
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
  for (const [id, view] of Object.entries(source)) {
    const cloned = cloneGroundItemView(view);
    if (!cloned || !cloned.id) {
      continue;
    }
    next[id] = cloned;
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
  const effects = Object.create(null);
  if (Array.isArray(payload?.effects)) {
    for (const entry of payload.effects) {
      const view = createEffectView(entry);
      if (!view) {
        continue;
      }
      effects[view.id] = view;
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
  return {
    tick,
    players,
    npcs,
    effects,
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

function readPatchSequence(patch) {
  if (!patch || typeof patch !== "object") {
    return null;
  }
  const payload = patch.payload && typeof patch.payload === "object" ? patch.payload : null;
  const candidates = [
    patch.sequence,
    patch.seq,
    patch.tick,
    patch.t,
    payload?.sequence,
    payload?.seq,
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

function normalizePatchEnvelope(raw) {
  if (!raw || typeof raw !== "object") {
    return null;
  }
  const kind = typeof raw.kind === "string" ? raw.kind : null;
  const entityId = typeof raw.entityId === "string" ? raw.entityId : null;
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
const applyEffectPosition = applyPlayerPosition;

function applyEffectParams(view, payload) {
  if (!payload || typeof payload !== "object") {
    return { applied: false, error: "invalid effect params payload" };
  }
  const params = cloneEffectParams(payload.params);
  view.params = params;
  return { applied: true };
}

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
  [PATCH_KIND_NPC_POS]: { target: "npcs", apply: applyNPCPosition },
  [PATCH_KIND_NPC_FACING]: { target: "npcs", apply: applyNPCFacing },
  [PATCH_KIND_NPC_HEALTH]: { target: "npcs", apply: applyNPCHealth },
  [PATCH_KIND_NPC_INVENTORY]: { target: "npcs", apply: applyNPCInventory },
  [PATCH_KIND_EFFECT_POS]: { target: "effects", apply: applyEffectPosition },
  [PATCH_KIND_EFFECT_PARAMS]: { target: "effects", apply: applyEffectParams },
  [PATCH_KIND_GROUND_ITEM_POS]: { target: "groundItems", apply: applyGroundItemPosition },
  [PATCH_KIND_GROUND_ITEM_QTY]: { target: "groundItems", apply: applyGroundItemQuantity },
};

function applyPatchesToSnapshot(baseSnapshot, patches, options = {}) {
  const basePlayers =
    baseSnapshot && typeof baseSnapshot === "object" ? baseSnapshot.players : null;
  const baseNPCs =
    baseSnapshot && typeof baseSnapshot === "object" ? baseSnapshot.npcs : null;
  const baseEffects =
    baseSnapshot && typeof baseSnapshot === "object" ? baseSnapshot.effects : null;
  const baseGroundItems =
    baseSnapshot && typeof baseSnapshot === "object"
      ? baseSnapshot.groundItems
      : null;

  const players = clonePlayersMap(basePlayers);
  const npcs = cloneNPCsMap(baseNPCs);
  const effects = cloneEffectsMap(baseEffects);
  const groundItems = cloneGroundItemsMap(baseGroundItems);

  const viewMaps = { players, npcs, effects, groundItems };
  const errors = [];
  let appliedCount = 0;
  const history = options.history && typeof options.history === "object"
    ? options.history
    : null;
  const batchTick = coerceTick(options.batchTick);

  if (!Array.isArray(patches) || patches.length === 0) {
    return { players, npcs, effects, groundItems, errors, appliedCount };
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
    const view = viewMap[patch.entityId];
    if (!view) {
      errors.push(
        makePatchError(patch.kind, patch.entityId, "unknown entity for patch"),
      );
      continue;
    }
    const patchTick = readPatchSequence(rawPatch);
    const dedupeTick = patchTick !== null ? patchTick : batchTick;
    const dedupeKey = `${patch.kind}:${patch.entityId}`;
    const isDuplicate = shouldSkipPatch(history, dedupeKey, dedupeTick);
    const result = handlerEntry.apply(view, patch.payload);
    if (!result || result.applied !== true) {
      const message = result?.error || "failed to apply patch";
      errors.push(makePatchError(patch.kind, patch.entityId, message));
      continue;
    }
    if (!isDuplicate) {
      appliedCount += 1;
      rememberPatch(history, dedupeKey, dedupeTick);
    }
  }

  return { players, npcs, effects, groundItems, errors, appliedCount };
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
      players: Object.create(null),
      npcs: Object.create(null),
      effects: Object.create(null),
      groundItems: Object.create(null),
    },
    patched: {
      tick: null,
      players: Object.create(null),
      npcs: Object.create(null),
      effects: Object.create(null),
      groundItems: Object.create(null),
    },
    lastAppliedPatchCount: 0,
    lastError: null,
    errors: [],
    lastUpdateSource: null,
    lastTick: null,
    patchHistory: createPatchHistory(),
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

  const baseline = buildBaselineFromSnapshot(payload || {});
  const patchList = Array.isArray(payload?.patches) ? payload.patches : [];
  const previousTick = Number.isFinite(state.lastTick) && state.lastTick >= 0
    ? Math.floor(state.lastTick)
    : null;
  const nextTick = baseline.tick;

  const historyEntries = Array.isArray(state.errors) ? state.errors.slice() : [];
  let lastError = null;
  const appendError = (kind, entityId, message, tickValue) => {
    const entry = {
      kind: typeof kind === "string" ? kind : null,
      entityId: typeof entityId === "string" ? entityId : null,
      message,
      tick: tickValue,
      source,
    };
    historyEntries.push(entry);
    lastError = entry;
    return entry;
  };

  if (!shouldResetHistory && previousTick !== null && nextTick !== null && nextTick < previousTick) {
    appendError(null, null, `out-of-order patch tick ${nextTick} < ${previousTick}`);
    const trimmedErrors = trimErrors(historyEntries, errorLimit);
    return {
      baseline: state.baseline,
      patched: state.patched,
      lastAppliedPatchCount: 0,
      lastError,
      errors: trimmedErrors,
      lastUpdateSource: source,
      lastTick: previousTick,
      patchHistory: history,
    };
  }

  const patchResult = applyPatchesToSnapshot(baseline, patchList, {
    history,
    batchTick: nextTick,
  });

  if (patchResult.errors.length > 0) {
    for (const error of patchResult.errors) {
      appendError(error.kind, error.entityId, error.message, baseline.tick);
    }
  }

  const trimmedErrors = trimErrors(historyEntries, errorLimit);
  const resolvedTick = nextTick !== null ? nextTick : previousTick;

  return {
    baseline: {
      tick: baseline.tick,
      players: baseline.players,
      npcs: baseline.npcs,
      effects: baseline.effects,
      groundItems: baseline.groundItems,
    },
    patched: {
      tick: baseline.tick,
      players: patchResult.players,
      npcs: patchResult.npcs,
      effects: patchResult.effects,
      groundItems: patchResult.groundItems,
    },
    lastAppliedPatchCount: patchResult.appliedCount,
    lastError,
    errors: trimmedErrors,
    lastUpdateSource: source,
    lastTick: resolvedTick,
    patchHistory: history,
  };
}

export {
  PATCH_KIND_PLAYER_POS,
  PATCH_KIND_PLAYER_FACING,
  PATCH_KIND_PLAYER_INTENT,
  PATCH_KIND_PLAYER_HEALTH,
  PATCH_KIND_PLAYER_INVENTORY,
  PATCH_KIND_NPC_POS,
  PATCH_KIND_NPC_FACING,
  PATCH_KIND_NPC_HEALTH,
  PATCH_KIND_NPC_INVENTORY,
  PATCH_KIND_EFFECT_POS,
  PATCH_KIND_EFFECT_PARAMS,
  PATCH_KIND_GROUND_ITEM_POS,
  PATCH_KIND_GROUND_ITEM_QTY,
  applyPatchesToSnapshot,
  applyPatchesToSnapshot as applyPatchesToPlayers,
  buildBaselineFromSnapshot,
};
