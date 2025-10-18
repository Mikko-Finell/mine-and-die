import { GameClientOrchestrator } from "./client-manager";
import {
  InMemoryInputStore,
  KeyboardInputController,
  type InputBindings,
  type PathTarget,
} from "./input";
import { WebSocketNetworkClient, type WorldConfigurationSnapshot } from "./network";
import { CanvasRenderer } from "./render";
import { InMemoryWorldStateStore } from "./world-state";

const HEALTH_CHECK_URL = "/health";
const JOIN_URL = "/join";
const WEBSOCKET_URL = "/ws";
const HEARTBEAT_INTERVAL_MS = 2000;
const PROTOCOL_VERSION = 1;

const RENDERER_CONFIGURATION = {
  dimensions: { width: 800, height: 600 },
  layers: [
    { id: "effect-visual", zIndex: 1 },
    { id: "effect-area", zIndex: 2 },
    { id: "effect-target", zIndex: 3 },
  ],
} as const;

const ORCHESTRATOR_CONFIGURATION = {
  autoConnect: true,
  reconcileIntervalMs: 0,
  keyframeRetryDelayMs: 1000,
} as const;

const INPUT_BINDINGS: InputBindings = {
  attackAction: "attack",
  fireballAction: "fireball",
  cameraLockKey: "c",
  movementKeys: {
    w: "up",
    a: "left",
    s: "down",
    d: "right",
  },
};

const clamp = (value: number, min: number, max: number): number => {
  if (value < min) {
    return min;
  }
  if (value > max) {
    return max;
  }
  return value;
};

const canvas = document.createElement("canvas");
canvas.width = RENDERER_CONFIGURATION.dimensions.width;
canvas.height = RENDERER_CONFIGURATION.dimensions.height;
canvas.style.display = "block";
canvas.style.width = "100vw";
canvas.style.height = "100vh";
canvas.style.touchAction = "none";
canvas.setAttribute("aria-label", "Game viewport");
canvas.style.flex = "1 1 auto";

document.documentElement.style.height = "100%";
document.documentElement.style.margin = "0";
document.documentElement.style.padding = "0";

document.body.style.margin = "0";
document.body.style.height = "100vh";
document.body.style.background = "#050505";
document.body.style.display = "flex";
document.body.style.alignItems = "stretch";
document.body.style.justifyContent = "stretch";
document.body.style.overflow = "hidden";
document.body.appendChild(canvas);

const context = canvas.getContext("2d");
if (!context) {
  throw new Error("Debug client failed to acquire 2D context");
}

const renderer = new CanvasRenderer(RENDERER_CONFIGURATION);
renderer.mount({ canvas, context });

const worldStateStore = new InMemoryWorldStateStore();
const networkClient = new WebSocketNetworkClient({
  joinUrl: JOIN_URL,
  websocketUrl: WEBSOCKET_URL,
  heartbeatIntervalMs: HEARTBEAT_INTERVAL_MS,
  protocolVersion: PROTOCOL_VERSION,
});

const orchestrator = new GameClientOrchestrator(ORCHESTRATOR_CONFIGURATION, {
  network: networkClient,
  renderer,
  worldState: worldStateStore,
});

const inputStore = new InMemoryInputStore({
  onCameraLockToggle: (locked) => {
    console.debug(`[debug-client] camera lock ${locked ? "enabled" : "disabled"}`);
  },
});

const inputDispatcher = orchestrator.createInputDispatcher({
  onPathCommand: (state) => {
    inputStore.setPathActive?.(state.active);
    inputStore.setPathTarget?.(state.target);
  },
  onCommandRejectionChanged: (rejection) => {
    inputStore.setCommandRejection?.(rejection);
    if (rejection) {
      console.warn(`[debug-client] command rejected`, rejection);
    }
  },
});

const inputController = new KeyboardInputController({
  store: inputStore,
  dispatcher: inputDispatcher,
  bindings: INPUT_BINDINGS,
});
inputController.register();

let worldDimensions: WorldConfigurationSnapshot | null = null;

const translatePointerToWorld = (event: PointerEvent): PathTarget | null => {
  const rect = canvas.getBoundingClientRect();
  if (rect.width === 0 || rect.height === 0) {
    return null;
  }
  const scaleX = canvas.width / rect.width;
  const scaleY = canvas.height / rect.height;
  const localX = (event.clientX - rect.left) * scaleX;
  const localY = (event.clientY - rect.top) * scaleY;

  const width = worldDimensions?.width ?? canvas.width;
  const height = worldDimensions?.height ?? canvas.height;

  return {
    x: clamp(localX, 0, width),
    y: clamp(localY, 0, height),
  };
};

canvas.addEventListener("pointerdown", (event) => {
  if (event.button === 0) {
    event.preventDefault();
    const target = translatePointerToWorld(event);
    if (!target) {
      return;
    }
    inputDispatcher.sendPathCommand(target);
    return;
  }
  if (event.button === 2) {
    event.preventDefault();
    inputDispatcher.cancelPath();
  }
});

canvas.addEventListener("contextmenu", (event) => {
  event.preventDefault();
});

const boot = async (): Promise<void> => {
  console.debug("[debug-client] booting minimal client UI");
  try {
    await orchestrator.boot({
      onReady: () => {
        const join = orchestrator.getJoinResponse();
        worldDimensions = join?.world ?? null;
        if (join) {
          console.info(`[debug-client] joined world as ${join.id}`);
          console.info(
            `[debug-client] catalog entries: ${Object.keys(join.effectCatalog ?? {}).length}`,
          );
          console.info(
            `[debug-client] world size: ${join.world?.width ?? "?"}×${join.world?.height ?? "?"}`,
          );
        }
      },
      onError: (error) => {
        console.error("[debug-client] client error", error);
      },
      onHeartbeat: (telemetry) => {
        const parts = [] as string[];
        if (typeof telemetry.roundTripTimeMs === "number") {
          parts.push(`rtt ${Math.round(telemetry.roundTripTimeMs)}ms`);
        }
        if (typeof telemetry.serverTime === "number") {
          parts.push(`server ${telemetry.serverTime}`);
        }
        parts.push(`age ${Date.now() - telemetry.receivedAt}ms`);
        console.debug(`[debug-client] heartbeat: ${parts.join(" · ")}`);
      },
      onLog: (message) => {
        console.debug(`[debug-client] ${message}`);
      },
    });
    const response = await fetch(HEALTH_CHECK_URL, { cache: "no-cache" });
    console.debug(`[debug-client] health status: ${response.status} ${response.statusText}`);
  } catch (error) {
    console.error("[debug-client] failed to boot client", error);
  }
};

void boot();

const cleanup = (): void => {
  inputController.unregister();
  canvas.remove();
  renderer.unmount();
  void orchestrator.shutdown().catch((error) => {
    console.error("[debug-client] failed during shutdown", error);
  });
};

window.addEventListener("beforeunload", cleanup);
window.addEventListener("pagehide", cleanup);
