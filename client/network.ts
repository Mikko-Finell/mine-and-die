import {
  normalizeEffectCatalog,
  type EffectCatalogSnapshot,
} from "./effect-catalog";
import { effectCatalogHash } from "./generated/effect-contracts-hash";

const isObject = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null;

const readFiniteNumber = (value: unknown, context: string): number => {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    throw new Error(`${context} must be a finite number.`);
  }
  return value;
};

const readOptionalFiniteNumber = (
  value: unknown,
): number | undefined =>
  typeof value === "number" && Number.isFinite(value) ? value : undefined;

const readString = (value: unknown, context: string): string => {
  if (typeof value !== "string" || value.length === 0) {
    throw new Error(`${context} must be a non-empty string.`);
  }
  return value;
};

const readOptionalBoolean = (value: unknown): boolean | undefined =>
  typeof value === "boolean" ? value : undefined;

const cloneRecord = (value: Record<string, unknown>): Record<string, unknown> =>
  Object.fromEntries(Object.entries(value));

export interface ActorSnapshot {
  readonly id: string;
  readonly x: number;
  readonly y: number;
  readonly facing?: string;
  readonly health?: number;
  readonly maxHealth?: number;
  readonly inventory?: Record<string, unknown>;
  readonly equipment?: Record<string, unknown>;
  readonly [key: string]: unknown;
}

export interface PlayerSnapshot extends ActorSnapshot {}

export interface NPCSnapshot extends ActorSnapshot {
  readonly type?: string;
  readonly aiControlled?: boolean;
  readonly experienceReward?: number;
}

export interface ObstacleSnapshot {
  readonly id: string;
  readonly type?: string;
  readonly x: number;
  readonly y: number;
  readonly width: number;
  readonly height: number;
  readonly [key: string]: unknown;
}

export interface GroundItemSnapshot {
  readonly id: string;
  readonly type: string;
  readonly fungibility_key?: string;
  readonly x: number;
  readonly y: number;
  readonly qty: number;
  readonly [key: string]: unknown;
}

export interface NetworkPatch {
  readonly kind: string;
  readonly entityId: string;
  readonly payload?: unknown;
}

const normalizeInventory = (
  value: unknown,
): Record<string, unknown> | undefined => {
  if (!isObject(value)) {
    return undefined;
  }
  return cloneRecord(value);
};

const normalizeActorSnapshot = (
  value: unknown,
  context: string,
): ActorSnapshot => {
  if (!isObject(value)) {
    throw new Error(`${context} must be an object.`);
  }

  const record = cloneRecord(value);
  const id = readString(record.id, `${context}.id`);
  const x = readFiniteNumber(record.x, `${context}.x`);
  const y = readFiniteNumber(record.y, `${context}.y`);
  const facingValue = record.facing;
  const facing = typeof facingValue === "string" && facingValue.length > 0 ? facingValue : undefined;
  const health = readOptionalFiniteNumber(record.health);
  const maxHealth = readOptionalFiniteNumber(record.maxHealth);
  const inventory = normalizeInventory(record.inventory);
  const equipment = normalizeInventory(record.equipment);

  const sanitized: Record<string, unknown> = { ...record, id, x, y };
  if (facing !== undefined) {
    sanitized.facing = facing;
  } else {
    delete sanitized.facing;
  }
  if (health !== undefined) {
    sanitized.health = health;
  } else {
    delete sanitized.health;
  }
  if (maxHealth !== undefined) {
    sanitized.maxHealth = maxHealth;
  } else {
    delete sanitized.maxHealth;
  }
  if (inventory !== undefined) {
    sanitized.inventory = inventory;
  } else {
    delete sanitized.inventory;
  }
  if (equipment !== undefined) {
    sanitized.equipment = equipment;
  } else {
    delete sanitized.equipment;
  }

  return sanitized as ActorSnapshot;
};

const normalizePlayerSnapshot = (
  value: unknown,
  context: string,
): PlayerSnapshot => normalizeActorSnapshot(value, context) as PlayerSnapshot;

