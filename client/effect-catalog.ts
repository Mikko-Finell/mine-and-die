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
