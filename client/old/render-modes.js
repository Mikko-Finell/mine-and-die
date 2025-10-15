export const RENDER_MODE_SNAPSHOT = "snapshot";
export const RENDER_MODE_PATCH = "patch";

const VALID_RENDER_MODES = new Set([
  RENDER_MODE_SNAPSHOT,
  RENDER_MODE_PATCH,
]);

export function isRenderMode(value) {
  return VALID_RENDER_MODES.has(value);
}

export function normalizeRenderMode(value) {
  if (typeof value !== "string") {
    return null;
  }
  const normalized = value.trim().toLowerCase();
  if (normalized === RENDER_MODE_PATCH) {
    return RENDER_MODE_PATCH;
  }
  if (normalized === RENDER_MODE_SNAPSHOT) {
    return RENDER_MODE_SNAPSHOT;
  }
  return null;
}

export function getValidRenderModes() {
  return Array.from(VALID_RENDER_MODES);
}
