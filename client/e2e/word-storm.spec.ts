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

test("word storm launches from the picker and scores valid unique words", async ({ browser }) => {
  test.setTimeout(140_000);
  const roomCode = `WS${Date.now().toString().slice(-6)}`;
  const players = await Promise.all(["Alice", "Bob"].map((name) => makePlayer(browser, name)));

  try {
    await fs.mkdir(SHOTS_DIR, { recursive: true });

    await test.step("Players join and choose Word Storm", async () => {
      for (const player of players) {
        await joinRoom(player, roomCode);
      }
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "word-storm-01-picker.png"),
        fullPage: true,
      });
      await players[0]!.page.getByRole("button", { name: /Choose Word Storm/i }).click();
      await expect(players[0]!.page.getByRole("heading", { name: /Word Storm Lobby/i })).toBeVisible();
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "word-storm-02-lobby.png"),
        fullPage: true,
      });
    });

    await test.step("Host configures one quick letter", async () => {
      await players[0]!.page.getByLabel("Rounds").selectOption("1");
      await players[0]!.page.getByLabel("Letter timer").selectOption("10");
      for (const player of players) {
        await player.page.getByRole("button", { name: /I'm ready/i }).click();
      }
      await expect(players[0]!.page.locator(".pill.ready")).toHaveCount(2, { timeout: 10_000 });
      await players[0]!.page.getByRole("button", { name: /Start game/i }).click();
    });

    await test.step("Players submit words and see duplicate feedback", async () => {
      for (const player of players) {
        await expect(player.page.getByRole("heading", { name: /Word Storm/i })).toBeVisible({
          timeout: 10_000,
        });
        await expect(player.page.getByText(/Scoring happens at the end/i)).toBeVisible();
      }
      const letter = (await players[0]!.page.locator(".storm-letter").innerText()).trim().toLowerCase();
      const [firstWord, secondWord] = wordsForLetter(letter);
      await submitWord(players[0]!.page, firstWord);
      await submitWord(players[1]!.page, firstWord);
      await expect(players[1]!.page.getByText(/Already entered/i)).toBeVisible();
      await submitWord(players[1]!.page, secondWord);
      await submitWord(players[0]!.page, `${letter}zzzzz`);
      await expect(players[0]!.page.getByText(/Tentative \+/i).first()).toBeVisible();
      await expect(players[0]!.page.getByText(/Official points arrive after dictionary cleanup/i)).toBeVisible();
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "word-storm-03-submit.png"),
        fullPage: true,
      });
    });

    await test.step("Reveal shows accepted words and final scores", async () => {
      for (const player of players) {
        await expect(player.page.getByRole("heading", { name: /Word Storm Results/i })).toBeVisible({
          timeout: 20_000,
        });
      }
      await expect(players[0]!.page.getByText(/Invalid words are crossed out/i)).toBeVisible();
      await expect(players[0]!.page.getByText(/Final score:/i).first()).toBeVisible();
      await expect(players[0]!.page.getByText(/Made-up words removed:/i).first()).toBeVisible();
      await expect(players[0]!.page.locator(".word-entry.crossed").first()).toBeVisible();
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "word-storm-04-reveal.png"),
        fullPage: true,
      });
      for (const player of players) {
        await expect(player.page.getByText(/Final scores/i)).toBeVisible({ timeout: 10_000 });
      }
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "word-storm-05-results.png"),
        fullPage: true,
      });
    });
  } finally {
    for (const player of players) {
      await player.ctx.close().catch(() => {});
    }
  }
});

async function submitWord(page: Page, word: string): Promise<void> {
  await page.getByPlaceholder(/Word starting with/i).fill(word);
  await page.getByRole("button", { name: "Enter" }).click();
}

function wordsForLetter(letter: string): [string, string] {
  const words: Record<string, [string, string]> = {
    a: ["apple", "anchor"],
    b: ["banana", "basket"],
    c: ["camel", "cabinet"],
    d: ["dragon", "diamond"],
    f: ["flower", "forest"],
    g: ["garden", "guitar"],
    h: ["hammer", "harbor"],
    l: ["lemon", "library"],
    m: ["mirror", "monkey"],
    p: ["pencil", "planet"],
    r: ["rabbit", "rocket"],
    s: ["shadow", "stream"],
    t: ["tiger", "turtle"],
    w: ["window", "wizard"],
  };
  return words[letter] ?? ["camel", "cabinet"];
}
