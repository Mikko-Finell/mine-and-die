export type FacingDirection = "up" | "down" | "left" | "right";

export interface IntentVector {
  readonly dx: number;
  readonly dy: number;
}

export interface PlayerIntent extends IntentVector {
  readonly facing: FacingDirection;
}

export interface PathTarget {
  readonly x: number;
  readonly y: number;
}

export interface PathCommandState {
  readonly active: boolean;
  readonly target: PathTarget | null;
}

export interface InputStateSnapshot {
  readonly pressedKeys: ReadonlySet<string>;
  readonly directionOrder: readonly string[];
  readonly currentFacing: FacingDirection;
  readonly pathActive: boolean;
  readonly pathTarget: PathTarget | null;
}

export interface InputKeyState {
  readonly pressedKeys: ReadonlySet<string>;
  readonly directionOrder: readonly string[];
}

export interface InputStore {
  readonly getState: () => InputStateSnapshot;
  readonly setIntent: (intent: PlayerIntent) => void;
  readonly updateFacing: (facing: FacingDirection) => void;
  readonly toggleCameraLock?: () => void;
  readonly setPathActive?: (pathActive: boolean) => void;
  readonly setPathTarget?: (target: PathTarget | null) => void;
  readonly setKeyState?: (state: InputKeyState) => void;
  readonly getPathTarget?: () => PathTarget | null;
}

export interface InputStoreOptions {
  readonly initialFacing?: FacingDirection;
  readonly initialPathActive?: boolean;
  readonly initialPathTarget?: PathTarget | null;
  readonly initialPressedKeys?: Iterable<string>;
  readonly initialDirectionOrder?: readonly string[];
  readonly initialCameraLocked?: boolean;
  readonly onIntentChanged?: (intent: PlayerIntent) => void;
  readonly onFacingChanged?: (facing: FacingDirection) => void;
  readonly onPathActiveChanged?: (pathActive: boolean) => void;
  readonly onPathTargetChanged?: (target: PathTarget | null) => void;
  readonly onCameraLockToggle?: (locked: boolean) => void;
}

export interface CommandAcknowledgement {
  readonly sequence: number;
  readonly tick?: number | null;
}

export interface CommandRejection {
  readonly sequence: number;
  readonly reason: string;
  readonly retry: boolean;
  readonly tick?: number | null;
}

export interface InputActionDispatcher {
  readonly sendAction: (action: string, params?: Record<string, unknown>) => void;
  readonly cancelPath: () => void;
  readonly sendCurrentIntent: (intent: PlayerIntent) => void;
  readonly sendPathCommand: (target: PathTarget) => void;
  readonly handleCommandAck: (ack: CommandAcknowledgement) => void;
  readonly handleCommandReject: (rejection: CommandRejection) => void;
  readonly handleResync: () => void;
  readonly handleDispatchResume: () => void;
}

export interface InputBindings {
  readonly attackAction: string;
  readonly fireballAction: string;
  readonly cameraLockKey: string;
  readonly movementKeys: Readonly<Record<string, FacingDirection>>;
}

export interface InputControllerConfiguration {
  readonly store: InputStore;
  readonly dispatcher: InputActionDispatcher;
  readonly bindings: InputBindings;
}

export interface InputController {
  readonly register: () => void;
  readonly unregister: () => void;
}

interface DispatcherOptions {
  readonly getProtocolVersion: () => number | null;
  readonly getAcknowledgedTick: () => number | null;
  readonly sendMessage: (payload: Record<string, unknown>) => void;
  readonly isDispatchPaused?: () => boolean;
  readonly onIntentDispatched?: (intent: PlayerIntent) => void;
  readonly onPathCommand?: (state: PathCommandState) => void;
}

type CommandKind = "input" | "path" | "cancelPath" | "action";

