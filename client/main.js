const statusEl = document.getElementById("status");
const canvas = document.getElementById("game-canvas");
const ctx = canvas.getContext("2d");

statusEl.textContent = "Connecting to server...";

async function checkHealth() {
  try {
    const response = await fetch("http://localhost:8080/health");
    if (!response.ok) {
      throw new Error(`Request failed: ${response.status}`);
    }
    const text = await response.text();
    statusEl.textContent = `Server status: ${text.trim()}`;
  } catch (err) {
    statusEl.textContent = `Unable to reach server: ${err.message}`;
  }
}

function drawPlaceholder() {
  ctx.fillStyle = "#111";
  ctx.fillRect(0, 0, canvas.width, canvas.height);

  ctx.fillStyle = "#ffd166";
  ctx.font = "32px sans-serif";
  ctx.fillText("Mine and Die", 300, 280);
  ctx.font = "18px sans-serif";
  ctx.fillText("Canvas placeholder", 320, 320);
}

drawPlaceholder();
checkHealth();
