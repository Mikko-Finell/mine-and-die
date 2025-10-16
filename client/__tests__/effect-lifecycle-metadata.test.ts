import { afterEach, beforeEach, describe, expect, test } from "vitest";

import {
  normalizeEffectCatalog,
  setEffectCatalog,
  type EffectCatalogSnapshot,
} from "../effect-catalog";
import { isLifecycleClientManaged } from "../effect-lifecycle-metadata";

describe("effect lifecycle metadata", () => {
  const bootstrapCatalog = (entries: EffectCatalogSnapshot): void => {
    setEffectCatalog(null);
    setEffectCatalog(entries);
  };

  beforeEach(() => {
    setEffectCatalog(null);
  });

  afterEach(() => {
    setEffectCatalog(null);
  });

  test("reports client-managed lifecycle entries from catalog metadata", () => {
    bootstrapCatalog(
      normalizeEffectCatalog({
        slash: { contractId: "attack", managedByClient: true },
      }),
    );

    expect(
      isLifecycleClientManaged({
        entryId: "slash",
      }),
    ).toBe(true);

    expect(
      isLifecycleClientManaged({
        instance: { entryId: "slash" },
      }),
    ).toBe(true);
  });

  test("treats entries without catalog matches as server-managed", () => {
    bootstrapCatalog(
      normalizeEffectCatalog({
        splash: { contractId: "blood-splatter", managedByClient: false },
      }),
    );

    expect(
      isLifecycleClientManaged({
        entryId: "splash",
      }),
    ).toBe(false);

    expect(
      isLifecycleClientManaged({
        entryId: "unknown-entry",
      }),
    ).toBe(false);
  });

  test("ignores nested heuristics when catalog ownership is unavailable", () => {
    bootstrapCatalog(
      normalizeEffectCatalog({
        slash: { contractId: "attack", managedByClient: false },
      }),
    );

    expect(
      isLifecycleClientManaged({
        // No entry identifier is present; the renderer must not rely on
        // nested replication metadata to guess ownership.
        instance: {
          entryId: null,
        },
      }),
    ).toBe(false);
  });
});
