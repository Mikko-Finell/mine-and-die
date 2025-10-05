import { sendCurrentIntent } from "./network.js";

export function registerInputHandlers(store) {
  function handleKey(event, isPressed) {
    const key = event.key.toLowerCase();
    if (["w", "a", "s", "d"].includes(key)) {
      event.preventDefault();
      if (isPressed) {
        store.keys.add(key);
      } else {
        store.keys.delete(key);
      }
      updateIntentFromKeys();
    }
  }

  function updateIntentFromKeys() {
    let dx = 0;
    let dy = 0;
    if (store.keys.has("w")) dy -= 1;
    if (store.keys.has("s")) dy += 1;
    if (store.keys.has("a")) dx -= 1;
    if (store.keys.has("d")) dx += 1;

    if (dx !== 0 || dy !== 0) {
      const length = Math.hypot(dx, dy) || 1;
      dx /= length;
      dy /= length;
    }

    if (dx === store.currentIntent.dx && dy === store.currentIntent.dy) {
      return;
    }

    store.currentIntent = { dx, dy };
    sendCurrentIntent(store);
  }

  document.addEventListener("keydown", (event) => handleKey(event, true));
  document.addEventListener("keyup", (event) => handleKey(event, false));
}
