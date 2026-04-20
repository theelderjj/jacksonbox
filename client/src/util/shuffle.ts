// Deterministic shuffle seeded on an arbitrary string. Same input + seed
// always produces the same permutation — used for per-drawing voting
// ballots so a player sees a stable order across re-renders, yet TRUE
// isn't always in the same slot across drawings.
//
// Algorithm: Fisher-Yates driven by a cheap LCG whose initial state is a
// 32-bit hash of the seed string. Not cryptographic; determinism +
// low-cost + "looks random" is all we need.

export function shuffleStable<T>(arr: readonly T[], seedStr: string): T[] {
  const out = arr.slice();
  let seed = 0;
  for (let i = 0; i < seedStr.length; i++) {
    seed = (seed * 31 + seedStr.charCodeAt(i)) >>> 0;
  }
  for (let i = out.length - 1; i > 0; i--) {
    seed = (seed * 1664525 + 1013904223) >>> 0;
    const j = seed % (i + 1);
    const tmp = out[i];
    out[i] = out[j];
    out[j] = tmp;
  }
  return out;
}
