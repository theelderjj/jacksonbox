import { test, expect, type Browser, type BrowserContext, type Page } from "@playwright/test";

// E2E mirror of internal/room/integration_test.go::TestIntegration_FullJrawfulRound.
// Three browser contexts share the MAIN room, ready up, then loop through
// draw → fake → vote until the server declares the game over.
//
// BuildPhases forces a minimum of 3 rounds (see phases.go), so we loop
// rather than hard-coding a single pass. The loop bail-out is the
// appearance of "Final scores" on any player's screen.
//
// No mocks — real Go server, real WebSocket, real Vite-served bundle.

type Player = { name: string; ctx: BrowserContext; page: Page };

const PLAYERS = ["Alice", "Bob", "Carol"] as const;
const MAX_ROUNDS_WATCHDOG = 5;

async function makePlayer(browser: Browser, name: string): Promise<Player> {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  return { name, ctx, page };
}

async function joinMain(p: Player): Promise<void> {
  await p.page.goto("/");
  await p.page.getByPlaceholder("your name").fill(p.name);
  await p.page.locator("select").selectOption("MAIN");
  await p.page.getByRole("button", { name: "Join" }).click();
}

async function drawOneStroke(p: Player): Promise<void> {
  // One stroke clears Drawing.tsx's "draw something first" guard — the
  // server doesn't care about stroke shape, just non-empty payload.
  //
  // Canvas.tsx uses PointerEvents (onPointerDown/Move/Up) because phones
  // need touch. Playwright's `page.mouse.*` is documented to also fire
  // pointer events, but in practice the mouse→pointer synthesis is
  // unreliable inside React's synthetic-event layer on headless chromium.
  // Dispatching pointer events straight at the canvas element is the
  // canonical fix for canvas drawing tests and behaves the same as a real
  // touchscreen drag.
  const canvas = p.page.locator("canvas").first();
  await canvas.waitFor({ state: "visible" });
  const box = await canvas.boundingBox();
  if (!box) throw new Error(`canvas has no bounding box for ${p.name}`);
  const cx = box.x + box.width / 2;
  const cy = box.y + box.height / 2;

  const path: Array<{ x: number; y: number }> = [
    { x: cx - 40, y: cy },
    { x: cx - 20, y: cy - 5 },
    { x: cx, y: cy },
    { x: cx + 20, y: cy + 5 },
    { x: cx + 40, y: cy },
  ];

  const common = {
    bubbles: true,
    cancelable: true,
    composed: true,
    pointerId: 1,
    pointerType: "mouse",
    isPrimary: true,
    button: 0,
    buttons: 1,
  };

  await canvas.dispatchEvent("pointerdown", {
    ...common,
    clientX: path[0]!.x,
    clientY: path[0]!.y,
  });
  for (let i = 1; i < path.length; i++) {
    const pt = path[i]!;
    await canvas.dispatchEvent("pointermove", {
      ...common,
      clientX: pt.x,
      clientY: pt.y,
    });
  }
  await canvas.dispatchEvent("pointerup", {
    ...common,
    buttons: 0,
    clientX: path[path.length - 1]!.x,
    clientY: path[path.length - 1]!.y,
  });
}

