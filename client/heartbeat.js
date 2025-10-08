export function computeRtt(payload, now) {
  const source = payload && typeof payload === "object" ? payload : {};

  if (Number.isFinite(source.rtt)) {
    return Math.max(0, source.rtt);
  }

  if (Number.isFinite(source.clientTime) && Number.isFinite(now)) {
    return Math.max(0, now - source.clientTime);
  }

  return null;
}

export function createHeartbeat({ now, setInterval, clearInterval, send }) {
  let intervalId = null;

  const start = (intervalMs) => {
    stop();
    send();
    intervalId = setInterval(() => {
      send();
    }, intervalMs);
    return intervalId;
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
