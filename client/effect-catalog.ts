import type { EffectDefinition, EndPolicy } from "./generated/effect-contracts";

export interface EffectCatalogEntryMetadata {
  readonly contractId: string;
  readonly definition: EffectDefinition | null;
  readonly blocks: Readonly<Record<string, unknown>>;
}

export type EffectCatalogSnapshot = Readonly<Record<string, EffectCatalogEntryMetadata>>;

const EMPTY_CATALOG: EffectCatalogSnapshot = Object.freeze({});

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null && !Array.isArray(value);

const cloneBlocks = (source: Record<string, unknown>): Record<string, unknown> => {
  const copy: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(source)) {
    copy[key] = value;
  }
  return copy;
};

const cloneJsonValue = (value: unknown): unknown => {
  if (Array.isArray(value)) {
    return value.map((item) => cloneJsonValue(item));
  }
  if (isRecord(value)) {
    const copy: Record<string, unknown> = {};
    for (const [key, nested] of Object.entries(value)) {
      copy[key] = cloneJsonValue(nested);
    }
    return copy;
  }
  return value;
};

const freezeJsonValue = <T>(value: T): T => {
  if (Array.isArray(value)) {
    for (const item of value) {
      freezeJsonValue(item);
    }
    return Object.freeze(value) as typeof value;
  }
  if (isRecord(value)) {
    for (const nested of Object.values(value)) {
      freezeJsonValue(nested);
    }
    return Object.freeze(value) as typeof value;
  }
  return value;
};

const DEFAULT_END_POLICY: EndPolicy = Object.freeze({ kind: 0 });

const cloneEffectDefinition = (source: Record<string, unknown>): EffectDefinition => {
  const cloned = cloneJsonValue(source) as Partial<EffectDefinition>;
  const endValue = cloned.end;
  if (!isRecord(endValue)) {
    cloned.end = DEFAULT_END_POLICY;
  }
  return freezeJsonValue(cloned) as EffectDefinition;
};

export const normalizeEffectCatalog = (input: unknown): EffectCatalogSnapshot => {
  if (input == null) {
    return EMPTY_CATALOG;
  }
  if (!isRecord(input)) {
    throw new Error("Effect catalog must be an object map of entry metadata.");
  }
  const result: Record<string, EffectCatalogEntryMetadata> = {};
  for (const [entryId, entryValue] of Object.entries(input)) {
    if (!isRecord(entryValue)) {
      throw new Error(`Effect catalog entry ${entryId} must be an object.`);
    }
    const { contractId, definition, blocks } = entryValue as {
      readonly contractId?: unknown;
      readonly definition?: unknown;
      readonly blocks?: unknown;
    };
    if (typeof contractId !== "string" || contractId.length === 0) {
      throw new Error(`Effect catalog entry ${entryId} missing contractId.`);
    }
    let normalizedDefinition: EffectDefinition | null = null;
    if (definition !== undefined && definition !== null) {
      if (!isRecord(definition)) {
        throw new Error(`Effect catalog entry ${entryId} definition must be an object.`);
      }
      normalizedDefinition = cloneEffectDefinition(definition);
    }
    let normalizedBlocks: Record<string, unknown> | undefined;
    if (blocks !== undefined) {
      if (!isRecord(blocks)) {
        throw new Error(`Effect catalog entry ${entryId} blocks must be an object.`);
      }
      normalizedBlocks = cloneBlocks(blocks);
    }
    result[entryId] = Object.freeze({
      contractId,
      definition: normalizedDefinition,
      blocks: Object.freeze(normalizedBlocks ?? {}),
    });
  }
  return Object.freeze(result);
};

let currentCatalog: EffectCatalogSnapshot = EMPTY_CATALOG;
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
  currentCatalog = catalog ? Object.freeze({ ...catalog }) : EMPTY_CATALOG;
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