const normalizeNPCSnapshot = (value: unknown, context: string): NPCSnapshot => {
  const base = normalizeActorSnapshot(value, context) as Record<string, unknown>;
  const typeValue = base.type;
  const type = typeof typeValue === "string" && typeValue.length > 0 ? typeValue : undefined;
  const aiControlled = readOptionalBoolean(base.aiControlled);
  const experienceReward = readOptionalFiniteNumber(base.experienceReward);

  const sanitized: Record<string, unknown> = { ...base };
  if (type !== undefined) {
    sanitized.type = type;
  } else {
    delete sanitized.type;
  }
  if (aiControlled !== undefined) {
    sanitized.aiControlled = aiControlled;
  } else {
    delete sanitized.aiControlled;
  }
  if (experienceReward !== undefined) {
    sanitized.experienceReward = experienceReward;
  } else {
    delete sanitized.experienceReward;
  }

  return sanitized as NPCSnapshot;
};

const normalizeObstacleSnapshot = (
  value: unknown,
  context: string,
): ObstacleSnapshot => {
  if (!isObject(value)) {
    throw new Error(`${context} must be an object.`);
  }
  const record = cloneRecord(value);
  const id = readString(record.id, `${context}.id`);
  const x = readFiniteNumber(record.x, `${context}.x`);
  const y = readFiniteNumber(record.y, `${context}.y`);
  const width = readFiniteNumber(record.width, `${context}.width`);
  const height = readFiniteNumber(record.height, `${context}.height`);
  const typeValue = record.type;
  const type = typeof typeValue === "string" && typeValue.length > 0 ? typeValue : undefined;

  const sanitized: Record<string, unknown> = { ...record, id, x, y, width, height };
  if (type !== undefined) {
    sanitized.type = type;
  } else {
    delete sanitized.type;
  }

  return sanitized as ObstacleSnapshot;
};

const normalizeGroundItemSnapshot = (
  value: unknown,
  context: string,
): GroundItemSnapshot => {
  if (!isObject(value)) {
    throw new Error(`${context} must be an object.`);
  }
  const record = cloneRecord(value);
  const id = readString(record.id, `${context}.id`);
  const type = readString(record.type, `${context}.type`);
  const x = readFiniteNumber(record.x, `${context}.x`);
  const y = readFiniteNumber(record.y, `${context}.y`);
  const qty = Math.floor(readFiniteNumber(record.qty, `${context}.qty`));
  const fungibility = typeof record.fungibility_key === "string" && record.fungibility_key.length > 0 ? record.fungibility_key : undefined;

  const sanitized: Record<string, unknown> = { ...record, id, type, x, y, qty };
  if (fungibility !== undefined) {
    sanitized.fungibility_key = fungibility;
  } else {
    delete sanitized.fungibility_key;
  }

  return sanitized as GroundItemSnapshot;
};

const mapArray = <T>(
  value: unknown,
  context: string,
  mapper: (entry: unknown, entryContext: string) => T,
): readonly T[] => {
  if (!Array.isArray(value)) {
    throw new Error(`${context} must be an array.`);
  }
  return value.map((entry, index) => mapper(entry, `${context}[${index}]`));
};

export const normalizePlayerSnapshots = (
  value: unknown,
  context: string,
): readonly PlayerSnapshot[] => mapArray(value ?? [], context, normalizePlayerSnapshot);

export const normalizeNPCSnapshots = (
  value: unknown,
  context: string,
): readonly NPCSnapshot[] => mapArray(value ?? [], context, normalizeNPCSnapshot);

export const normalizeObstacleSnapshots = (
  value: unknown,
  context: string,
): readonly ObstacleSnapshot[] => mapArray(value ?? [], context, normalizeObstacleSnapshot);

export const normalizeGroundItemSnapshots = (
  value: unknown,
  context: string,
): readonly GroundItemSnapshot[] => mapArray(value ?? [], context, normalizeGroundItemSnapshot);

export const normalizeNetworkPatches = (
  value: unknown,
  context: string,
): readonly NetworkPatch[] => {
  if (!Array.isArray(value)) {
    if (value === undefined) {
      return [];
    }
    throw new Error(`${context} must be an array.`);
  }

  return value.map((entry, index) => {
    if (!isObject(entry)) {
      throw new Error(`${context}[${index}] must be an object.`);
    }
    const record = entry as Record<string, unknown>;
    const kind = readString(record.kind, `${context}[${index}].kind`);
    const entityId = readString(record.entityId, `${context}[${index}].entityId`);
    const payload = record.payload;
    return { kind, entityId, payload } as NetworkPatch;
  });
};

