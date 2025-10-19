# Remove server-sent catalog: minimal, safe plan

## Decision

* Canonical source of truth: the **generated client catalog**.
* The **server will not send the catalog**; it will expose a **stable schema hash/version** only.

**How to use this document**
1. Pick the first thing that is not marked [DONE]
2. Start working on it
3. When finished mark as [DONE] or [IN PRORGESS] if it's large and you were unable to compelete it
4. Always add a next task so the next contributor can easily pick up where you left off

**Kickoff**

* [DONE] Create this plan

***Next task**
Start on the generator work. Describe the `next task` in one paragraph of technical detail when done.

## What changes (conceptually)

**Generator**

* [TODO] Produce a stable hash/version of the canonical catalog.
* [TODO] Make that hash available to both client and server artifacts (no specifics on how).

**Server**

* [TODO] Stop including the catalog in join/resync responses.
* [TODO] Include only the catalog hash/version in the handshake.
* [TODO] (Optional) Keep a temporary debug endpoint/flag to fetch the catalog during migration; default off.

**Client**

* [TODO] Remove validation of server-provided catalog.
* [TODO] On join, compare server hash vs local generated hash.
* [TODO] If mismatch → clear compatibility error; otherwise use the local generated catalog.

## Tests (acceptance criteria)

* **Generator drift test:** If the generator output changes, the published hash changes in both client and server artifacts together, or CI fails.
* **Server contract test:** Join/resync payloads contain the hash/version and **do not** contain the catalog.
* **Client handshake tests:**

  * Matching hash → proceeds.
  * Mismatched hash → fails with a specific error.
* **E2E canary:** Boot client+server built from the same commit; join succeeds without transferring a catalog.

## CI/Quality gates

* [TODO] Regeneration check: fail if generated artifacts changed but weren’t committed.
* [TODO] Cross-artifact check: fail if client’s hash and server’s hash disagree.
* [TODO] Size check (optional): assert join payload shrinks vs baseline.

## Rollout sequence

1. Add hash emission to the generator and wire it into both sides.
2. Teach the client to compare hashes (keep old path behind a feature flag).
3. Flip the server to stop sending catalogs; send only the hash.
4. Remove the old client path and the server feature flag once canary is green.

## Migration & ops

* [TODO] Short compatibility window: server can send both hash and catalog; client prefers hash path.
* [TODO] Telemetry: log both hashes on join for a week to confirm fleet alignment.
* [TODO] Clear operator message for hash mismatches (action: rebuild/update one side).

## Risks & mitigations

* **Clock skew of releases:** Use the overlap window (both paths available) to avoid breakage.
* **Hash instability:** Ensure deterministic serialization before hashing.
* **Hidden consumers of the old endpoint:** Deprecation notice and a brief audit; cut after the window.

## Outcome

* Smaller, simpler protocol.
* One canonical catalog (the generated one).
* Incompatibility detected instantly via hash, not at runtime via deep JSON checks.
