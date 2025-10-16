export interface JoinResponse {
  readonly id: string;
  readonly seed: string;
  readonly protocolVersion: number;
}

export interface NetworkMessageEnvelope {
  readonly type: "keyframe" | "patch" | "heartbeat" | "error" | string;
  readonly payload: unknown;
  readonly receivedAt: number;
}

export interface NetworkEventHandlers {
  readonly onJoin?: (response: JoinResponse) => void;
  readonly onMessage?: (message: NetworkMessageEnvelope) => void;
  readonly onDisconnect?: (code?: number, reason?: string) => void;
  readonly onError?: (error: Error) => void;
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

  constructor(public readonly configuration: NetworkClientConfiguration) {}

  async join(): Promise<JoinResponse> {
    await this.disconnect();
    this.joinResponse = null;

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
    };

    if (typeof joinPayload.id !== "string" || joinPayload.id.length === 0) {
      throw new Error("Join response missing player identifier.");
    }

    if (!joinPayload.config || typeof joinPayload.config !== "object") {
      throw new Error("Join response missing world configuration.");
    }

    const config = joinPayload.config as { readonly seed?: unknown };
    if (typeof config.seed !== "string" || config.seed.length === 0) {
      throw new Error("Join response missing world seed.");
    }

    if (typeof joinPayload.ver !== "number") {
      throw new Error("Join response missing protocol version.");
    }

    if (joinPayload.ver !== this.configuration.protocolVersion) {
      throw new Error(
        `Protocol mismatch: expected ${this.configuration.protocolVersion}, received ${joinPayload.ver}`,
      );
    }

    const joinResponse: JoinResponse = {
      id: joinPayload.id,
      seed: config.seed,
      protocolVersion: joinPayload.ver,
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
        resolve();
      };

      const handleMessage = (event: MessageEvent<string>): void => {
        let messagePayload: unknown = event.data;
        let messageType = "unknown";

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

        const envelope: NetworkMessageEnvelope = {
          type: messageType,
          payload: messagePayload,
          receivedAt: Date.now(),
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
}
