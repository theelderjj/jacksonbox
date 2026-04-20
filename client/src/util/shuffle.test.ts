import { describe, expect, it } from "vitest";
import { shuffleStable } from "./shuffle";

describe("shuffleStable", () => {
  it("is deterministic: same seed + input → same output across calls", () => {
    // Scenario: we re-render Voting.tsx five times for drawing `d1`. Each
    // render re-invokes shuffleStable with the same seed.
    // Expected: every call returns byte-identical output — otherwise the
    // user would see the ballot reshuffle mid-vote, which is janky.
    const input = ["a", "b", "c", "d", "e", "f"];
    const first = shuffleStable(input, "d1");
    for (let i = 0; i < 4; i++) {
      expect(shuffleStable(input, "d1")).toEqual(first);
    }
  });

  it("preserves membership (is a permutation, not a filter)", () => {
    // Scenario: shuffle a 10-element list. Sort both input and output.
    // Expected: identical sorted contents — nothing dropped, nothing added.
    const input = ["TRUE", "cat", "dog", "fish", "bird", "rock", "tree", "sun", "moon", "star"];
    const out = shuffleStable(input, "seed-xyz");
    expect(out).toHaveLength(input.length);
    expect([...out].sort()).toEqual([...input].sort());
  });

  it("does not mutate the input array", () => {
    // Scenario: caller passes an array and re-reads it after the call.
    // Expected: input is unchanged — shuffleStable must work on a copy.
    const input = ["a", "b", "c", "d"];
    const snapshot = [...input];
    shuffleStable(input, "seed");
    expect(input).toEqual(snapshot);
  });

  it("different seeds usually produce different orders", () => {
    // Scenario: shuffle the same 8-item input with 20 different seeds.
    // Expected: the majority of results differ from each other — if they
    // all matched, the hash→LCG pipeline would be effectively broken.
    // We allow a handful of coincidental matches (pigeonhole) but the
    // distinct count must be clearly larger than 1.
    const input = ["a", "b", "c", "d", "e", "f", "g", "h"];
    const results = new Set<string>();
    for (let i = 0; i < 20; i++) {
      results.add(shuffleStable(input, `seed-${i}`).join(","));
    }
    expect(results.size).toBeGreaterThan(10);
  });

  it("handles empty + single-element inputs as identity", () => {
    // Scenario: degenerate cases (0 or 1 element). Fisher-Yates with n≤1
    // has no iterations; the output must equal the input.
    expect(shuffleStable<string>([], "x")).toEqual([]);
    expect(shuffleStable(["only"], "x")).toEqual(["only"]);
  });

  it("empty seed still produces a valid permutation", () => {
    // Scenario: empty seed string — hash accumulator stays 0, LCG still
    // advances. We care that the output is a well-formed permutation,
    // not about its specific shape.
    const input = [1, 2, 3, 4, 5];
    const out = shuffleStable(input, "");
    expect(out).toHaveLength(5);
    expect([...out].sort()).toEqual([1, 2, 3, 4, 5]);
  });
});
