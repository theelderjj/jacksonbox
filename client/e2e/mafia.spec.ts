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

async function readRole(page: Page): Promise<string> {
  const text = await page.locator("h3").filter({ hasText: "Your role:" }).first().textContent();
  return (text ?? "").replace("Your role:", "").trim();
}

test("mafia launches from the picker and the town can eliminate the mafia", async ({ browser }) => {
  test.setTimeout(240_000);
  const roomCode = `MF${Date.now().toString().slice(-6)}`;
  const players = await Promise.all(
    ["Alice", "Bob", "Cara", "Drew", "Eli", "Fran"].map((name) => makePlayer(browser, name)),
  );

  try {
    await fs.mkdir(SHOTS_DIR, { recursive: true });

    await test.step("Players join and the host chooses Mafia", async () => {
      for (const player of players) {
        await joinRoom(player, roomCode);
      }
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "mafia-01-picker.png"),
        fullPage: true,
      });
      await players[0]!.page.getByRole("button", { name: /Choose Mafia/i }).click();
      await expect(players[0]!.page.getByRole("heading", { name: /Mafia Lobby/i })).toBeVisible();
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "mafia-02-lobby.png"),
        fullPage: true,
      });
    });

    await test.step("Host tightens the timers and starts the game", async () => {
      await players[0]!.page.getByLabel("Max day cycles").selectOption("1");
      await players[0]!.page.getByLabel("Night action time").selectOption("10");
      await players[0]!.page.getByLabel("Day discussion time").selectOption("20");
      await players[0]!.page.getByLabel("Day vote time").selectOption("10");
      for (const player of players) {
        await player.page.getByRole("button", { name: /I'm ready/i }).click();
      }
      await expect(players[0]!.page.locator(".pill.ready")).toHaveCount(6, { timeout: 10_000 });
      await players[0]!.page.getByRole("button", { name: /Start game/i }).click();
    });

    const roles = new Map<string, string>();

    await test.step("Role reveal completes and each player learns their role", async () => {
      for (const player of players) {
        await expect(player.page.getByRole("heading", { name: /^Mafia$/i })).toBeVisible({ timeout: 15_000 });
        roles.set(player.name, await readRole(player.page));
      }
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "mafia-03-role.png"),
        fullPage: true,
      });
    });

    await test.step("Night actions resolve privately", async () => {
      await Promise.all(
        players.map((player) =>
          expect(player.page.getByRole("heading", { name: /Night falls/i })).toBeVisible({ timeout: 15_000 }),
        ),
      );
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "mafia-04-night.png"),
        fullPage: true,
      });

      const mafiaPlayer = players.find((player) => roles.get(player.name) === "Mafia");
      const doctorPlayer = players.find((player) => roles.get(player.name) === "Doctor");
      const detectivePlayer = players.find((player) => roles.get(player.name) === "Detective");
      const targetTown = players.find((player) => roles.get(player.name) === "Citizen");
      if (!mafiaPlayer || !doctorPlayer || !detectivePlayer || !targetTown) {
        throw new Error(`unexpected role layout: ${JSON.stringify(Object.fromEntries(roles))}`);
      }

      await mafiaPlayer.page.getByRole("button", { name: targetTown.name }).click();
      await doctorPlayer.page.getByRole("button", { name: targetTown.name }).click();
      await detectivePlayer.page.getByRole("button", { name: mafiaPlayer.name }).click();
    });

    let mafiaPlayerName = "";

    await test.step("The town discusses, nominates, and then votes the mafia out", async () => {
      for (const player of players) {
        await expect(player.page.getByRole("heading", { name: /Day discussion/i })).toBeVisible({ timeout: 20_000 });
      }
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "mafia-05-discussion.png"),
        fullPage: true,
      });

      const mafiaPlayer = players.find((player) => roles.get(player.name) === "Mafia");
      if (!mafiaPlayer) throw new Error("missing mafia player");
      mafiaPlayerName = mafiaPlayer.name;

      for (const player of players) {
        await expect(player.page.getByRole("heading", { name: /Nominations/i })).toBeVisible({ timeout: 60_000 });
      }
      for (const player of players) {
        if (player.name === mafiaPlayerName) {
          await player.page.getByRole("button", { name: "Alice" }).click();
        } else {
          await player.page.getByRole("button", { name: mafiaPlayerName }).click();
        }
      }

      for (const player of players) {
        await expect(player.page.getByRole("heading", { name: /Day vote/i })).toBeVisible({ timeout: 60_000 });
      }
      for (const player of players) {
        await player.page.getByRole("button", { name: mafiaPlayerName }).click();
      }
    });

    await test.step("Reveal and results show a town win", async () => {
      for (const player of players) {
        await expect(player.page.getByRole("heading", { name: /Vote reveal/i })).toBeVisible({ timeout: 10_000 });
      }
      await expect(players[0]!.page.getByText(/The town wins the game/i)).toBeVisible();
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "mafia-06-reveal.png"),
        fullPage: true,
      });

      for (const player of players) {
        await expect(player.page.getByText(/Final scores/i)).toBeVisible({ timeout: 15_000 });
      }
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "mafia-07-results.png"),
        fullPage: true,
      });
      await expect(players[0]!.page.getByText("1000 pts", { exact: true })).toHaveCount(5);
      await expect(players[0]!.page.getByText("0 pts", { exact: true })).toHaveCount(1);
    });
  } finally {
    for (const player of players) {
      await player.ctx.close().catch(() => {});
    }
  }
});