// waitForDrawOrEnd races the next round's "Draw this" heading against
// "Final scores". Needed because the last round transitions through
// voting → reveal → scoring → game_end with non-zero wire latency:
// an instant isVisible() check right after a round returns often
// reports false even though game_end is about to arrive, pushing the
// outer loop into a bogus extra iteration. Racing both headings
// collapses "continue" and "end" into one wait.
async function waitForDrawOrEnd(p: Player, round: number): Promise<"draw" | "end"> {
  const drawP = p.page
    .getByRole("heading", { name: "Draw this" })
    .waitFor({ state: "visible", timeout: 30_000 })
    .then(() => "draw" as const);
  const endP = p.page
    .getByText(/Final scores/i)
    .waitFor({ state: "visible", timeout: 30_000 })
    .then(() => "end" as const);
  try {
    return await Promise.race([drawP, endP]);
  } catch (e) {
    // Diagnostic dump: what screen is this player actually stuck on?
    // Capturing h1/h2/h3, visible buttons, and any banner text gives us
    // enough to decide whether the hang is server-side (phase never
    // advanced) or client-side (phase advanced but UI didn't follow).
    const diag = await p.page
      .evaluate(() => {
        const headings = Array.from(document.querySelectorAll("h1,h2,h3"))
          .map((h) => (h.textContent ?? "").trim())
          .filter(Boolean);
        const buttons = Array.from(document.querySelectorAll("button"))
          .map((b) => ({
            text: (b.textContent ?? "").trim(),
            disabled: (b as HTMLButtonElement).disabled,
          }))
          .filter((x) => x.text);
        const banners = Array.from(document.querySelectorAll(".banner"))
          .map((b) => (b.textContent ?? "").trim())
          .filter(Boolean);
        const phaseSlug =
          document.body.innerText.match(/Round \d+[^\n]*/i)?.[0] ?? null;
        return { headings, buttons, banners, phaseSlug };
      })
      .catch(() => ({ headings: [], buttons: [], banners: [], phaseSlug: null }));
    throw new Error(
      `${p.name} round ${round}: neither "Draw this" nor "Final scores" appeared within 30s (${(e as Error).message})\n` +
        `  DIAG headings=${JSON.stringify(diag.headings)}\n` +
        `  DIAG buttons=${JSON.stringify(diag.buttons)}\n` +
        `  DIAG banners=${JSON.stringify(diag.banners)}\n` +
        `  DIAG phaseSlug=${JSON.stringify(diag.phaseSlug)}`
    );
  }
}

async function playOneRound(
  players: readonly Player[],
  round: number
): Promise<"played" | "ended"> {
  // --- Drawing (or early game_end) -------------------------------------
  // Race first player's screen between the new round's draw heading
  // and the terminal Final scores screen. If the game just ended, we
  // bail without waiting 30s for a heading that will never appear.
  const outcome = await waitForDrawOrEnd(players[0]!, round);
  if (outcome === "end") return "ended";
  for (const p of players.slice(1)) {
    await expect(
      p.page.getByRole("heading", { name: "Draw this" }),
      `${p.name} drawing screen, round ${round}`
    ).toBeVisible({ timeout: 30_000 });
  }
  for (const p of players) {
    await drawOneStroke(p);
    const submit = p.page.getByRole("button", { name: "Submit drawing" });
    // If this fails, myPrompt didn't arrive (see store.ts reset-boundary
    // bug history) — the button stays disabled on null prompt.
    await expect(submit, `${p.name} submit-drawing enabled, round ${round}`).toBeEnabled({
      timeout: 10_000,
    });
    await submit.click();
    // No stale "Draw something first" banner — if this fires, drawOneStroke
    // didn't land any pointer events and strokesRef stayed empty.
    await expect(
      p.page.getByText(/Draw something first/i),
      `${p.name} should not see empty-strokes banner, round ${round}`
    ).toHaveCount(0);
    // Assert the "Submit drawing" button goes away — either React flipped
    // its label to "Submitted — waiting" (same screen), or the phase has
    // already advanced (last submitter races the server; Drawing screen
    // unmounts entirely). Both outcomes prove the submit landed; checking
    // only for "Submitted" flaked for the last player.
    await expect(
      submit,
      `${p.name} "Submit drawing" button should disappear after click, round ${round}`
    ).toBeHidden({ timeout: 10_000 });
  }

  // --- Fake prompts ----------------------------------------------------
  for (const p of players) {
    await expect(
      p.page.getByRole("heading", { name: "Fake a prompt" }),
      `${p.name} fake screen, round ${round}`
    ).toBeVisible({ timeout: 30_000 });
  }
  for (const p of players) {
    const inputs = p.page.getByPlaceholder("What could this be?");
    const count = await inputs.count();
    for (let i = 0; i < count; i++) {
      // Seed text with round + player so we never collide with another
      // player's fake or with the (unknown) true prompt. The server's
      // §8 dedupe only merges identical strings, so unique strings stay
      // as distinct choices.
      await inputs.nth(i).fill(`${p.name.toLowerCase()}-r${round}-f${i}`);
    }
    // Clicking "Submit fake" replaces the button with "Submitted — waiting
    // on others", so always target the first remaining one.
    let remaining = await p.page.getByRole("button", { name: "Submit fake" }).count();
    while (remaining > 0) {
      await p.page.getByRole("button", { name: "Submit fake" }).first().click();
      remaining = await p.page.getByRole("button", { name: "Submit fake" }).count();
    }
  }

  // --- Voting ----------------------------------------------------------
  for (const p of players) {
    await expect(
      p.page.getByRole("heading", { name: "Vote" }),
      `${p.name} vote screen, round ${round}`
    ).toBeVisible({ timeout: 30_000 });
  }
  for (const p of players) {
    // Click the first choice button in each voting card. Drawings authored
    // by this player don't render in the ballot so we don't need to filter.
    //
    // IMPORTANT: scope to cards that contain a <canvas>. The leader's
    // LeaderControls panel also uses className="card" and has Pause /
    // Pencils-down buttons; without the canvas filter Alice (the first
    // joiner, hence party leader) clicks Pause before her votes, which
    // stops the voting phase timer on the server and — combined with her
    // loop then iterating off-by-one through the remaining cards — leaves
    // one drawing un-voted. Voting never finalizes, leaderboard never
    // starts, round 2 never arrives.
    const cards = p.page
      .locator(".card")
      .filter({ has: p.page.locator("canvas") })
      .filter({ has: p.page.locator("button") });
    const cardCount = await cards.count();
    const ownFakePrefix = `${p.name.toLowerCase()}-r${round}-`;
    for (let i = 0; i < cardCount; i++) {
      const buttons = cards.nth(i).locator("button:not([disabled])");
      const buttonCount = await buttons.count();
      let clicked = false;
      for (let j = 0; j < buttonCount; j++) {
        const btn = buttons.nth(j);
        const text = ((await btn.textContent()) ?? "").trim().toLowerCase();
        if (text.startsWith(ownFakePrefix)) continue;
        await btn.click().catch(() => {
          // Card button disappeared (phase advanced mid-click). OK.
        });
        clicked = true;
        break;
      }
      if (!clicked) {
        throw new Error(`${p.name} round ${round}: no valid vote choice found for ballot ${i}`);
      }
    }
  }

  // --- Leader-driven reveal / round advance ---------------------------
  const leader = players[0]!;
  await expect(
    leader.page.getByRole("heading", { name: new RegExp(`Round ${round}`, "i") }),
    `leaderboard round ${round}`
  ).toBeVisible({ timeout: 30_000 });
  for (;;) {
    const next = leader.page.getByRole("button", { name: /Next reveal|Start next round/i });
    await expect(next).toBeVisible({ timeout: 30_000 });
    const label = ((await next.textContent()) ?? "").trim().toLowerCase();
    await next.click();
    if (label.includes("start next round")) break;
  }

  return "played";
}

