import {
  parseServerEvent,
  handleProtocolVersion,
  applyStateSnapshot,
  deriveDisplayMaps,
} from "../../network.js";
import {
  applyEffectLifecycleBatch,
  updateEphemeralEffectLifecycleEntries,
} from "../../effect-lifecycle.js";

export function ingestStateMessage(store, rawMessage) {
  const parsed = parseServerEvent(rawMessage);
  if (!parsed || parsed.type !== "state") {
    throw new Error("Expected state message payload");
  }

  handleProtocolVersion(parsed.data, "state message");

  const snapshot = applyStateSnapshot(store, parsed.data, null);
  store.players = snapshot.players;
  store.npcs = snapshot.npcs;
  store.obstacles = snapshot.obstacles;
  store.effects = snapshot.effects;
  store.groundItems = snapshot.groundItems;
  store.lastTick = snapshot.lastTick;

  if (snapshot.worldConfig) {
    store.worldConfig = snapshot.worldConfig;
    store.WORLD_WIDTH = snapshot.worldConfig.width;
    store.WORLD_HEIGHT = snapshot.worldConfig.height;
  }

  if (Number.isFinite(snapshot.keyframeInterval)) {
    store.keyframeInterval = snapshot.keyframeInterval;
  }

  if (snapshot.currentFacing) {
    store.currentFacing = snapshot.currentFacing;
  }

  const { displayPlayers, displayNPCs } = deriveDisplayMaps(
    store.players,
    store.npcs,
    store.displayPlayers,
    store.displayNPCs,
  );
  store.displayPlayers = displayPlayers;
  store.displayNPCs = displayNPCs;

  const lifecycleSummary = applyEffectLifecycleBatch(store, parsed.data);
  store.lastEffectLifecycleSummary = lifecycleSummary;
  updateEphemeralEffectLifecycleEntries(store, lifecycleSummary);

  return { parsed, snapshot, lifecycleSummary };
}