export interface WorldConfigurationSnapshot {
  readonly width: number;
  readonly height: number;
}

export interface JoinResponse {
  readonly id: string;
  readonly seed: string;
  readonly protocolVersion: number;
  readonly effectCatalogHash: string;
  readonly effectCatalog: EffectCatalogSnapshot;
  readonly world: WorldConfigurationSnapshot;
  readonly players: readonly PlayerSnapshot[];
  readonly npcs: readonly NPCSnapshot[];
  readonly obstacles: readonly ObstacleSnapshot[];
  readonly groundItems: readonly GroundItemSnapshot[];
  readonly patches: readonly NetworkPatch[];
}

export interface NetworkMessageEnvelope {
  readonly type: "keyframe" | "keyframeNack" | "patch" | "heartbeat" | "error" | string;
  readonly payload: unknown;
  readonly receivedAt: number;
}

export interface HeartbeatAck {
  readonly serverTime: number | null;
  readonly clientTime: number | null;
  readonly roundTripTime: number | null;
  readonly receivedAt: number;
}

export interface NetworkEventHandlers {
  readonly onJoin?: (response: JoinResponse) => void;
  readonly onMessage?: (message: NetworkMessageEnvelope) => void;
  readonly onDisconnect?: (code?: number, reason?: string) => void;
  readonly onError?: (error: Error) => void;
  readonly onHeartbeat?: (ack: HeartbeatAck) => void;
}

export interface NetworkClientConfiguration {
  readonly joinUrl: string;
  readonly websocketUrl: string;
  readonly heartbeatIntervalMs: number;
  readonly protocolVersion: number;
}

export interface NetworkClient {
  readonly configuration: NetworkClientConfiguration;
  readonly join: () => Promise<JoinResponse>;
  readonly connect: (handlers: NetworkEventHandlers) => Promise<void>;
  readonly disconnect: () => Promise<void>;
  readonly send: (data: unknown) => void;
}

export class WebSocketNetworkClient implements NetworkClient {
  private socket: WebSocket | null = null;
  private joinResponse: JoinResponse | null = null;
  private handlers: NetworkEventHandlers | null = null;
  private heartbeatTimer: number | null = null;
  private lastHeartbeatSentAt: number | null = null;

  constructor(public readonly configuration: NetworkClientConfiguration) {}