test.describe("full Jrawful game", () => {
  test("three players play through every round and reach game_end", async ({ browser }) => {
    test.setTimeout(5 * 60_000); // 5 minutes — plenty even if a phase times out

    const players = await Promise.all(PLAYERS.map((n) => makePlayer(browser, n)));

    try {
      // --- Lobby ---------------------------------------------------------
      for (const p of players) {
        await joinMain(p);
        await expect(p.page.getByRole("heading", { name: /Lobby/ })).toBeVisible({
          timeout: 15_000,
        });
      }
      for (const p of players) {
        await p.page.getByRole("button", { name: /I'm ready/ }).click();
      }

      // --- Round loop ----------------------------------------------------
      // BuildPhases forces a 3-round minimum; loop until playOneRound
      // reports the terminal screen or until the watchdog trips.
      // playOneRound internally races "Draw this" vs "Final scores" so we
      // don't need a separate pre-loop isGameOver poll (which was racy —
      // an instant check misses the transition frame between scoring_rN
      // completing and game_end being broadcast).
      for (let round = 1; round <= MAX_ROUNDS_WATCHDOG; round++) {
        const outcome = await playOneRound(players, round);
        if (outcome === "ended") break;
      }

      // --- Game end -----------------------------------------------------
      for (const p of players) {
        await expect(p.page.getByText(/Final scores/i), `${p.name} final screen`).toBeVisible({
          timeout: 60_000,
        });
      }
    } finally {
      for (const p of players) {
        await p.ctx.close();
      }
    }
  });
});
