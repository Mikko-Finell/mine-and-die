// client/main.ts
var appRoot = document.querySelector("#app");
if (!appRoot) {
  throw new Error("Missing app root");
}
var consoleOutput = document.querySelector("#console-output");
var healthStatus = document.querySelector("#health-status");
var serverTime = document.querySelector("#server-time");
var heartbeat = document.querySelector("#heartbeat");
var refreshButton = document.querySelector("#refresh-health");
var canvas = document.querySelector("#viewport");
function logToConsole(message) {
  var _a;
  if (!consoleOutput) {
    return;
  }
  const timestamp = (/* @__PURE__ */ new Date()).toLocaleTimeString();
  consoleOutput.textContent = `[${timestamp}] ${message}
${(_a = consoleOutput.textContent) != null ? _a : ""}`;
}
async function fetchHealth() {
  if (!healthStatus) {
    return;
  }
  healthStatus.textContent = "Checking\u2026";
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
function updateServerTime() {
  if (serverTime) {
    const now = /* @__PURE__ */ new Date();
    serverTime.textContent = now.toLocaleTimeString();
  }
  if (heartbeat) {
    heartbeat.textContent = `${Math.round(Math.random() * 1e3)} ms`;
  }
}
function bootViewport() {
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
refreshButton == null ? void 0 : refreshButton.addEventListener("click", () => {
  fetchHealth().catch((error) => {
    logToConsole(`Refresh failed: ${String(error)}`);
  });
});
bootViewport();
updateServerTime();
fetchHealth().catch((error) => {
  logToConsole(`Initial health check failed: ${String(error)}`);
});
setInterval(updateServerTime, 1e3);
//# sourceMappingURL=main.js.map
