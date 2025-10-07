/**
 * Example vendor helper. Replace this file with a real library once needed.
 */
export function createVendorBanner(libraryName) {
  const target = libraryName || "external module";
  return `Loaded vendor helper: ${target}`;
}