interface PendingCommandEntry {
  readonly sequence: number;
  readonly kind: CommandKind;
  payload: Record<string, unknown>;
  sent: boolean;
  retries: number;
  lastAttemptAt: number;
  intent?: PlayerIntent;
  pathTarget?: PathTarget | null;
}

const COMMAND_RETRY_BASE_DELAY_MS = 50;
const COMMAND_RETRY_MAX_DELAY_MS = 400;
const COMMAND_RETRY_MULTIPLIER = 2;

const DEFAULT_FACING: FacingDirection = "down";

const normalizeKey = (event: KeyboardEvent): string => {
  const key = event.key;
  if (typeof key === "string" && key.length === 1) {
    return key.toLowerCase();
  }
  return key;
};

const clampFinite = (value: number): number => {
  if (!Number.isFinite(value)) {
    return 0;
  }
  if (value > 1) {
    return 1;
  }
  if (value < -1) {
    return -1;
  }
  return value;
};

const cloneIntent = (intent: PlayerIntent): PlayerIntent => ({
  dx: intent.dx,
  dy: intent.dy,
  facing: intent.facing,
});

const normalizeKeyCollection = (keys: Iterable<string>): Set<string> => {
  const normalized = new Set<string>();
  for (const key of keys) {
    if (typeof key === "string" && key.length > 0) {
      normalized.add(key.toLowerCase());
    }
  }
  return normalized;
};

const normalizeDirectionOrder = (keys: readonly string[]): string[] =>
  keys.map((key) => key.toLowerCase()).filter((key) => key.length > 0);

export class InMemoryInputStore implements InputStore {
  private readonly onIntentChanged?: (intent: PlayerIntent) => void;
  private readonly onFacingChanged?: (facing: FacingDirection) => void;
  private readonly onPathActiveChanged?: (pathActive: boolean) => void;
  private readonly onPathTargetChanged?: (target: PathTarget | null) => void;
  private readonly onCameraLockToggle?: (locked: boolean) => void;

  private readonly pressedKeys: Set<string>;
  private directionOrder: string[];
  private currentFacing: FacingDirection;
  private pathActive: boolean;
  private pathTarget: PathTarget | null;
  private cameraLocked: boolean;
  private lastIntent: PlayerIntent;

  constructor(options: InputStoreOptions = {}) {
    this.onIntentChanged = options.onIntentChanged;
    this.onFacingChanged = options.onFacingChanged;
    this.onPathActiveChanged = options.onPathActiveChanged;
    this.onPathTargetChanged = options.onPathTargetChanged;
    this.onCameraLockToggle = options.onCameraLockToggle;
    this.pressedKeys = normalizeKeyCollection(options.initialPressedKeys ?? []);
    this.directionOrder = normalizeDirectionOrder(options.initialDirectionOrder ?? []);
    this.currentFacing = options.initialFacing ?? DEFAULT_FACING;
    this.pathActive = options.initialPathActive ?? false;
    this.pathTarget = options.initialPathTarget ? { ...options.initialPathTarget } : null;
    this.cameraLocked = options.initialCameraLocked ?? false;
    this.lastIntent = { dx: 0, dy: 0, facing: this.currentFacing };
  }

  getState(): InputStateSnapshot {
    return {
      pressedKeys: new Set(this.pressedKeys),
      directionOrder: [...this.directionOrder],
      currentFacing: this.currentFacing,
      pathActive: this.pathActive,
      pathTarget: this.pathTarget ? { ...this.pathTarget } : null,
    };
  }

  setIntent(intent: PlayerIntent): void {
    const snapshot = cloneIntent(intent);
    this.lastIntent = snapshot;
    this.currentFacing = snapshot.facing;
    this.onIntentChanged?.(cloneIntent(snapshot));
  }

  updateFacing(facing: FacingDirection): void {
    if (this.currentFacing === facing) {
      return;
    }
    this.currentFacing = facing;
    this.onFacingChanged?.(facing);
  }

