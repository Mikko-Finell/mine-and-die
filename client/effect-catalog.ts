export interface EffectCatalogEntryMetadata {
  readonly contractId: string;
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
    const { contractId, blocks } = entryValue as {
      readonly contractId?: unknown;
      readonly blocks?: unknown;
    };
    if (typeof contractId !== "string" || contractId.length === 0) {
      throw new Error(`Effect catalog entry ${entryId} missing contractId.`);
    }
    let normalizedBlocks: Record<string, unknown> | undefined;
    if (blocks !== undefined) {
      if (!isRecord(blocks)) {
        throw new Error(`Effect catalog entry ${entryId} blocks must be an object.`);
      }
      normalizedBlocks = cloneBlocks(blocks);
    }
    result[entryId] = {
      contractId,
      blocks: Object.freeze(normalizedBlocks ?? {}),
    };
  }
  return Object.freeze(result);
};

let currentCatalog: EffectCatalogSnapshot = EMPTY_CATALOG;

export const setEffectCatalog = (catalog: EffectCatalogSnapshot | null | undefined): void => {
  currentCatalog = catalog ? Object.freeze({ ...catalog }) : EMPTY_CATALOG;
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
