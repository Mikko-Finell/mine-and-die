import { afterEach, beforeEach, describe, expect, test } from "vitest";

import { setEffectCatalog } from "../effect-catalog";
import { isLifecycleClientManaged } from "../effect-lifecycle-metadata";
import { effectCatalog as generatedEffectCatalog } from "../generated/effect-contracts";

describe("effect lifecycle metadata", () => {
  beforeEach(() => {
    setEffectCatalog(null);
  });

  afterEach(() => {
    setEffectCatalog(null);
  });

  test("reports client-managed lifecycle entries from catalog metadata", () => {
    setEffectCatalog({ attack: generatedEffectCatalog.attack });

    expect(
      isLifecycleClientManaged({
        entryId: "attack",
      }),
    ).toBe(true);

    expect(
      isLifecycleClientManaged({
        instance: { entryId: "attack" },
      }),
    ).toBe(true);
  });

  test("treats entries without catalog matches as server-managed", () => {
    setEffectCatalog({ fireball: generatedEffectCatalog.fireball });

    expect(
      isLifecycleClientManaged({
        entryId: "fireball",
      }),
    ).toBe(false);

    expect(
      isLifecycleClientManaged({
        entryId: "unknown-entry",
      }),
    ).toBe(false);
  });

  test("ignores nested heuristics when catalog ownership is unavailable", () => {
    setEffectCatalog({ attack: generatedEffectCatalog.attack });

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
