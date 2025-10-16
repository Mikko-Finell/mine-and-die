import {
  getEffectCatalogEntry,
  type EffectCatalogEntryMetadata,
} from "./effect-catalog";

export interface LifecycleInstanceLike {
  readonly entryId?: string | null;
}

export interface LifecycleEntryLike {
  readonly entryId?: string | null;
  readonly instance?: LifecycleInstanceLike | null;
}

const selectEntryId = (entry: LifecycleEntryLike): string | null => {
  if (typeof entry.entryId === "string" && entry.entryId.length > 0) {
    return entry.entryId;
  }

  const instance = entry.instance;
  if (instance && typeof instance.entryId === "string" && instance.entryId.length > 0) {
    return instance.entryId;
  }

  return null;
};

export const resolveLifecycleCatalogEntry = (
  entry: LifecycleEntryLike | null | undefined,
): EffectCatalogEntryMetadata | null => {
  if (!entry) {
    return null;
  }

  const entryId = selectEntryId(entry);
  if (!entryId) {
    return null;
  }

  return getEffectCatalogEntry(entryId);
};

export const isLifecycleClientManaged = (
  entry: LifecycleEntryLike | null | undefined,
): boolean => resolveLifecycleCatalogEntry(entry)?.managedByClient === true;
