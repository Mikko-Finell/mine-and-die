import { sendCurrentIntent } from "./network.js";

const DEFAULT_FACING = "down";
const KEY_TO_FACING = {
  w: "up",
  a: "left",
  s: "down",
  d: "right",
};
const MOVEMENT_KEYS = new Set(Object.keys(KEY_TO_FACING));

export function registerInputHandlers(store) {
  function handleKey(event, isPressed) {
    const key = event.key.toLowerCase();
    if (!MOVEMENT_KEYS.has(key)) {
      return;
    }

    event.preventDefault();
    if (isPressed) {
      if (!store.keys.has(key)) {
        store.directionOrder = store.directionOrder.filter((entry) => entry !== key);
        store.directionOrder.push(key);
      }
      store.keys.add(key);
    } else {
      store.keys.delete(key);
      store.directionOrder = store.directionOrder.filter((entry) => entry !== key);
    }

    updateIntentFromKeys();
  }

  function deriveFacing(rawDx, rawDy) {
    const currentFacing = store.currentFacing || DEFAULT_FACING;

    if (rawDx === 0 && rawDy === 0) {
      if (store.directionOrder.length > 0) {
        const lastKey = store.directionOrder[store.directionOrder.length - 1];
        return KEY_TO_FACING[lastKey] || currentFacing;
      }
      return currentFacing;
    }

    const absX = Math.abs(rawDx);
    const absY = Math.abs(rawDy);

    if (absY >= absX && rawDy !== 0) {
      return rawDy > 0 ? "down" : "up";
    }
    if (rawDx !== 0) {
      return rawDx > 0 ? "right" : "left";
    }
    return currentFacing;
  }

  function updateIntentFromKeys() {
    let dx = 0;
    let dy = 0;
    if (store.keys.has("w")) dy -= 1;
    if (store.keys.has("s")) dy += 1;
    if (store.keys.has("a")) dx -= 1;
    if (store.keys.has("d")) dx += 1;

    const rawDx = dx;
    const rawDy = dy;

    if (dx !== 0 || dy !== 0) {
      const length = Math.hypot(dx, dy) || 1;
      dx /= length;
      dy /= length;
    }

    const nextFacing = deriveFacing(rawDx, rawDy);
    const facingChanged = nextFacing !== store.currentFacing;
    if (facingChanged) {
      store.currentFacing = nextFacing;
    }

    if (!facingChanged && dx === store.currentIntent.dx && dy === store.currentIntent.dy) {
      return;
    }

    store.currentIntent = { dx, dy };
    sendCurrentIntent(store);
  }

  document.addEventListener("keydown", (event) => handleKey(event, true));
  document.addEventListener("keyup", (event) => handleKey(event, false));
}
