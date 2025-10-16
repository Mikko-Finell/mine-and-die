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
  constructor(public readonly configuration: NetworkClientConfiguration) {}

  async join(): Promise<JoinResponse> {
    throw new Error("Network join is not implemented.");
  }

  async connect(_handlers: NetworkEventHandlers): Promise<void> {
    throw new Error("Network connect is not implemented.");
  }

  async disconnect(): Promise<void> {
    throw new Error("Network disconnect is not implemented.");
  }

  send(_data: unknown): void {
    throw new Error("Network send is not implemented.");
  }
}
