import { test, expect, type Browser, type BrowserContext, type Page } from "@playwright/test";

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
  await p.page.getByRole("button", { name: "Join" }).click();
}

async function drawOneStroke(p: Player): Promise<void> {
  const canvas = p.page.locator("canvas").first();
  await canvas.waitFor({ state: "visible" });
  const box = await canvas.boundingBox();
  if (!box) throw new Error(`canvas has no bounding box for ${p.name}`);

  const cx = box.x + box.width / 2;
  const cy = box.y + box.height / 2;
  const points: Array<{ x: number; y: number }> = [
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

  await canvas.dispatchEvent("pointerdown", { ...common, clientX: points[0]!.x, clientY: points[0]!.y });
  for (const pt of points.slice(1)) {
    await canvas.dispatchEvent("pointermove", { ...common, clientX: pt.x, clientY: pt.y });
  }
  await canvas.dispatchEvent("pointerup", {
    ...common,
    buttons: 0,
    clientX: points[points.length - 1]!.x,
    clientY: points[points.length - 1]!.y,
  });
}

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
    const diag = await p.page
      .evaluate(() => {
        const headings = Array.from(document.querySelectorAll("h1,h2,h3"))
          .map((h) => (h.textContent ?? "").trim())
          .filter(Boolean);
        const buttons = Array.from(document.querySelectorAll("button"))
          .map((b) => ({ text: (b.textContent ?? "").trim(), disabled: (b as HTMLButtonElement).disabled }))
          .filter((x) => x.text);
        const banners = Array.from(document.querySelectorAll(".banner"))
          .map((b) => (b.textContent ?? "").trim())
          .filter(Boolean);
        const phaseSlug = document.body.innerText.match(/Round \d+[^\n]*/i)?.[0] ?? null;
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

async function submitDrawing(p: Player, round: number): Promise<void> {
  await drawOneStroke(p);
  const submit = p.page.getByRole("button", { name: "Submit drawing" });
  await expect(submit, `${p.name} submit-drawing enabled, round ${round}`).toBeEnabled({
    timeout: 10_000,
  });
  await submit.click();
  await expect(
    p.page.getByText(/Draw something first/i),
    `${p.name} should not see empty-strokes banner, round ${round}`
  ).toHaveCount(0);
  await expect(
    submit,
    `${p.name} "Submit drawing" button should disappear after click, round ${round}`
  ).toBeHidden({ timeout: 10_000 });
}

async function submitFakes(p: Player, round: number): Promise<void> {
  const inputs = p.page.getByPlaceholder("What could this be?");
  const count = await inputs.count();
  for (let i = 0; i < count; i++) {
    await inputs.nth(i).fill(`${p.name.toLowerCase()}-r${round}-f${i}`);
  }

  let remaining = await p.page.getByRole("button", { name: "Submit fake" }).count();
  while (remaining > 0) {
    await p.page.getByRole("button", { name: "Submit fake" }).first().click();
    remaining = await p.page.getByRole("button", { name: "Submit fake" }).count();
  }
}

async function castVotes(p: Player, round: number): Promise<void> {
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
        // The phase can advance between counting and clicking.
      });
      clicked = true;
      break;
    }
    if (!clicked) {
      throw new Error(`${p.name} round ${round}: no valid vote choice found for ballot ${i}`);
    }
  }
}

async function playOneRound(players: readonly Player[], round: number): Promise<"played" | "ended"> {
  const outcome = await test.step(`Round ${round}: wait for drawing phase`, async () => {
    return waitForDrawOrEnd(players[0]!, round);
  });
  if (outcome === "end") return "ended";

  await test.step(`Round ${round}: everyone draws`, async () => {
    for (const p of players.slice(1)) {
      await expect(
        p.page.getByRole("heading", { name: "Draw this" }),
        `${p.name} drawing screen, round ${round}`
      ).toBeVisible({ timeout: 30_000 });
    }
    for (const p of players) {
      await test.step(`${p.name}: submit drawing`, async () => submitDrawing(p, round));
    }
  });

  await test.step(`Round ${round}: everyone writes fake prompts`, async () => {
    for (const p of players) {
      await expect(
        p.page.getByRole("heading", { name: "Fake a prompt" }),
        `${p.name} fake screen, round ${round}`
      ).toBeVisible({ timeout: 30_000 });
    }
    for (const p of players) {
      await test.step(`${p.name}: submit fake prompts`, async () => submitFakes(p, round));
    }
  });

  await test.step(`Round ${round}: everyone votes`, async () => {
    for (const p of players) {
      await expect(
        p.page.getByRole("heading", { name: "Vote" }),
        `${p.name} vote screen, round ${round}`
      ).toBeVisible({ timeout: 30_000 });
    }
    for (const p of players) {
      await test.step(`${p.name}: cast votes`, async () => castVotes(p, round));
    }
  });

  await test.step(`Round ${round}: leader reveals results`, async () => {
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
  });

  return "played";
}

test.describe("full Jrawful game", () => {
  test("three players play through every round and reach game_end", async ({ browser }) => {
    test.setTimeout(5 * 60_000);
    const players = await Promise.all(PLAYERS.map((name) => makePlayer(browser, name)));

    try {
      await test.step("Lobby: players join and ready up", async () => {
        for (const p of players) {
          await test.step(`${p.name}: join MAIN room`, async () => {
            await joinMain(p);
            await expect(p.page.getByRole("heading", { name: /Lobby/ })).toBeVisible({
              timeout: 15_000,
            });
          });
        }
        for (const p of players) {
          await test.step(`${p.name}: ready`, async () => {
            await p.page.getByRole("button", { name: /I'm ready/ }).click();
          });
        }
      });

      await test.step("Game: play rounds until final scores", async () => {
        for (let round = 1; round <= MAX_ROUNDS_WATCHDOG; round++) {
          const outcome = await playOneRound(players, round);
          if (outcome === "ended") break;
        }
      });

      await test.step("Game end: every player sees final scores", async () => {
        for (const p of players) {
          await expect(p.page.getByText(/Final scores/i), `${p.name} final screen`).toBeVisible({
            timeout: 60_000,
          });
        }
      });
    } finally {
      for (const p of players) {
        await p.ctx.close();
      }
    }
  });
});
