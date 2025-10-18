export interface GeometryBatch {
  readonly staticGeometry: readonly GeometryEntry[];
}

export interface GeometryEntry {
  readonly id: string;
  readonly vertices: readonly [number, number][];
  readonly layer?: { readonly zIndex?: number };
  readonly style?: unknown;
}

export const findStaticGeometry = (batch: GeometryBatch | undefined, id: string) =>
  batch?.staticGeometry.find((entry) => entry.id === id) ?? null;

export const computeVertexCentroid = (vertices: readonly [number, number][]) => {
  if (vertices.length === 0) {
    return { x: 0, y: 0 };
  }
  let sumX = 0;
  let sumY = 0;
  for (const [x, y] of vertices) {
    sumX += x;
    sumY += y;
  }
  return { x: sumX / vertices.length, y: sumY / vertices.length };
};

export const sortGeometryByRenderOrder = (
  geometry: readonly GeometryEntry[],
): readonly string[] =>
  [...geometry]
    .map((entry, index) => ({ entry, index }))
    .sort((left, right) => {
      const leftZ =
        typeof left.entry.layer?.zIndex === "number" && Number.isFinite(left.entry.layer.zIndex)
          ? left.entry.layer.zIndex
          : 0;
      const rightZ =
        typeof right.entry.layer?.zIndex === "number" && Number.isFinite(right.entry.layer.zIndex)
          ? right.entry.layer.zIndex
          : 0;
      if (leftZ !== rightZ) {
        return leftZ - rightZ;
      }
      return left.index - right.index;
    })
    .map((wrapped) => wrapped.entry.id);
