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

test("split the vote launches from the picker and reveals the achieved target", async ({ browser }) => {
  test.setTimeout(120_000);
  const roomCode = `SV${Date.now().toString().slice(-6)}`;
  const players = await Promise.all(["Alice", "Bob"].map((name) => makePlayer(browser, name)));

  try {
    await fs.mkdir(SHOTS_DIR, { recursive: true });

    await test.step("Players join and choose Split the Vote", async () => {
      for (const player of players) {
        await joinRoom(player, roomCode);
      }
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "split-vote-01-picker.png"),
        fullPage: true,
      });
      await players[0]!.page.getByRole("button", { name: /Choose Split the Vote/i }).click();
      await expect(players[0]!.page.getByRole("heading", { name: /Split the Vote Lobby/i })).toBeVisible();
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "split-vote-02-lobby.png"),
        fullPage: true,
      });
    });

    await test.step("Players ready and leader starts", async () => {
      for (const player of players) {
        await player.page.getByRole("button", { name: /I'm ready/i }).click();
      }
      await expect(players[0]!.page.locator(".pill.ready")).toHaveCount(2, { timeout: 10_000 });
      await players[0]!.page.getByRole("button", { name: /Start game/i }).click();
    });

    await test.step("Splitter submits the prompt and options", async () => {
      await expect(players[0]!.page.getByRole("heading", { name: /Split the Vote Setup/i })).toBeVisible();
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "split-vote-03-setup.png"),
        fullPage: true,
      });
      await players[0]!.page.getByPlaceholder("Ask the room something").fill("Which side are you on?");
      await players[0]!.page.getByPlaceholder("First choice").fill("Left");
      await players[0]!.page.getByPlaceholder("Second choice").fill("Right");
      await players[0]!.page.getByRole("button", { name: /Start vote/i }).click();
    });

    await test.step("The room votes into a perfect split", async () => {
      for (const player of players) {
        await expect(player.page.getByRole("heading", { name: /Split the Vote/i })).toBeVisible({
          timeout: 10_000,
        });
      }
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "split-vote-04-vote.png"),
        fullPage: true,
      });
      await expect(players[0]!.page.getByRole("button", { name: "Left" })).toBeDisabled();
      await expect(players[0]!.page.getByRole("button", { name: "Right" })).toBeDisabled();
      await players[1]!.page.getByRole("button", { name: "Right" }).click();
    });

    await test.step("The round resolves with splitter-only setup and a scoring hit", async () => {
      await expect(players[0]!.page.getByRole("heading", { name: /Split the Vote Reveal/i })).toBeVisible({
        timeout: 10_000,
      });
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "split-vote-05-reveal.png"),
        fullPage: true,
      });
      for (const player of players) {
        await expect(player.page.getByText(/Final scores/i)).toBeVisible({ timeout: 10_000 });
      }
      await expect(players[0]!.page.getByText("1500 pts", { exact: true })).toBeVisible();
      await expect(players[1]!.page.getByText("500 pts", { exact: true })).toBeVisible();
    });
  } finally {
    for (const player of players) {
      await player.ctx.close().catch(() => {});
    }
  }
});