  toggleCameraLock(): void {
    this.cameraLocked = !this.cameraLocked;
    this.onCameraLockToggle?.(this.cameraLocked);
  }

  setPathActive(pathActive: boolean): void {
    if (this.pathActive === pathActive) {
      return;
    }
    this.pathActive = pathActive;
    this.onPathActiveChanged?.(pathActive);
    if (!pathActive) {
      this.setPathTarget(null);
    }
  }

  setKeyState(state: InputKeyState): void {
    this.pressedKeys.clear();
    for (const key of state.pressedKeys) {
      if (typeof key === "string" && key.length > 0) {
        this.pressedKeys.add(key.toLowerCase());
      }
    }
    this.directionOrder = normalizeDirectionOrder(state.directionOrder);
  }

  setPathTarget(target: PathTarget | null): void {
    const nextTarget = target ? { x: target.x, y: target.y } : null;
    const hasChanged =
      (this.pathTarget?.x ?? null) !== (nextTarget?.x ?? null) ||
      (this.pathTarget?.y ?? null) !== (nextTarget?.y ?? null);
    if (!hasChanged) {
      return;
    }
    this.pathTarget = nextTarget;
    this.onPathTargetChanged?.(this.pathTarget ? { ...this.pathTarget } : null);
  }

  isCameraLocked(): boolean {
    return this.cameraLocked;
  }

  getLastIntent(): PlayerIntent {
    return cloneIntent(this.lastIntent);
  }

  getPathTarget(): PathTarget | null {
    return this.pathTarget ? { ...this.pathTarget } : null;
  }
}

export class NetworkInputActionDispatcher implements InputActionDispatcher {
  private nextSequence = 1;
  private readonly pendingCommands: PendingCommandEntry[] = [];
  private readonly retryTimers = new Map<number, ReturnType<typeof setTimeout>>();
  private lastIntent: PlayerIntent | null = null;

  constructor(private readonly options: DispatcherOptions) {}

  sendAction(action: string, params?: Record<string, unknown>): void {
    if (!action) {
      return;
    }
    const payload: Record<string, unknown> = { type: "action", action };
    if (params && typeof params === "object") {
      payload.params = params;
    }
    this.queueCommand("action", payload);
  }

  cancelPath(): void {
    this.queueCommand("cancelPath", { type: "cancelPath" });
    this.options.onPathCommand?.({ active: false, target: null });
  }

  sendPathCommand(target: PathTarget): void {
    if (!Number.isFinite(target?.x) || !Number.isFinite(target?.y)) {
      return;
    }
    this.queueCommand(
      "path",
      { type: "path", x: target.x, y: target.y },
      { pathTarget: { x: target.x, y: target.y } },
    );
    this.options.onPathCommand?.({
      active: true,
      target: { x: target.x, y: target.y },
    });
  }

  sendCurrentIntent(intent: PlayerIntent): void {
    const dx = clampFinite(intent.dx);
    const dy = clampFinite(intent.dy);
    const snapshot: PlayerIntent = { dx, dy, facing: intent.facing };
    this.lastIntent = cloneIntent(snapshot);
    this.queueCommand("input", { type: "input", dx, dy, facing: intent.facing }, { intent: snapshot });
    this.options.onIntentDispatched?.(cloneIntent(snapshot));
  }

  handleCommandAck(ack: CommandAcknowledgement): void {
    const sequence = this.normalizeSequence(ack?.sequence);
    if (sequence === null) {
      return;
    }
    const index = this.pendingCommands.findIndex((entry) => entry.sequence === sequence);
    if (index === -1) {
      return;
    }
    this.clearRetry(sequence);
    this.pendingCommands.splice(index, 1);
    this.flushPending();
  }

