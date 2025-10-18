import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  KeyboardInputController,
  type FacingDirection,
  type InputBindings,
  type InputStateSnapshot,
  type InputStore,
  type PlayerIntent,
} from "../input";

interface TestStoreContext {
  readonly store: InputStore;
  readonly intents: PlayerIntent[];
  readonly facingUpdates: FacingDirection[];
  readonly toggleCameraLock: ReturnType<typeof vi.fn>;
  readonly keyStates: InputStateSnapshot[];
  readonly pathTargets: (ReturnType<NonNullable<InputStore["getPathTarget"]>> | null)[];
}

type EventHandler = (event: unknown) => void;

interface ListenerRegistry {
  readonly add: (type: string, handler: EventHandler) => void;
  readonly remove: (type: string, handler: EventHandler) => void;
  readonly dispatch: (type: string, event: unknown) => void;
}

const BINDINGS: InputBindings = {
  attackAction: "attack",
  fireballAction: "fireball",
  cameraLockKey: "c",
  movementKeys: {
    w: "up",
    a: "left",
    s: "down",
    d: "right",
  },
};

const createListenerRegistry = (): ListenerRegistry => {
  const listeners = new Map<string, Set<EventHandler>>();

  return {
    add: (type, handler) => {
      let handlers = listeners.get(type);
      if (!handlers) {
        handlers = new Set<EventHandler>();
        listeners.set(type, handlers);
      }
      handlers.add(handler);
    },
    remove: (type, handler) => {
      listeners.get(type)?.delete(handler);
    },
    dispatch: (type, event) => {
      const handlers = listeners.get(type);
      if (!handlers) {
        return;
      }
      for (const handler of Array.from(handlers)) {
        handler(event);
      }
    },
  };
};

const createKeyboardEvent = (init: { key?: string; code?: string; repeat?: boolean } = {}) => {
  let prevented = false;
  return {
    key: init.key ?? "",
    code: init.code ?? "",
    repeat: init.repeat ?? false,
    get defaultPrevented(): boolean {
      return prevented;
    },
    preventDefault: () => {
      prevented = true;
    },
  };
};

const createTestStore = (snapshot?: Partial<InputStateSnapshot>): TestStoreContext => {
  let currentFacing: FacingDirection = snapshot?.currentFacing ?? "down";
  let pathActive = snapshot?.pathActive ?? false;
  let pathTarget = snapshot?.pathTarget ?? null;
  const intents: PlayerIntent[] = [];
  const facingUpdates: FacingDirection[] = [];
  const toggleCameraLock = vi.fn();
  const keyStates: InputStateSnapshot[] = [];
  const pathTargets: (ReturnType<NonNullable<InputStore["getPathTarget"]>> | null)[] = [];

  const baseSnapshot: InputStateSnapshot = {
    pressedKeys: snapshot?.pressedKeys ?? new Set<string>(),
    directionOrder: snapshot?.directionOrder ?? [],
    currentFacing,
    pathActive,
    pathTarget,
    lastCommandRejection: snapshot?.lastCommandRejection ?? null,
  };

  const store: InputStore = {
    getState: () => ({
      pressedKeys: new Set(baseSnapshot.pressedKeys),
      directionOrder: [...baseSnapshot.directionOrder],
      currentFacing,
      pathActive,
      pathTarget,
      lastCommandRejection: baseSnapshot.lastCommandRejection,
    }),
    setIntent: (intent) => {
      intents.push(intent);
      currentFacing = intent.facing;
    },
    updateFacing: (facing) => {
      facingUpdates.push(facing);
      currentFacing = facing;
    },
    toggleCameraLock,
    setPathActive: (value) => {
      pathActive = value;
      if (!value) {
        pathTarget = null;
      }
    },
    setKeyState: (state) => {
      keyStates.push({
        pressedKeys: new Set(state.pressedKeys),
        directionOrder: [...state.directionOrder],
        currentFacing,
        pathActive,
        pathTarget,
      });
    },
    setPathTarget: (target) => {
      pathTarget = target ? { ...target } : null;
      pathTargets.push(store.getPathTarget ? store.getPathTarget() : null);
    },
    getPathTarget: () => (pathTarget ? { ...pathTarget } : null),
  };

  return { store, intents, facingUpdates, toggleCameraLock, keyStates, pathTargets };
};

const createController = (snapshot?: Partial<InputStateSnapshot>) => {
  const testStore = createTestStore(snapshot);
  const dispatcher = {
    sendAction: vi.fn(),
    cancelPath: vi.fn(),
    sendCurrentIntent: vi.fn(),
  } as const;

  const controller = new KeyboardInputController({
    store: testStore.store,
    dispatcher,
    bindings: BINDINGS,
  });

  return { controller, dispatcher, testStore };
};

let documentListeners: ListenerRegistry;
let windowListeners: ListenerRegistry;

const dispatchDocumentEvent = (type: string, event: unknown): void => {
  documentListeners.dispatch(type, event);
};

const dispatchWindowEvent = (type: string, event: unknown): void => {
  windowListeners.dispatch(type, event);
};

