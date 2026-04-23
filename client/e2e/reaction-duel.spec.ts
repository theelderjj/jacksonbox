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

test("reaction duel launches from the picker and resolves a round", async ({ browser }) => {
  test.setTimeout(120_000);
  const roomCode = `RD${Date.now().toString().slice(-6)}`;
  const players = await Promise.all(["Alice", "Bob"].map((name) => makePlayer(browser, name)));

  try {
    await fs.mkdir(SHOTS_DIR, { recursive: true });

    await test.step("Players join a fresh room", async () => {
      for (const player of players) {
        await joinRoom(player, roomCode);
      }
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "reaction-duel-01-picker.png"),
        fullPage: true,
      });
    });

    await test.step("Leader selects Reaction Duel and tightens countdown", async () => {
      await players[0]!.page.getByRole("button", { name: /Choose Reaction Duel/i }).click();
      await expect(players[0]!.page.getByRole("heading", { name: /Reaction Duel Lobby/i })).toBeVisible();
      await players[0]!.page.locator("select").nth(1).selectOption("1");
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "reaction-duel-02-lobby.png"),
        fullPage: true,
      });
    });

    await test.step("Both players ready and leader starts", async () => {
      for (const player of players) {
        await player.page.getByRole("button", { name: /I'm ready/i }).click();
      }
      await expect(players[0]!.page.locator(".pill.ready")).toHaveCount(2, { timeout: 10_000 });
      await players[0]!.page.getByRole("button", { name: /Start game/i }).click();
    });

    await test.step("The duel turns green and both players tap", async () => {
      await expect(players[0]!.page.getByRole("heading", { name: /Wait for green|Countdown/i })).toBeVisible({
        timeout: 10_000,
      });
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "reaction-duel-03-countdown.png"),
        fullPage: true,
      });
      await Promise.all(
        players.map((player) =>
          expect(player.page.getByRole("heading", { name: /Tap now/i })).toBeVisible({ timeout: 20_000 })
        )
      );
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "reaction-duel-04-go.png"),
        fullPage: true,
      });
      await Promise.all(players.map((player) => player.page.getByRole("button", { name: "TAP" }).click()));
    });

    await test.step("Reveal results arrive", async () => {
      for (const player of players) {
        await expect(player.page.getByRole("heading", { name: /Reaction Duel Results/i })).toBeVisible({
          timeout: 10_000,
        });
      }
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "reaction-duel-05-results.png"),
        fullPage: true,
      });
    });
  } finally {
    for (const player of players) {
      await player.ctx.close().catch(() => {});
    }
  }
});