  handleCommandReject(rejection: CommandRejection): void {
    const sequence = this.normalizeSequence(rejection?.sequence);
    if (sequence === null) {
      return;
    }
    const entryIndex = this.pendingCommands.findIndex((candidate) => candidate.sequence === sequence);
    if (entryIndex === -1) {
      return;
    }
    const entry = this.pendingCommands[entryIndex];
    this.clearRetry(sequence);
    if (!rejection?.retry) {
      this.pendingCommands.splice(entryIndex, 1);
      this.flushPending();
      return;
    }
    entry.sent = false;
    const nextAttempt = Math.min(entry.retries + 1, 100);
    entry.retries = nextAttempt;
    this.scheduleRetry(entry, nextAttempt);
  }

  handleResync(): void {
    for (const timer of this.retryTimers.values()) {
      clearTimeout(timer);
    }
    this.retryTimers.clear();
    for (const entry of this.pendingCommands) {
      entry.sent = false;
      entry.retries = 0;
      entry.lastAttemptAt = 0;
    }
  }

  handleDispatchResume(): void {
    this.flushPending();
  }

  private queueCommand(
    kind: CommandKind,
    payload: Record<string, unknown>,
    extras: { intent?: PlayerIntent; pathTarget?: PathTarget | null } = {},
  ): void {
    const sequence = this.nextSequence++;
    const entry: PendingCommandEntry = {
      sequence,
      kind,
      payload: { ...payload },
      sent: false,
      retries: 0,
      lastAttemptAt: 0,
      intent: extras.intent ? cloneIntent(extras.intent) : extras.intent,
      pathTarget: extras.pathTarget ? { ...extras.pathTarget } : extras.pathTarget ?? null,
    };
    this.pendingCommands.push(entry);
    this.flushPending();
  }

  private flushPending(): void {
    if (this.isDispatchPaused()) {
      return;
    }
    for (const entry of this.pendingCommands) {
      if (entry.sent || this.retryTimers.has(entry.sequence)) {
        continue;
      }
      this.refreshEntryPayload(entry);
      this.dispatch(entry);
      if (this.isDispatchPaused()) {
        return;
      }
    }
  }

  private dispatch(entry: PendingCommandEntry): void {
    if (this.isDispatchPaused()) {
      entry.sent = false;
      return;
    }
    const message = this.attachMetadata({ ...entry.payload }, entry.sequence);
    this.options.sendMessage(message);
    entry.sent = true;
    entry.lastAttemptAt = Date.now();
  }

  private refreshEntryPayload(entry: PendingCommandEntry): void {
    switch (entry.kind) {
      case "input": {
        const intent = entry.intent ?? this.lastIntent;
        if (intent) {
          const dx = clampFinite(intent.dx);
          const dy = clampFinite(intent.dy);
          entry.payload = { type: "input", dx, dy, facing: intent.facing };
        }
        break;
      }
      case "path": {
        const target = entry.pathTarget;
        if (target) {
          entry.payload = { type: "path", x: target.x, y: target.y };
        }
        break;
      }
      case "cancelPath": {
        entry.payload = { type: "cancelPath" };
        break;
      }
      default:
        break;
    }
  }

  private scheduleRetry(entry: PendingCommandEntry, attempt: number): void {
    let delay = COMMAND_RETRY_BASE_DELAY_MS * Math.pow(COMMAND_RETRY_MULTIPLIER, Math.max(1, attempt) - 1);
    if (!Number.isFinite(delay)) {
      delay = COMMAND_RETRY_MAX_DELAY_MS;
    }
    delay = Math.min(delay, COMMAND_RETRY_MAX_DELAY_MS);
    const timer = setTimeout(() => {
      this.retryTimers.delete(entry.sequence);
      entry.sent = false;
      this.refreshEntryPayload(entry);
      this.flushPending();
    }, delay);
    this.retryTimers.set(entry.sequence, timer);
  }

  private clearRetry(sequence: number): void {
    const timer = this.retryTimers.get(sequence);
    if (timer !== undefined) {
      clearTimeout(timer);
      this.retryTimers.delete(sequence);
    }
  }