  async join(): Promise<JoinResponse> {
    await this.disconnect();
    this.joinResponse = null;
    this.lastHeartbeatSentAt = null;

    const response = await fetch(this.configuration.joinUrl, {
      method: "POST",
      cache: "no-store",
    });

    if (!response.ok) {
      throw new Error(`Join request failed with status ${response.status}`);
    }

    const payload = (await response.json()) as unknown;
    if (!payload || typeof payload !== "object") {
      throw new Error("Join response payload is not an object.");
    }

    const joinPayload = payload as {
      readonly ver?: unknown;
      readonly id?: unknown;
      readonly config?: unknown;
      readonly effectCatalogHash?: unknown;
      readonly players?: unknown;
      readonly npcs?: unknown;
      readonly obstacles?: unknown;
      readonly groundItems?: unknown;
      readonly patches?: unknown;
    };

    if (typeof joinPayload.id !== "string" || joinPayload.id.length === 0) {
      throw new Error("Join response missing player identifier.");
    }

    if (!joinPayload.config || typeof joinPayload.config !== "object") {
      throw new Error("Join response missing world configuration.");
    }

    const config = joinPayload.config as {
      readonly seed?: unknown;
      readonly width?: unknown;
      readonly height?: unknown;
      readonly effectCatalog?: unknown;
    };
    if (typeof config.seed !== "string" || config.seed.length === 0) {
      throw new Error("Join response missing world seed.");
    }

    if (typeof config.width !== "number" || !Number.isFinite(config.width)) {
      throw new Error("Join response missing world width.");
    }

    if (typeof config.height !== "number" || !Number.isFinite(config.height)) {
      throw new Error("Join response missing world height.");
    }

    if (typeof joinPayload.ver !== "number") {
      throw new Error("Join response missing protocol version.");
    }

    const catalogHash = readString(
      joinPayload.effectCatalogHash,
      "join.effectCatalogHash",
    );

    if (catalogHash !== effectCatalogHash) {
      throw new Error(
        `Effect catalog mismatch: expected hash ${effectCatalogHash}, received ${catalogHash}.`,
      );
    }

    const effectCatalog = normalizeEffectCatalog(config.effectCatalog);

    if (joinPayload.ver !== this.configuration.protocolVersion) {
      throw new Error(
        `Protocol mismatch: expected ${this.configuration.protocolVersion}, received ${joinPayload.ver}`,
      );
    }

    const world: WorldConfigurationSnapshot = {
      width: config.width,
      height: config.height,
    };

    const players = normalizePlayerSnapshots(joinPayload.players, "join.players");
    const npcs = normalizeNPCSnapshots(joinPayload.npcs, "join.npcs");
    const obstacles = normalizeObstacleSnapshots(joinPayload.obstacles, "join.obstacles");
    const groundItems = normalizeGroundItemSnapshots(joinPayload.groundItems, "join.groundItems");
    const patches = normalizeNetworkPatches(joinPayload.patches, "join.patches");

    const joinResponse: JoinResponse = {
      id: joinPayload.id,
      seed: config.seed,
      protocolVersion: joinPayload.ver,
      effectCatalogHash: catalogHash,
      effectCatalog,
      world,
      players,
      npcs,
      obstacles,
      groundItems,
      patches,
    };

    this.joinResponse = joinResponse;
    return joinResponse;
  }

  async connect(handlers: NetworkEventHandlers): Promise<void> {
    if (!this.joinResponse) {
      throw new Error("Cannot connect before joining the world.");
    }

    await this.disconnect();

    this.handlers = handlers;
    const socketUrl = this.createWebSocketUrl(this.joinResponse.id);
    const socket = new WebSocket(socketUrl);
    this.socket = socket;

    await new Promise<void>((resolve, reject) => {
      let resolved = false;

      const handleOpen = (): void => {
        resolved = true;
        socket.removeEventListener("open", handleOpen);
        handlers.onJoin?.(this.joinResponse!);
        this.startHeartbeat();
        resolve();
      };

      const handleMessage = (event: MessageEvent<string>): void => {
        let messagePayload: unknown = event.data;
        let messageType = "unknown";
        const receivedAt = Date.now();

        if (typeof event.data === "string") {
          try {
            const parsed = JSON.parse(event.data) as unknown;
            messagePayload = parsed;
            if (parsed && typeof parsed === "object" && "type" in parsed) {
              const candidate = (parsed as { readonly type?: unknown }).type;
              if (typeof candidate === "string" && candidate.length > 0) {
                messageType = candidate;
              }
            }
          } catch (error) {
            messagePayload = event.data;
            if (handlers.onError) {
              const cause = error instanceof Error ? error : new Error(String(error));
              handlers.onError(cause);
            }
          }
        }

        if (messageType === "heartbeat") {
          const ack = this.parseHeartbeatAck(messagePayload, receivedAt);
          if (ack) {
            handlers.onHeartbeat?.(ack);
          }
          return;
        }

        const envelope: NetworkMessageEnvelope = {
          type: messageType,
          payload: messagePayload,
          receivedAt,
        };

        handlers.onMessage?.(envelope);
      };

      const handleClose = (event: CloseEvent): void => {
        socket.removeEventListener("message", handleMessage);
        socket.removeEventListener("close", handleClose);
        socket.removeEventListener("error", handleError);

        if (this.socket === socket) {
          this.socket = null;
        }

        this.stopHeartbeat();

        if (!resolved) {
          resolved = true;
          const reasonText = event.reason ? `, reason ${event.reason}` : "";
          reject(
            new Error(
              `WebSocket closed before establishing session (code ${event.code}${reasonText})`,
            ),
          );
          return;
        }

        handlers.onDisconnect?.(event.code, event.reason);
        if (this.handlers === handlers) {
          this.handlers = null;
        }
      };

      const handleError = (event: Event): void => {
        const error =
          event instanceof ErrorEvent && event.error instanceof Error
            ? event.error
            : new Error("WebSocket connection error.");

        if (!resolved) {
          resolved = true;
          socket.removeEventListener("open", handleOpen);
          socket.removeEventListener("message", handleMessage);
          socket.removeEventListener("close", handleClose);
          socket.removeEventListener("error", handleError);
          if (this.socket === socket) {
            this.socket = null;
          }
          this.stopHeartbeat();
          reject(error);
          return;
        }

        handlers.onError?.(error);
      };

      socket.addEventListener("open", handleOpen);
      socket.addEventListener("message", handleMessage);
      socket.addEventListener("close", handleClose);
      socket.addEventListener("error", handleError);
    });
  }

