const appRoot = document.querySelector<HTMLElement>("#app");
if (!appRoot) {
  throw new Error("Missing app root");
}

const consoleOutput = document.querySelector<HTMLPreElement>("#console-output");
const healthStatus = document.querySelector<HTMLSpanElement>("#health-status");
const serverTime = document.querySelector<HTMLSpanElement>("#server-time");
const heartbeat = document.querySelector<HTMLSpanElement>("#heartbeat");
const refreshButton = document.querySelector<HTMLButtonElement>("#refresh-health");
const canvas = document.querySelector<HTMLCanvasElement>("#viewport");

function logToConsole(message: string): void {
  if (!consoleOutput) {
    return;
  }
  const timestamp = new Date().toLocaleTimeString();
  consoleOutput.textContent = `[${timestamp}] ${message}\n${consoleOutput.textContent ?? ""}`;
}

async function fetchHealth(): Promise<void> {
  if (!healthStatus) {
    return;
  }
  healthStatus.textContent = "Checking…";
  try {
    const response = await fetch("/health", { cache: "no-cache" });
    if (!response.ok) {
      throw new Error(`health check failed with ${response.status}`);
    }
    const text = (await response.text()).trim();
    healthStatus.textContent = text || "ok";
    logToConsole(`Health check succeeded: ${text || "ok"}`);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    healthStatus.textContent = "offline";
    logToConsole(`Health check failed: ${message}`);
  }
}

function updateServerTime(): void {
  if (serverTime) {
    const now = new Date();
    serverTime.textContent = now.toLocaleTimeString();
  }
  if (heartbeat) {
    heartbeat.textContent = `${Math.round(Math.random() * 1000)} ms`;
  }
}

function bootViewport(): void {
  if (!canvas) {
    return;
  }
  const context = canvas.getContext("2d");
  if (!context) {
    logToConsole("Canvas context unavailable");
    return;
  }
  const { width, height } = canvas;
  const gradient = context.createLinearGradient(0, 0, width, height);
  gradient.addColorStop(0, "#0f172a");
  gradient.addColorStop(1, "#1e293b");
  context.fillStyle = gradient;
  context.fillRect(0, 0, width, height);

  context.fillStyle = "#38bdf8";
  context.font = "24px 'Segoe UI', sans-serif";
  context.textAlign = "center";
  context.fillText("Mine & Die", width / 2, height / 2);
}

refreshButton?.addEventListener("click", () => {
  fetchHealth().catch((error) => {
    logToConsole(`Refresh failed: ${String(error)}`);
  });
});

bootViewport();
updateServerTime();
fetchHealth().catch((error) => {
  logToConsole(`Initial health check failed: ${String(error)}`);
});

setInterval(updateServerTime, 1000);