  private attachMetadata(payload: Record<string, unknown>, sequence: number): Record<string, unknown> {
    const version = this.options.getProtocolVersion();
    if (typeof version === "number" && Number.isFinite(version)) {
      payload.ver = Math.floor(version);
    }
    const ack = this.options.getAcknowledgedTick();
    if (typeof ack === "number" && Number.isFinite(ack) && ack >= 0) {
      payload.ack = Math.floor(ack);
    }
    payload.seq = sequence;
    return payload;
  }

  private normalizeSequence(value: unknown): number | null {
    if (typeof value !== "number" || !Number.isFinite(value)) {
      return null;
    }
    const normalized = Math.floor(value);
    return normalized > 0 ? normalized : null;
  }

  private isDispatchPaused(): boolean {
    return this.options.isDispatchPaused?.() === true;
  }
}

export class KeyboardInputController implements InputController {
  private readonly pressedKeys = new Set<string>();
  private directionOrder: string[] = [];
  private lastIntent: PlayerIntent = { dx: 0, dy: 0, facing: DEFAULT_FACING };
  private readonly movementKeyMap: Map<string, FacingDirection>;
  private readonly cameraLockKey: string;
  private registered = false;

  private readonly handleKeydown = (event: KeyboardEvent): void => {
    this.handleKey(event, true);
  };

  private readonly handleKeyup = (event: KeyboardEvent): void => {
    this.handleKey(event, false);
  };

  private readonly handleWindowBlur = (): void => {
    if (!this.registered) {
      return;
    }
    if (this.pressedKeys.size === 0 && this.lastIntent.dx === 0 && this.lastIntent.dy === 0) {
      return;
    }

    this.pressedKeys.clear();
    this.directionOrder = [];
    this.publishKeyStateSnapshot();
    const { store } = this.configuration;
    const state = store.getState();
    const facing = state.currentFacing ?? DEFAULT_FACING;
    this.dispatchIntent({ dx: 0, dy: 0, facing });
  };

  constructor(public readonly configuration: InputControllerConfiguration) {
    this.movementKeyMap = new Map(
      Object.entries(configuration.bindings.movementKeys).map(([key, facing]) => [key.toLowerCase(), facing]),
    );
    this.cameraLockKey = configuration.bindings.cameraLockKey.toLowerCase();
  }

  register(): void {
    if (this.registered) {
      return;
    }

    this.registered = true;
    const state = this.configuration.store.getState();
    this.pressedKeys.clear();
    for (const key of state.pressedKeys) {
      this.pressedKeys.add(key.toLowerCase());
    }
    this.directionOrder = [...state.directionOrder].map((key) => key.toLowerCase());
    this.lastIntent = {
      dx: 0,
      dy: 0,
      facing: state.currentFacing ?? DEFAULT_FACING,
    };

    document.addEventListener("keydown", this.handleKeydown);
    document.addEventListener("keyup", this.handleKeyup);
    window.addEventListener("blur", this.handleWindowBlur);
  }

  unregister(): void {
    if (!this.registered) {
      return;
    }

    this.registered = false;
    document.removeEventListener("keydown", this.handleKeydown);
    document.removeEventListener("keyup", this.handleKeyup);
    window.removeEventListener("blur", this.handleWindowBlur);
    this.pressedKeys.clear();
    this.directionOrder = [];
    this.lastIntent = { dx: 0, dy: 0, facing: DEFAULT_FACING };
  }

