import { expect, test, type Browser, type BrowserContext, type Page } from "@playwright/test";
import fs from "node:fs/promises";
import path from "node:path";

type Player = { name: string; ctx: BrowserContext; page: Page };
const SHOTS_DIR = path.resolve(process.cwd(), "../docs/demo-shots");

async function makePlayer(browser: Browser, name: string): Promise<Player> {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  return { name, ctx, page };
}

async function joinRoom(player: Player, roomCode: string): Promise<void> {
  await player.page.goto("/");
  await player.page.getByPlaceholder("room code").fill(roomCode);
  await player.page.getByPlaceholder("your name").fill(player.name);
  await player.page.getByRole("button", { name: "Join" }).click();
  await expect(player.page.getByRole("heading", { name: /Jackson Box/i })).toBeVisible();
}

async function waitForProductImage(page: Page): Promise<void> {
  const image = page.locator("img").first();
  await expect(image).toBeVisible({ timeout: 15_000 });
  await expect
    .poll(
      async () =>
        image.evaluate((img) => {
          const el = img as HTMLImageElement;
          return el.complete && el.naturalWidth > 0 && el.naturalHeight > 0;
        }),
      { timeout: 30_000 },
    )
    .toBe(true);
}

test("price is right launches from the picker and scores the closest legal guess", async ({ browser }) => {
  test.setTimeout(180_000);
  const roomCode = `PR${Date.now().toString().slice(-6)}`;
  const players = await Promise.all(["Alice", "Bob", "Cara"].map((name) => makePlayer(browser, name)));

  try {
    await fs.mkdir(SHOTS_DIR, { recursive: true });

    await test.step("Players join and choose Price is Right", async () => {
      for (const player of players) {
        await joinRoom(player, roomCode);
      }
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "price-is-right-01-picker.png"),
        fullPage: true,
      });
      await players[0]!.page.getByRole("button", { name: /Choose Price is Right/i }).click();
      await expect(players[0]!.page.getByRole("heading", { name: /Price is Right Lobby/i })).toBeVisible();
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "price-is-right-02-lobby.png"),
        fullPage: true,
      });
    });

    await test.step("Host configures one round and fixed threshold", async () => {
      await players[0]!.page.getByLabel("Rounds").selectOption("1");
      await players[0]!.page.getByLabel("Guess time").selectOption("10");
      await players[0]!.page.getByLabel("Minimum item price").selectOption("5");
      await players[0]!.page.getByLabel("Threshold mode").selectOption("fixed");
      for (const player of players) {
        await player.page.getByRole("button", { name: /I'm ready/i }).click();
      }
      await expect(players[0]!.page.locator(".pill.ready")).toHaveCount(3, { timeout: 10_000 });
      await players[0]!.page.getByRole("button", { name: /Start game/i }).click();
    });

    await test.step("Each player submits a guess", async () => {
      for (const player of players) {
        await expect(player.page.getByRole("heading", { name: /Price is Right/i })).toBeVisible({ timeout: 10_000 });
      }
      await waitForProductImage(players[0]!.page);
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "price-is-right-03-guess.png"),
        fullPage: true,
      });
      await players[0]!.page.getByPlaceholder("Enter your guess").fill("1.00");
      await players[0]!.page.getByRole("button", { name: /Lock guess/i }).click();
      await players[1]!.page.getByPlaceholder("Enter your guess").fill("2.00");
      await players[1]!.page.getByRole("button", { name: /Lock guess/i }).click();
      await players[2]!.page.getByPlaceholder("Enter your guess").fill("3.00");
      await players[2]!.page.getByRole("button", { name: /Lock guess/i }).click();
    });

    await test.step("Reveal shows the actual price and winner", async () => {
      for (const player of players) {
        await expect(player.page.getByRole("heading", { name: /Price Reveal/i })).toBeVisible({ timeout: 10_000 });
      }
      await expect(players[0]!.page.getByText(/Winner: Cara/i)).toBeVisible();
      await waitForProductImage(players[0]!.page);
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "price-is-right-04-reveal.png"),
        fullPage: true,
      });
      for (const player of players) {
        await expect(player.page.getByText(/Final scores/i)).toBeVisible({ timeout: 10_000 });
      }
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "price-is-right-05-results.png"),
        fullPage: true,
      });
      await expect(players[0]!.page.getByText("1000 pts", { exact: true })).toHaveCount(1);
      await expect(players[0]!.page.getByText("0 pts", { exact: true })).toHaveCount(2);
    });
  } finally {
    for (const player of players) {
      await player.ctx.close().catch(() => {});
    }
  }
});