  async disconnect(): Promise<void> {
    const socket = this.socket;
    if (!socket) {
      this.handlers = null;
      return;
    }

    await new Promise<void>((resolve) => {
      const handleClose = (): void => {
        socket.removeEventListener("close", handleClose);
        resolve();
      };

      if (socket.readyState === WebSocket.CLOSED) {
        resolve();
        return;
      }

      socket.addEventListener("close", handleClose);
      socket.close();
    });

    if (this.socket === socket) {
      this.socket = null;
    }
    this.handlers = null;
    this.stopHeartbeat();
  }

  send(data: unknown): void {
    const socket = this.socket;
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      throw new Error("Cannot send message: WebSocket is not connected.");
    }

    const payload = typeof data === "string" ? data : JSON.stringify(data);
    socket.send(payload);
  }

  private createWebSocketUrl(playerId: string): string {
    const { websocketUrl } = this.configuration;
    const url = websocketUrl.startsWith("ws:") || websocketUrl.startsWith("wss:")
      ? new URL(websocketUrl)
      : new URL(websocketUrl, window.location.origin);

    if (url.protocol === "http:" || url.protocol === "https:") {
      url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
    }

    url.searchParams.set("id", playerId);
    return url.toString();
  }

  private startHeartbeat(): void {
    this.stopHeartbeat();
    this.sendHeartbeat();
    const interval = this.configuration.heartbeatIntervalMs;
    if (!Number.isFinite(interval) || interval <= 0) {
      return;
    }
    this.heartbeatTimer = window.setInterval(() => {
      this.sendHeartbeat();
    }, Math.floor(interval));
  }

  private stopHeartbeat(): void {
    if (this.heartbeatTimer !== null) {
      window.clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
    this.lastHeartbeatSentAt = null;
  }

  private sendHeartbeat(): void {
    const socket = this.socket;
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      return;
    }

    const sentAt = Date.now();
    const payload: Record<string, unknown> = {
      type: "heartbeat",
      sentAt,
      ver: this.configuration.protocolVersion,
    };

    try {
      this.send(payload);
      this.lastHeartbeatSentAt = sentAt;
    } catch (error) {
      const handlers = this.handlers;
      if (handlers?.onError) {
        const cause = error instanceof Error ? error : new Error(String(error));
        handlers.onError(cause);
      }
    }
  }

  private parseHeartbeatAck(payload: unknown, receivedAt: number): HeartbeatAck | null {
    if (!payload || typeof payload !== "object") {
      return null;
    }

    const data = payload as {
      readonly serverTime?: unknown;
      readonly clientTime?: unknown;
      readonly rtt?: unknown;
    };

    const serverTime = typeof data.serverTime === "number" && Number.isFinite(data.serverTime)
      ? data.serverTime
      : null;
    const clientTime = typeof data.clientTime === "number" && Number.isFinite(data.clientTime)
      ? data.clientTime
      : this.lastHeartbeatSentAt;

    let roundTripTime: number | null = null;
    if (typeof data.rtt === "number" && Number.isFinite(data.rtt)) {
      roundTripTime = Math.max(0, data.rtt);
    } else if (clientTime !== null) {
      const fallback = receivedAt - clientTime;
      if (Number.isFinite(fallback)) {
        roundTripTime = Math.max(0, fallback);
      }
    }

    return {
      serverTime,
      clientTime,
      roundTripTime,
      receivedAt,
    };
  }
}