  private handleKey(event: KeyboardEvent, isPressed: boolean): void {
    if (!this.registered) {
      return;
    }

    if (isPressed && !event.repeat && this.isAttackKey(event)) {
      event.preventDefault();
      this.configuration.dispatcher.sendAction(this.configuration.bindings.attackAction);
      return;
    }

    if (isPressed && !event.repeat && this.isFireballKey(event)) {
      event.preventDefault();
      this.configuration.dispatcher.sendAction(this.configuration.bindings.fireballAction);
      return;
    }

    const normalizedKey = normalizeKey(event);

    if (isPressed && !event.repeat && normalizedKey.toLowerCase() === this.cameraLockKey) {
      event.preventDefault();
      this.configuration.store.toggleCameraLock?.();
      return;
    }

    const facing = this.movementKeyMap.get(normalizedKey.toLowerCase());
    if (!facing) {
      return;
    }

    event.preventDefault();

    if (isPressed) {
      if (!event.repeat && this.configuration.store.getState().pathActive) {
        this.configuration.dispatcher.cancelPath();
        this.configuration.store.setPathActive?.(false);
      }

      if (!this.pressedKeys.has(normalizedKey)) {
        this.directionOrder = this.directionOrder.filter((entry) => entry !== normalizedKey);
      this.directionOrder.push(normalizedKey);
    }
    this.pressedKeys.add(normalizedKey);
  } else {
    this.pressedKeys.delete(normalizedKey);
    this.directionOrder = this.directionOrder.filter((entry) => entry !== normalizedKey);
  }

    this.publishKeyStateSnapshot();
    this.updateIntentFromKeys();
  }

  private isAttackKey(event: KeyboardEvent): boolean {
    return event.code === "Space" || event.key === " ";
  }

  private isFireballKey(event: KeyboardEvent): boolean {
    const normalizedKey = normalizeKey(event);
    return normalizedKey === "f" || event.code === "KeyF";
  }

  private updateIntentFromKeys(): void {
    let rawDx = 0;
    let rawDy = 0;

    for (const key of this.pressedKeys) {
      const facing = this.movementKeyMap.get(key);
      if (!facing) {
        continue;
      }
      if (facing === "up") {
        rawDy -= 1;
      } else if (facing === "down") {
        rawDy += 1;
      } else if (facing === "left") {
        rawDx -= 1;
      } else if (facing === "right") {
        rawDx += 1;
      }
    }

    let dx = rawDx;
    let dy = rawDy;
    if (dx !== 0 || dy !== 0) {
      const length = Math.hypot(dx, dy) || 1;
      dx /= length;
      dy /= length;
    }

    const state = this.configuration.store.getState();
    const previousFacing = state.currentFacing ?? DEFAULT_FACING;
    const nextFacing = this.deriveFacing(rawDx, rawDy, previousFacing);

    if (nextFacing !== previousFacing) {
      this.configuration.store.updateFacing(nextFacing);
    }

    if (dx === this.lastIntent.dx && dy === this.lastIntent.dy && nextFacing === this.lastIntent.facing) {
      return;
    }

    this.dispatchIntent({ dx, dy, facing: nextFacing });
  }

  private deriveFacing(rawDx: number, rawDy: number, fallback: FacingDirection): FacingDirection {
    if (rawDx === 0 && rawDy === 0) {
      const lastKey = this.directionOrder[this.directionOrder.length - 1];
      if (lastKey) {
        const resolved = this.movementKeyMap.get(lastKey);
        if (resolved) {
          return resolved;
        }
      }
      return fallback;
    }

    const absX = Math.abs(rawDx);
    const absY = Math.abs(rawDy);
    if (absY >= absX && rawDy !== 0) {
      return rawDy > 0 ? "down" : "up";
    }
    if (rawDx !== 0) {
      return rawDx > 0 ? "right" : "left";
    }
    return fallback;
  }

  private dispatchIntent(intent: PlayerIntent): void {
    this.configuration.store.setIntent(intent);
    this.configuration.dispatcher.sendCurrentIntent(intent);
    this.lastIntent = intent;
  }

  private publishKeyStateSnapshot(): void {
    this.configuration.store.setKeyState?.({
      pressedKeys: this.pressedKeys,
      directionOrder: this.directionOrder,
    });
  }
}
