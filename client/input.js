import { sendAction, sendCurrentIntent } from "./network.js";

const DEFAULT_FACING = "down";
const KEY_TO_FACING = {
  w: "up",
  a: "left",
  s: "down",
  d: "right",
};
const MOVEMENT_KEYS = new Set(Object.keys(KEY_TO_FACING));
const ATTACK_ACTION = "attack";

// registerInputHandlers keeps the authoritative record of keyboard intent on the
// client. We maintain two pieces of state:
//   • `store.keys` tracks which movement keys are currently depressed so we can
//     compute the desired velocity vector (which is normalized before being
//     sent to the server).
//   • `store.directionOrder` remembers the order keys were pressed so we can
//     surface the last meaningful direction as the avatar's facing when the
//     player comes to a stop.
// Every time the derived intent or facing changes we immediately send the
// updated payload via `sendCurrentIntent`, keeping the server simulation in
// sync with the player's local input.
export function registerInputHandlers(store) {
  function handleKey(event, isPressed) {
    if (event.code === "Space") {
      event.preventDefault();
      if (isPressed && !event.repeat) {
        sendAction(store, ATTACK_ACTION);
      }
      return;
    }

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

  // deriveFacing converts the raw (non-normalized) directional input into one
  // of the four cardinal facing strings. Priority works as follows:
  //   • When there is movement, whichever axis has the greater absolute value
  //     wins. This mimics typical top-down movement, ensuring diagonals favor
  //     vertical motion unless horizontal input is stronger.
  //   • When movement ceases we fall back to the last pressed key (tracked in
  //     `store.directionOrder`) so the avatar keeps looking the way the player
  //     expects while idle.
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

  // updateIntentFromKeys reads the current keyboard state, builds a normalized
  // velocity vector, and decides whether the facing should change. The
  // resulting intent is cached on the store and only dispatched when something
  // actually changed to avoid redundant network traffic.
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
