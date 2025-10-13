function isFiniteNumber(value) {
  return typeof value === "number" && Number.isFinite(value);
}

function normalizeIdentifier(value) {
  if (typeof value !== "string") {
    return null;
  }
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : null;
}

function normalizeSequence(value) {
  if (isFiniteNumber(value)) {
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

function normalizeTick(value) {
  if (!isFiniteNumber(value)) {
    return null;
  }
  const normalized = Math.floor(value);
  return normalized >= 0 ? normalized : null;
}

function coerceTimestamp(value) {
  if (isFiniteNumber(value)) {
    return value;
  }
  return Date.now();
}

function createEmptyState() {
  return {
    unknownUpdateCount: 0,
    lastUnknownUpdateAt: null,
    lastUnknownUpdate: null,
  };
}

function isDiagnosticsState(value) {
  return (
    value &&
    typeof value === "object" &&
    typeof value.unknownUpdateCount === "number" &&
    Object.prototype.hasOwnProperty.call(value, "lastUnknownUpdateAt") &&
    Object.prototype.hasOwnProperty.call(value, "lastUnknownUpdate")
  );
}

export function createEffectDiagnosticsState() {
  return createEmptyState();
}

export function resetEffectDiagnosticsState(state) {
  const target = isDiagnosticsState(state) ? state : createEmptyState();
  target.unknownUpdateCount = 0;
  target.lastUnknownUpdateAt = null;
  target.lastUnknownUpdate = null;
  return target;
}

export function recordUnknownEffectUpdate(state, event, options = {}) {
  const target = isDiagnosticsState(state) ? state : createEmptyState();
  const timestamp = coerceTimestamp(options.timestamp);
  const count = isFiniteNumber(target.unknownUpdateCount)
    ? Math.max(0, Math.floor(target.unknownUpdateCount))
    : 0;

  target.unknownUpdateCount = count + 1;
  target.lastUnknownUpdateAt = timestamp;
  target.lastUnknownUpdate = {
    id: normalizeIdentifier(event?.id ?? null),
    seq: normalizeSequence(event?.seq ?? null),
    tick: normalizeTick(event?.tick ?? null),
  };

  return target;
}