beforeEach(() => {
  documentListeners = createListenerRegistry();
  windowListeners = createListenerRegistry();

  const documentStub = {
    addEventListener: vi.fn((type: string, handler: EventHandler) => {
      documentListeners.add(type, handler);
    }),
    removeEventListener: vi.fn((type: string, handler: EventHandler) => {
      documentListeners.remove(type, handler);
    }),
  };

  const windowStub = {
    addEventListener: vi.fn((type: string, handler: EventHandler) => {
      windowListeners.add(type, handler);
    }),
    removeEventListener: vi.fn((type: string, handler: EventHandler) => {
      windowListeners.remove(type, handler);
    }),
  };

  Object.assign(globalThis, {
    document: documentStub,
    window: { ...windowStub },
  });
});

afterEach(() => {
  vi.restoreAllMocks();
  Reflect.deleteProperty(globalThis, "document");
  Reflect.deleteProperty(globalThis, "window");
});

describe("KeyboardInputController", () => {
  it("dispatches movement intents and cancels active paths", () => {
    const { controller, dispatcher, testStore } = createController({ pathActive: true });
    controller.register();

    try {
      const keyDown = createKeyboardEvent({ key: "w" });
      dispatchDocumentEvent("keydown", keyDown);

      expect(keyDown.defaultPrevented).toBe(true);
      expect(dispatcher.cancelPath).toHaveBeenCalledTimes(1);
      const firstKeyState = testStore.keyStates[0];
      expect(firstKeyState).toBeDefined();
      expect(Array.from(firstKeyState!.pressedKeys)).toEqual(["w"]);
      expect(firstKeyState!.directionOrder).toEqual(["w"]);
      expect(dispatcher.sendCurrentIntent).toHaveBeenNthCalledWith(1, {
        dx: 0,
        dy: -1,
        facing: "up",
      });
      expect(testStore.intents.at(-1)).toEqual({ dx: 0, dy: -1, facing: "up" });
      expect(testStore.facingUpdates.at(-1)).toBe("up");

      const keyUp = createKeyboardEvent({ key: "w" });
      dispatchDocumentEvent("keyup", keyUp);
      expect(dispatcher.sendCurrentIntent).toHaveBeenLastCalledWith({
        dx: 0,
        dy: 0,
        facing: "up",
      });
      const lastKeyState = testStore.keyStates.at(-1);
      expect(lastKeyState).toBeDefined();
      expect(Array.from(lastKeyState!.pressedKeys)).toEqual([]);
      expect(lastKeyState!.directionOrder).toEqual([]);
    } finally {
      controller.unregister();
    }
  });

  it("ignores repeated key presses when cancelling a path", () => {
    const { controller, dispatcher } = createController({ pathActive: true });
    controller.register();

    try {
      const first = createKeyboardEvent({ key: "d" });
      dispatchDocumentEvent("keydown", first);
      const repeat = createKeyboardEvent({ key: "d", repeat: true });
      dispatchDocumentEvent("keydown", repeat);

      expect(dispatcher.cancelPath).toHaveBeenCalledTimes(1);
    } finally {
      controller.unregister();
    }
  });

  it("fires ability bindings for attack and fireball", () => {
    const { controller, dispatcher } = createController();
    controller.register();

    try {
      const attack = createKeyboardEvent({ code: "Space" });
      dispatchDocumentEvent("keydown", attack);
      const fireball = createKeyboardEvent({ key: "f" });
      dispatchDocumentEvent("keydown", fireball);

      expect(dispatcher.sendAction).toHaveBeenNthCalledWith(1, "attack");
      expect(dispatcher.sendAction).toHaveBeenNthCalledWith(2, "fireball");
    } finally {
      controller.unregister();
    }
  });

  it("toggles camera lock when the binding is pressed", () => {
    const { controller, testStore } = createController();
    controller.register();

    try {
      const event = createKeyboardEvent({ key: "c" });
      dispatchDocumentEvent("keydown", event);
      expect(event.defaultPrevented).toBe(true);
      expect(testStore.toggleCameraLock).toHaveBeenCalledTimes(1);
    } finally {
      controller.unregister();
    }
  });

  it("clears intents on window blur", () => {
    const { controller, dispatcher } = createController();
    controller.register();

    try {
      const keyDown = createKeyboardEvent({ key: "a" });
      dispatchDocumentEvent("keydown", keyDown);
      expect(dispatcher.sendCurrentIntent).toHaveBeenCalledTimes(1);

      dispatchWindowEvent("blur", { type: "blur" });
      expect(dispatcher.sendCurrentIntent).toHaveBeenLastCalledWith({
        dx: 0,
        dy: 0,
        facing: "left",
      });
    } finally {
      controller.unregister();
    }
  });

  it("stops handling events after unregister", () => {
    const { controller, dispatcher } = createController();
    controller.register();
    controller.unregister();

    const event = createKeyboardEvent({ key: "w" });
    dispatchDocumentEvent("keydown", event);

    expect(dispatcher.sendCurrentIntent).not.toHaveBeenCalled();
  });
});
