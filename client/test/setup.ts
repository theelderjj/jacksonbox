// Global test setup. Runs once per worker before any test file.
// - jest-dom adds matchers like toBeInTheDocument() on expect(...).
// - crypto.randomUUID is polyfilled for jsdom (older builds lack it).

import "@testing-library/jest-dom/vitest";

if (typeof globalThis.crypto === "undefined" || !globalThis.crypto.randomUUID) {
  // Minimal RFC4122-ish v4 stand-in; good enough for envelope IDs in tests.
  const randomUUID = (): `${string}-${string}-${string}-${string}-${string}` => {
    const hex = (n: number): string => Math.floor(Math.random() * n).toString(16);
    const seg = (len: number): string => Array.from({ length: len }, () => hex(16)).join("");
    return `${seg(8)}-${seg(4)}-${seg(4)}-${seg(4)}-${seg(12)}` as `${string}-${string}-${string}-${string}-${string}`;
  };
  Object.defineProperty(globalThis, "crypto", {
    value: { ...(globalThis.crypto ?? {}), randomUUID },
    configurable: true,
  });
}
