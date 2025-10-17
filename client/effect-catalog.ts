import {
  effectCatalog as generatedEffectCatalog,
  type EffectCatalogEntry,
} from "./generated/effect-contracts";

export type EffectCatalogEntryMetadata = EffectCatalogEntry;

export type EffectCatalogSnapshot = Readonly<Record<string, EffectCatalogEntryMetadata>>;

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null && !Array.isArray(value);

const cloneAndFreeze = <T>(value: T): T => {
  if (value === null || typeof value !== "object") {
    return value;
  }

  if (Array.isArray(value)) {
    const cloned = value.map((item) => cloneAndFreeze(item)) as unknown as T;
    return Object.freeze(cloned) as T;
  }

  const entries: Record<string, unknown> = {};
  for (const [key, entry] of Object.entries(value as Record<string, unknown>)) {
    entries[key] = cloneAndFreeze(entry);
  }
  return Object.freeze(entries) as T;
};

const canonicalCatalog: EffectCatalogSnapshot = cloneAndFreeze(generatedEffectCatalog);

const deepEqual = (left: unknown, right: unknown): boolean => {
  if (Object.is(left, right)) {
    return true;
  }

  if (typeof left !== typeof right) {
    return false;
  }

  if (left === null || right === null) {
    return false;
  }

  if (Array.isArray(left) && Array.isArray(right)) {
    if (left.length !== right.length) {
      return false;
    }
    return left.every((value, index) => deepEqual(value, right[index]));
  }

  if (isRecord(left) && isRecord(right)) {
    const leftKeys = Object.keys(left);
    const rightKeys = Object.keys(right);
    if (leftKeys.length !== rightKeys.length) {
      return false;
    }
    return leftKeys.every((key) => deepEqual(left[key], right[key]));
  }

  return false;
};

const validateAgainstCanonical = (input: unknown): void => {
  if (!isRecord(input)) {
    throw new Error("Effect catalog payload must be an object map of entry metadata.");
  }

  const inputKeys = Object.keys(input);
  const canonicalKeys = Object.keys(canonicalCatalog);

  if (inputKeys.length !== canonicalKeys.length) {
    throw new Error(
      `Effect catalog mismatch: expected ${canonicalKeys.length} entries, received ${inputKeys.length}.`,
    );
  }

  for (const key of inputKeys) {
    const canonicalEntry = canonicalCatalog[key];
    if (!canonicalEntry) {
      throw new Error(`Effect catalog contains unknown entry ${key}.`);
    }
    const candidate = (input as Record<string, unknown>)[key];
    if (!deepEqual(candidate, canonicalEntry)) {
      throw new Error(`Effect catalog entry ${key} does not match generated metadata.`);
    }
  }
};

export const normalizeEffectCatalog = (input: unknown): EffectCatalogSnapshot => {
  validateAgainstCanonical(input);
  return canonicalCatalog;
};

let currentCatalog: EffectCatalogSnapshot = canonicalCatalog;
const catalogListeners = new Set<(catalog: EffectCatalogSnapshot) => void>();

const notifyCatalogListeners = (): void => {
  for (const listener of catalogListeners) {
    try {
      listener(currentCatalog);
    } catch (error) {
      // Listeners are third-party callbacks; swallow errors to avoid
      // interrupting other subscribers.
      void error;
    }
  }
};

export const setEffectCatalog = (catalog: EffectCatalogSnapshot | null | undefined): void => {
  currentCatalog = catalog ? cloneAndFreeze(catalog) : canonicalCatalog;
  notifyCatalogListeners();
};

export const getEffectCatalog = (): EffectCatalogSnapshot => currentCatalog;

export const getEffectCatalogEntry = (id: string): EffectCatalogEntryMetadata | null => {
  if (typeof id !== "string" || id.length === 0) {
    return null;
  }
  return currentCatalog[id] ?? null;
};

export const getEffectCatalogBlock = <T = unknown>(
  id: string,
  blockKey: string,
): T | undefined => {
  const entry = getEffectCatalogEntry(id);
  if (!entry) {
    return undefined;
  }
  const blockValue = entry.blocks[blockKey];
  return blockValue as T | undefined;
};

export const isClientManaged = (
  entry: EffectCatalogEntryMetadata | null | undefined,
): boolean => entry?.managedByClient === true;

export const subscribeEffectCatalog = (
  listener: (catalog: EffectCatalogSnapshot) => void,
): (() => void) => {
  if (typeof listener !== "function") {
    throw new Error("Effect catalog listener must be a function.");
  }
  catalogListeners.add(listener);
  // Emit current snapshot immediately so subscribers can hydrate.
  listener(currentCatalog);
  return () => {
    catalogListeners.delete(listener);
  };
};
