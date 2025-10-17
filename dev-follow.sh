#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVER_DIR="${SERVER_DIR:-$ROOT_DIR/server}"
BIN_DIR="$SERVER_DIR/bin"
PID_FILE="$BIN_DIR/server.pid"
PORT_FILE="$BIN_DIR/port"

BRANCH="${BRANCH:-main}"
BUILD_CMD="${BUILD_CMD:-go build -o bin/server.new .}"
RUN_CMD="${RUN_CMD:-./bin/server}"
POLL_SECONDS="${POLL_SECONDS:-10}"
PORT="${PORT:-8080}"
AUTO_BUMP="${AUTO_BUMP:-0}"
SYNC_MODE="${SYNC:-1}"
FOLLOW="${FOLLOW:-$SYNC_MODE}"

mkdir -p "$BIN_DIR"
SRV_PID=""
CLEANUP_DONE=0

port_in_use() {
  local p="${1:?port required}"
  lsof -iTCP:"$p" -sTCP:LISTEN -t >/dev/null 2>&1
}

wait_port_free() {
  local p="${1:?port required}"
  for _ in {1..30}; do port_in_use "$p" || return 0; sleep 0.1; done
  return 1
}

kill_pid_if_running() {
  local pid="${1:-}"
  [[ -z "$pid" ]] && return 0
  if ps -p "$pid" >/dev/null 2>&1; then
    kill "$pid" 2>/dev/null || true
    for _ in {1..20}; do ps -p "$pid" >/dev/null 2>&1 || break; sleep 0.1; done
    ps -p "$pid" >/dev/null 2>&1 && kill -9 "$pid" 2>/dev/null || true
  fi
}

ensure_port_clear() {
  if [[ -f "$PID_FILE" ]]; then
    local oldpid
    oldpid="$(cat "$PID_FILE" || true)"
    kill_pid_if_running "$oldpid"
    rm -f "$PID_FILE" || true
  fi

  if port_in_use "$PORT"; then
    lsof -iTCP:"$PORT" -sTCP:LISTEN -t 2>/dev/null | xargs -I{} bash -c 'kill {} 2>/dev/null || true'
    sleep 0.2
    port_in_use "$PORT" && lsof -iTCP:"$PORT" -sTCP:LISTEN -t 2>/dev/null | xargs -I{} bash -c 'kill -9 {} 2>/dev/null || true'
  fi

  wait_port_free "$PORT"
}

pick_port_if_needed() {
  if [[ "$AUTO_BUMP" != "1" ]]; then
    ensure_port_clear || { echo "❌ Port $PORT is busy; set AUTO_BUMP=1 to auto-pick."; exit 1; }
    echo "$PORT" > "$PORT_FILE"
    return 0
  fi

  local base="$PORT"
  for p in "$base" $((base+1)) $((base+2)) $((base+3)) $((base+4)) $((base+5)); do
    PORT="$p"
    if ensure_port_clear; then
      echo "🔌 using port $PORT"
      echo "$PORT" > "$PORT_FILE"
      return 0
    fi
  done
  echo "❌ No free port in range $base..$((base+5))"; exit 1
}

start_server() {
  pick_port_if_needed
  pushd "$SERVER_DIR" >/dev/null
  if [[ "$RUN_CMD" == "./bin/server" ]]; then
    PORT="$PORT" "$RUN_CMD" &
  else
    PORT="$PORT" bash -c "exec $RUN_CMD" &
  fi
  SRV_PID=$!
  popd >/dev/null
  echo "$SRV_PID" > "$PID_FILE"
  echo "▶️  started pid=$SRV_PID @ $(date) on :$PORT"
}

stop_server() {
  local pid="${SRV_PID:-}"
  if [[ -z "$pid" ]] && [[ -f "$PID_FILE" ]]; then
    pid="$(cat "$PID_FILE" || true)"
  fi

  if [[ -n "$pid" ]] && ps -p "$pid" >/dev/null 2>&1; then
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
    echo "⏹  stopped pid=$pid"
  fi
  SRV_PID=""
  rm -f "$PID_FILE" || true
}

build_swap_run() {
  echo "🔨 building…"
  if (cd "$SERVER_DIR" && eval "$BUILD_CMD"); then
    mv "$BIN_DIR/server.new" "$BIN_DIR/server"
    echo "✅ build ok"
    stop_server
    wait_port_free "$PORT" || true
    start_server
  else
    echo "❌ build failed; keeping old server running"
    rm -f "$BIN_DIR/server.new" || true
  fi
}

cleanup() {
  local reason="${1:-EXIT}"

  if [[ "$CLEANUP_DONE" == "1" ]]; then
    return 0
  fi
  CLEANUP_DONE=1

  if [[ "$reason" == "SIGINT" ]] || [[ "$reason" == "SIGTERM" ]]; then
    local pid="${SRV_PID:-}"
    if [[ -z "$pid" ]] && [[ -f "$PID_FILE" ]]; then
      pid="$(cat "$PID_FILE" || true)"
    fi
    if [[ -n "$pid" ]] && ps -p "$pid" >/dev/null 2>&1; then
      local sig="-INT"
      [[ "$reason" == "SIGTERM" ]] && sig="-TERM"
      kill "$sig" "$pid" 2>/dev/null || true
    fi
  fi

  stop_server
  rm -f "$PID_FILE" || true
  echo "🧹 cleaned up"
}

handle_sigint() {
  cleanup SIGINT
  exit 130
}

handle_sigterm() {
  cleanup SIGTERM
  exit 143
}

trap handle_sigint INT
trap handle_sigterm TERM
trap 'cleanup EXIT' EXIT

if [[ "$SYNC_MODE" == "1" ]]; then
  git -C "$ROOT_DIR" fetch origin
  git -C "$ROOT_DIR" switch -f "$BRANCH"
  git -C "$ROOT_DIR" reset --hard "origin/$BRANCH"
else
  echo "ℹ️  Running without git sync; using local sources"
fi

build_swap_run

if [[ "$FOLLOW" != "1" ]]; then
  if [[ -n "$SRV_PID" ]]; then
    wait "$SRV_PID"
  fi
  exit 0
fi

while sleep "$POLL_SECONDS"; do
  git -C "$ROOT_DIR" fetch origin --quiet
  LOCAL=$(git -C "$ROOT_DIR" rev-parse HEAD)
  REMOTE=$(git -C "$ROOT_DIR" rev-parse "origin/$BRANCH")
  if [[ "$LOCAL" != "$REMOTE" ]]; then
    echo "⬇️  upstream changed -> updating to $REMOTE"
    git -C "$ROOT_DIR" reset --hard "origin/$BRANCH"
    build_swap_run
  fi

done

