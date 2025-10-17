# Rendering Backend Abstraction Spec

## Purpose
- Document the seams in the current Canvas-only renderer so contributors understand why it is tightly coupled to the 2D API.
- Lay out the architecture required to swap the draw backend (Canvas, WebGL, sprite sheets) without rewriting orchestration logic.
- Provide concrete refactor steps and ownership boundaries for anyone implementing new renderers.

## Current Entry Points
- **Renderer host:** [`client/render.ts`](../../client/render.ts) mounts an `HTMLCanvasElement`, grabs a `CanvasRenderingContext2D`, and drives draw calls through `CanvasRenderer.renderBatch`.
- **Render payload:** `RenderBatch` mixes geometry intent with Canvas defaults (`fill`, `stroke`) and assumes polygon vertices map directly to 2D path commands.
- **Effects runtime:** [`tools/js-effects`](../../tools/js-effects) injects a raw `CanvasRenderingContext2D` into every effect, and effect scripts perform `ctx.arc`, `ctx.fill`, and gradient calls directly.
- **Decal assets:** Animated decals accept DOM elements or `ImageBitmap` objects rather than abstract texture handles, tying resource management to Canvas-friendly blobs.

## Goals
1. Allow multiple renderer backends (Canvas2D, WebGL2, sprite sheet compositor) to mount without touching the client orchestrator or effect scripts.
2. Represent render intent as backend-agnostic commands so the same batch feeds any renderer.
3. Keep effects deterministic while allowing each backend to optimize shaders, buffers, or blits independently.
4. Normalize asset handles so the renderer, not application code, manages GPU resources, atlases, or cached canvases.

## Architectural Layers
| Layer | Responsibility | Notes |
| --- | --- | --- |
| `RenderSurface` | Lifecycle hooks (`mount`, `unmount`, `resize`) and submission (`submitBatch`) with no DOM types. | Canvas/WebGL implementations adapt to DOM specifics. |
| `RenderCommandBuffer` | Immutable list of typed draw commands (polygons, sprites, billboards, debug overlays). | Produced by orchestration and effects code. |
| `EffectRenderer` | Backend-neutral helper exposing `drawCircle`, `drawSprite`, `drawTrail`, etc. | Implemented per backend; effect scripts use this API only. |
| `AssetRegistry` | Maps logical asset IDs to backend resources (patterns, textures, sprite sheets). | Allows sprites and atlases to be resolved uniformly. |

## Command Model
1. Replace the ad-hoc `RenderBatch.staticGeometry` with explicit command objects:
   - `FillPolygon` (`vertices`, `fillColor`, optional stroke descriptor)
   - `SpriteQuad` (`spriteId`, `frame`, `destination`, `tint`)
   - `DebugLine` / `DebugText` for diagnostics
2. Ensure commands carry semantic units (world coordinates, layer ID) and leave rasterization to the renderer.
3. Serialize batches as plain data so they can be inspected or replayed in tooling.

## Renderer Surface Contract
1. Introduce a `RenderSurface` interface:
   ```ts
   interface RenderSurface {
     readonly configuration: RendererConfiguration;
     mount(host: RenderHost): void;
     resize(dimensions: RenderDimensions): void;
     submit(commands: RenderCommandBuffer): void;
     dispose(): void;
   }
   ```
2. Define `RenderHost` to wrap DOM specifics (`canvas`, `webglContext`, resize observer hooks) so only surface implementations depend on browser objects.
3. Update the orchestrator to depend solely on `RenderSurface`, enabling runtime selection (`CanvasSurface`, `WebGLSurface`, `HeadlessSurface`).

## Effect Runtime Abstraction
- Replace the raw context handle in `EffectFrameContext` with an `EffectRenderer` that exposes declarative draw helpers.
- Refactor existing effects to call helpers like `renderer.drawCircle(position, style)` instead of `ctx.arc`.
- Provide Canvas and WebGL implementations of `EffectRenderer`, each translating helpers into backend-specific draw calls or shader submissions.
- Keep timing, spawn/update/end semantics unchanged so the server contract is unaffected.

## Asset & Texture Management
- Replace DOM element references in decal specs with logical asset IDs resolved through `AssetRegistry`.
- Allow registries to load resources lazily per backend (e.g., `HTMLImageElement` for Canvas, `WebGLTexture` for WebGL, atlas lookups for sprite sheets).
- Store sprite sheet metadata (frames, frame size, pivot) in shared JSON that both server tooling and client registries can read.

## Migration Path
1. **Surface extraction:** Introduce `RenderSurface` and adapt current Canvas code into `CanvasSurface` without changing behaviour.
2. **Command buffer:** Refactor orchestration to emit typed commands; add translators to keep Canvas drawing working during the transition.
3. **Effect renderer:** Update `tools/js-effects` runtime and effect definitions to consume the new helper API; provide Canvas implementation first.
4. **Asset registry:** Swap decal references for asset IDs and implement lazy resource loading.
5. **Introduce new backends:** With abstractions in place, create `WebGLSurface` or sprite-based surfaces iteratively, reusing command buffer and effect helpers.

## Testing & Validation
- Preserve the existing visual baselines by snapshotting command buffers during regression runs and replaying them through multiple surfaces.
- Ensure `npm test` continues to validate render orchestration once the new interfaces land.
- Add backend-specific sanity checks (e.g., WebGL context loss handling, sprite atlas bounds) under `client/__tests__/` alongside the existing network and lifecycle suites.

## Contributor Checklist
1. Read the `RenderSurface` and command definitions before touching rendering code.
2. Extend `EffectRenderer` helpers instead of calling backend APIs directly.
3. Register new assets through `AssetRegistry`; do not pass DOM elements through render batches.
4. Update this spec if you add new command types or surface responsibilities.
