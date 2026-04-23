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

async function drawQuickMark(page: Page, offset = 0): Promise<void> {
  const canvas = page.locator("canvas").first();
  await expect(canvas).toBeVisible();
  const box = await canvas.boundingBox();
  if (!box) throw new Error("canvas not visible");
  await page.mouse.move(box.x + box.width * (0.25 + offset), box.y + box.height * 0.25);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width * (0.7 - offset * 0.2), box.y + box.height * (0.7 - offset * 0.1));
  await page.mouse.up();
}

test("draw duel launches from the picker and resolves a judged round", async ({ browser }) => {
  test.setTimeout(180_000);
  const roomCode = `DD${Date.now().toString().slice(-6)}`;
  const players = await Promise.all(["Alice", "Bob", "Cara"].map((name) => makePlayer(browser, name)));

  try {
    await test.step("Players join and choose Draw Duel", async () => {
      await fs.mkdir(SHOTS_DIR, { recursive: true });
      for (const player of players) {
        await joinRoom(player, roomCode);
      }
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "draw-duel-01-picker.png"),
        fullPage: true,
      });
      await players[0]!.page.getByRole("button", { name: /Choose Draw Duel/i }).click();
      await expect(players[0]!.page.getByRole("heading", { name: /Draw Duel Lobby/i })).toBeVisible();
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "draw-duel-02-lobby.png"),
        fullPage: true,
      });
    });

    await test.step("Host configures a short single round and starts", async () => {
      await players[0]!.page.getByLabel("Rounds").selectOption("1");
      await players[0]!.page.getByLabel("Draw time").selectOption("20");
      await players[0]!.page.getByLabel("Judging time").selectOption("10");
      for (const player of players) {
        await player.page.getByRole("button", { name: /I'm ready/i }).click();
      }
      await expect(players[0]!.page.locator(".pill.ready")).toHaveCount(3, { timeout: 10_000 });
      await players[0]!.page.getByRole("button", { name: /Start game/i }).click();
    });

    await test.step("Both artists draw while the judge waits", async () => {
      for (const player of players) {
        await expect(player.page.getByRole("heading", { name: /Draw Duel/i })).toBeVisible({ timeout: 10_000 });
      }
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "draw-duel-03-draw.png"),
        fullPage: true,
      });
      await drawQuickMark(players[0]!.page, 0);
      await players[0]!.page.getByRole("button", { name: /Submit drawing/i }).click();
      await drawQuickMark(players[1]!.page, 0.08);
      await players[1]!.page.getByRole("button", { name: /Submit drawing/i }).click();
      await expect(players[2]!.page.getByRole("heading", { name: /Judge the duel/i })).toBeVisible({
        timeout: 10_000,
      });
    });

    await test.step("Judge picks a winner and reveal resolves", async () => {
      for (const player of players) {
        await expect(player.page.getByRole("heading", { name: /Judge the duel/i })).toBeVisible({ timeout: 10_000 });
      }
      await players[2]!.page.screenshot({
        path: path.join(SHOTS_DIR, "draw-duel-04-vote.png"),
        fullPage: true,
      });
      await players[2]!.page.getByRole("button", { name: /Vote for Alice/i }).click();
      for (const player of players) {
        await expect(player.page.getByRole("heading", { name: /Draw Duel Reveal/i })).toBeVisible({ timeout: 10_000 });
      }
      await expect(players[0]!.page.getByText(/Alice wins the duel/i)).toBeVisible();
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "draw-duel-05-reveal.png"),
        fullPage: true,
      });
      for (const player of players) {
        await expect(player.page.getByText(/Final scores/i)).toBeVisible({ timeout: 10_000 });
      }
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "draw-duel-06-results.png"),
        fullPage: true,
      });
      await expect(players[0]!.page.getByText("1000 pts", { exact: true })).toBeVisible();
      await expect(players[2]!.page.getByText("500 pts", { exact: true })).toBeVisible();
    });
  } finally {
    for (const player of players) {
      await player.ctx.close().catch(() => {});
    }
  }
});
