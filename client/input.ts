export type FacingDirection = "up" | "down" | "left" | "right";

export interface IntentVector {
  readonly dx: number;
  readonly dy: number;
}

export interface PlayerIntent extends IntentVector {
  readonly facing: FacingDirection;
}

export interface InputStateSnapshot {
  readonly pressedKeys: ReadonlySet<string>;
  readonly directionOrder: readonly string[];
  readonly currentFacing: FacingDirection;
  readonly pathActive: boolean;
}

export interface InputStore {
  readonly getState: () => InputStateSnapshot;
  readonly setIntent: (intent: PlayerIntent) => void;
  readonly updateFacing: (facing: FacingDirection) => void;
  readonly toggleCameraLock?: () => void;
}

export interface InputActionDispatcher {
  readonly sendAction: (action: string, params?: Record<string, unknown>) => void;
  readonly cancelPath: () => void;
  readonly sendCurrentIntent: (intent: IntentVector) => void;
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

export class KeyboardInputController implements InputController {
  constructor(public readonly configuration: InputControllerConfiguration) {}

  register(): void {
    throw new Error("Keyboard input controller registration is not implemented.");
  }

  unregister(): void {
    throw new Error("Keyboard input controller unregistration is not implemented.");
  }
}
