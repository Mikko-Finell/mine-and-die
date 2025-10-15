export function computeRtt(payload, now) {
  if (!payload || typeof payload !== "object") {
    return null;
  }
  const { rtt, clientTime } = payload;
  if (Number.isFinite(rtt)) {
    return Math.max(0, rtt);
  }
  if (Number.isFinite(clientTime)) {
    return Math.max(0, now - clientTime);
  }
  return null;
}

export function createHeartbeat({ now, setInterval, clearInterval, send }) {
  let intervalId = null;

  const invokeSend = () => {
    send(now());
  };

  const start = (intervalMs) => {
    stop();
    invokeSend();
    if (!Number.isFinite(intervalMs) || intervalMs <= 0) {
      return;
    }
    intervalId = setInterval(invokeSend, intervalMs);
  };

  const stop = () => {
    if (intervalId === null) {
      return;
    }
    clearInterval(intervalId);
    intervalId = null;
  };

  const isRunning = () => intervalId !== null;

  return { start, stop, isRunning };
}
