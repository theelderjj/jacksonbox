import { test, expect, type Browser, type BrowserContext, type Page } from "@playwright/test";
import fs from "node:fs/promises";
import path from "node:path";

type Player = { name: string; ctx: BrowserContext; page: Page };

const PLAYERS = ["Alice", "Bob", "Carol"] as const;
const SHOTS_DIR = path.resolve(process.cwd(), "../docs/demo-shots");

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

  const pathPoints: Array<{ x: number; y: number }> = [
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
    clientX: pathPoints[0]!.x,
    clientY: pathPoints[0]!.y,
  });
  for (let i = 1; i < pathPoints.length; i++) {
    const pt = pathPoints[i]!;
    await canvas.dispatchEvent("pointermove", {
      ...common,
      clientX: pt.x,
      clientY: pt.y,
    });
  }
  await canvas.dispatchEvent("pointerup", {
    ...common,
    buttons: 0,
    clientX: pathPoints[pathPoints.length - 1]!.x,
    clientY: pathPoints[pathPoints.length - 1]!.y,
  });
}

test("capture real gameplay screenshots for README gif", async ({ browser }) => {
  test.setTimeout(10 * 60_000);
  await test.step("Prepare screenshot output folder", async () => {
    await fs.mkdir(SHOTS_DIR, { recursive: true });
    for (const file of await fs.readdir(SHOTS_DIR)) {
      if (file.endsWith(".png")) await fs.unlink(path.join(SHOTS_DIR, file));
    }
  });

  const players = await Promise.all(PLAYERS.map((n) => makePlayer(browser, n)));

  try {
    await test.step("Capture lobby", async () => {
      for (const p of players) {
        await joinMain(p);
        await expect(p.page.getByRole("heading", { name: /Lobby/ })).toBeVisible({ timeout: 15000 });
      }
      await players[0]!.page.screenshot({ path: path.join(SHOTS_DIR, "01-lobby.png"), fullPage: true });
    });

    await test.step("Ready players", async () => {
      for (const p of players) {
        await p.page.getByRole("button", { name: /I'm ready/ }).click();
      }
    });

    await test.step("Capture drawing phase", async () => {
      for (const p of players) {
        await expect(p.page.getByRole("heading", { name: "Draw this" })).toBeVisible({ timeout: 30000 });
      }
      await players[0]!.page.screenshot({ path: path.join(SHOTS_DIR, "02-draw-phase.png"), fullPage: true });
    });

    await test.step("Submit demo drawings", async () => {
      for (const p of players) {
        await drawOneStroke(p);
        const submit = p.page.getByRole("button", { name: "Submit drawing" });
        await expect(submit).toBeEnabled({ timeout: 10000 });
        await submit.click();
      }
    });

    await test.step("Capture and submit fake prompts", async () => {
      for (const p of players) {
        await expect(p.page.getByRole("heading", { name: "Fake a prompt" })).toBeVisible({ timeout: 30000 });
      }
      await players[0]!.page.screenshot({ path: path.join(SHOTS_DIR, "03-fake-phase.png"), fullPage: true });
      for (const p of players) {
        const inputs = p.page.getByPlaceholder("What could this be?");
        const count = await inputs.count();
        for (let i = 0; i < count; i++) {
          await inputs.nth(i).fill(`${p.name.toLowerCase()}-demo-fake-${i}`);
        }
        const fakeButtons = p.page.getByRole("button", { name: "Submit fake" });
        const fakeCount = await fakeButtons.count();
        for (let i = 0; i < fakeCount; i++) {
          await fakeButtons.first().click();
        }
      }
    });

    await test.step("Capture and submit votes", async () => {
      for (const p of players) {
        await expect(p.page.getByRole("heading", { name: "Vote" })).toBeVisible({ timeout: 30000 });
      }
      await players[0]!.page.screenshot({ path: path.join(SHOTS_DIR, "04-vote-phase.png"), fullPage: true });
      for (const p of players) {
        const cards = p.page
          .locator(".card")
          .filter({ has: p.page.locator("canvas") })
          .filter({ has: p.page.locator("button") });
        const cardCount = await cards.count();
        const ownFakePrefix = `${p.name.toLowerCase()}-demo-fake-`;
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
            throw new Error(`${p.name}: no valid vote choice found for ballot ${i}`);
          }
        }
      }
    });

    await test.step("Capture highlighted reveal", async () => {
      await expect(
        players[0]!.page.getByRole("heading", { name: /Round 1/i }),
        "leader reveal screen"
      ).toBeVisible({ timeout: 30000 });
      for (let i = 0; i < 6; i++) {
        if (await players[0]!.page.locator(".revealed-answer").isVisible().catch(() => false)) break;
        await players[0]!.page.getByRole("button", { name: "Next reveal" }).click();
      }
      await expect(
        players[0]!.page.getByLabel("Revealed answer").first(),
        "highlighted revealed answer"
      ).toBeVisible({ timeout: 10000 });
      await players[0]!.page.screenshot({ path: path.join(SHOTS_DIR, "05-reveal-phase.png"), fullPage: true });
    });
  } finally {
    for (const p of players) {
      await p.ctx.close().catch(() => {
        // The browser may already be closed if the test was interrupted.
      });
    }
  }
});
