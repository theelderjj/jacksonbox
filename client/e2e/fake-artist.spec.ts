import { expect, test, type Browser, type BrowserContext, type Page } from "@playwright/test";
import fs from "node:fs/promises";
import path from "node:path";

type Player = { name: string; ctx: BrowserContext; page: Page };
const SHOTS_DIR = path.resolve(process.cwd(), "../docs/demo-shots");
const PLAYER_NAMES = ["Alice", "Bob", "Cara", "Drew", "Eli", "Fran", "Gabe", "Hope", "Ivan", "Jules"] as const;

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
  await Promise.race([
    player.page.getByRole("heading", { name: /Jackson Box/i }).waitFor({ state: "visible", timeout: 15_000 }),
    player.page.getByRole("heading", { name: /Fake Artist Lobby/i }).waitFor({ state: "visible", timeout: 15_000 }),
    player.page.getByRole("heading", { name: /Fake Artist/i }).waitFor({ state: "visible", timeout: 15_000 }),
    player.page.getByRole("heading", { name: /Replay the drawing/i }).waitFor({ state: "visible", timeout: 15_000 }),
    player.page.getByRole("heading", { name: /Vote for the fake artist/i }).waitFor({ state: "visible", timeout: 15_000 }),
    player.page.getByRole("heading", { name: /Fake Artist Guess/i }).waitFor({ state: "visible", timeout: 15_000 }),
    player.page.getByRole("heading", { name: /Fake Artist Reveal/i }).waitFor({ state: "visible", timeout: 15_000 }),
  ]);
}

async function drawQuickMark(page: Page, offset = 0): Promise<void> {
  const canvas = page.locator("canvas").first();
  await expect(canvas).toBeVisible();
  const box = await canvas.boundingBox();
  if (!box) throw new Error("canvas not visible");
  const pathPoints: Array<{ x: number; y: number }> = [
    { x: box.x + box.width * (0.22 + offset), y: box.y + box.height * (0.22 + offset * 0.3) },
    { x: box.x + box.width * (0.38 + offset * 0.4), y: box.y + box.height * (0.35 + offset * 0.4) },
    { x: box.x + box.width * (0.54 + offset * 0.2), y: box.y + box.height * (0.52 + offset * 0.3) },
    { x: box.x + box.width * (0.72 - offset * 0.1), y: box.y + box.height * (0.68 + offset * 0.1) },
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
  await page.waitForTimeout(220);
  for (let i = 1; i < pathPoints.length; i++) {
    const point = pathPoints[i]!;
    await canvas.dispatchEvent("pointermove", {
      ...common,
      clientX: point.x,
      clientY: point.y,
    });
    await page.waitForTimeout(220);
  }
  await canvas.dispatchEvent("pointerup", {
    ...common,
    buttons: 0,
    clientX: pathPoints[pathPoints.length - 1]!.x,
    clientY: pathPoints[pathPoints.length - 1]!.y,
  });
}

async function clearExistingShots(): Promise<void> {
  const files = await fs.readdir(SHOTS_DIR);
  await Promise.all(
    files
      .filter((name) => name.startsWith("fake-artist-") && name.endsWith(".png"))
      .map((name) => fs.unlink(path.join(SHOTS_DIR, name))),
  );
}

async function waitForNextDrawingTurn(page: Page, previousDrawer: string): Promise<void> {
  await expect
    .poll(
      async () => {
        const statusText = ((await page.getByText(/draws next|is drawing now/i).first().textContent().catch(() => "")) ?? "").trim();
        const drawerName = ((await page.getByLabel("Current drawer").textContent().catch(() => "")) ?? "").trim();
        const replayVisible = await page.getByRole("heading", { name: /Replay the drawing/i }).isVisible().catch(() => false);
        if (replayVisible) return "replay";
        if (drawerName !== "" && drawerName !== previousDrawer && /draws next|is drawing now/i.test(statusText)) {
          return "next";
        }
        return "waiting";
      },
      { timeout: 30_000 },
    )
    .not.toBe("waiting");
}

test("fake artist launches from the picker and reaches reveal", async ({ browser }) => {
  test.setTimeout(540_000);
  const roomCode = `FA${Date.now().toString().slice(-6)}`;
  const players = await Promise.all(PLAYER_NAMES.map((name) => makePlayer(browser, name)));

  try {
    await fs.mkdir(SHOTS_DIR, { recursive: true });
    await clearExistingShots();

    await test.step("Leader picks Fake Artist, then the rest of the room joins", async () => {
      await joinRoom(players[0]!, roomCode);
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "fake-artist-01-picker.png"),
        fullPage: true,
      });
      await players[0]!.page.getByRole("button", { name: /Choose Fake Artist/i }).click();
      await expect(players[0]!.page.getByRole("heading", { name: /Fake Artist Lobby/i })).toBeVisible();
      for (const player of players.slice(1)) {
        await joinRoom(player, roomCode);
        await expect(player.page.getByRole("heading", { name: /Fake Artist Lobby/i })).toBeVisible({
          timeout: 15_000,
        });
      }
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "fake-artist-02-lobby.png"),
        fullPage: true,
      });
    });

    let fakePlayer: Player | null = null;
    await test.step("Host configures a short single round and starts", async () => {
      await players[0]!.page.getByLabel("Rounds").selectOption("1");
      await players[0]!.page.getByLabel("Draw time per player").selectOption("10");
      await players[0]!.page.getByLabel("Voting time").selectOption("15");
      const colorToggle = players[0]!.page
        .locator("label")
        .filter({ has: players[0]!.page.getByText("Color code each drawer") })
        .getByRole("button");
      if (((await colorToggle.textContent()) ?? "").trim() === "Off") {
        await colorToggle.click();
      }
      for (const player of players) {
        await player.page.getByRole("button", { name: /I'm ready/i }).click();
      }
      await expect(players[0]!.page.locator(".pill.ready")).toHaveCount(players.length, { timeout: 15_000 });
      await players[0]!.page.getByRole("button", { name: /Start game/i }).click();
      await expect(players[0]!.page.getByRole("heading", { name: /Fake Artist/i })).toBeVisible({
        timeout: 10_000,
      });
    });

    await test.step("Detect the fake artist from the private role prompt", async () => {
      for (const player of players) {
        await expect(player.page.getByText(/draws next|is drawing now/i)).toBeVisible({ timeout: 10_000 });
        const note = player.page.locator(".intro-note").first();
        const text = (await note.textContent()) ?? "";
        if (text.includes("fake artist")) {
          fakePlayer = player;
        }
      }
      expect(fakePlayer).not.toBeNull();
      await expect(fakePlayer!.page.locator(".intro-note")).not.toContainText("Draw this together:");
    });

    await test.step("Each player draws twice on the same canvas", async () => {
      const drawCounts = new Map<string, number>();
      const totalTurns = players.length * 2;
      const deadline = Date.now() + 330_000;
      let completedTurns = 0;
      while (completedTurns < totalTurns && Date.now() < deadline) {
        await expect(players[0]!.page.getByText(/is drawing now/i)).toBeVisible({ timeout: 20_000 });
        const drawerName = ((await players[0]!.page.getByLabel("Current drawer").textContent()) ?? "").trim();
        const turnsTaken = drawCounts.get(drawerName) ?? 0;
        if (turnsTaken < 2) {
          const drawer = players.find((player) => player.name === drawerName);
          if (!drawer) {
            throw new Error(`No player matched current drawer ${drawerName}`);
          }
          await drawQuickMark(drawer.page, completedTurns * 0.04);
          drawCounts.set(drawerName, turnsTaken + 1);
          completedTurns += 1;
          await players[0]!.page.screenshot({
            path: path.join(
              SHOTS_DIR,
              `fake-artist-03-turn-${String(completedTurns).padStart(2, "0")}-${drawerName.toLowerCase()}.png`,
            ),
            fullPage: true,
          });
        }
        if (completedTurns < totalTurns) {
          await waitForNextDrawingTurn(players[0]!.page, drawerName);
        }
      }
      expect(completedTurns).toBe(totalTurns);
    });

    await test.step("Replay begins and the room votes for the fake", async () => {
      for (const player of players) {
        await expect(player.page.getByRole("heading", { name: /Replay the drawing/i })).toBeVisible({
          timeout: 90_000,
        });
      }
      const totalTurns = players.length * 2;
      const leaderReplayLabel = players[0]!.page.getByLabel("Replay status");
      const capturedCountdowns = new Set<number>();
      const capturedDraws = new Set<number>();
      const replayDeadline = Date.now() + 90_000;
      while (
        (capturedCountdowns.size < totalTurns || capturedDraws.size < totalTurns) &&
        Date.now() < replayDeadline
      ) {
        const text = ((await leaderReplayLabel.textContent()) ?? "").trim();
        const countdownMatch = text.match(new RegExp(`Turn (\\d+) of ${totalTurns}:\\s*(.+) draws in (\\d)`));
        const drawMatch = text.match(new RegExp(`Turn (\\d+) of ${totalTurns}:\\s*(.+) drawing replays now`));
        if (!countdownMatch && !drawMatch) {
          await players[0]!.page.waitForTimeout(150);
          continue;
        }
        if (countdownMatch) {
          const turnNumber = Number(countdownMatch[1]);
          const drawerSlug = countdownMatch[2]!.trim().toLowerCase().replace(/[^a-z0-9]+/g, "-");
          const countdownValue = countdownMatch[3]!;
          if (!capturedCountdowns.has(turnNumber)) {
            await players[0]!.page.screenshot({
              path: path.join(
                SHOTS_DIR,
                `fake-artist-04-replay-${String(turnNumber).padStart(2, "0")}-countdown-${countdownValue}-${drawerSlug}.png`,
              ),
              fullPage: true,
            });
            capturedCountdowns.add(turnNumber);
          }
        } else if (drawMatch) {
          const turnNumber = Number(drawMatch[1]);
          const drawerSlug = drawMatch[2]!.trim().toLowerCase().replace(/[^a-z0-9]+/g, "-");
          if (!capturedDraws.has(turnNumber)) {
            await players[0]!.page.screenshot({
              path: path.join(
                SHOTS_DIR,
                `fake-artist-04-replay-${String(turnNumber).padStart(2, "0")}-draw-${drawerSlug}.png`,
              ),
              fullPage: true,
            });
            capturedDraws.add(turnNumber);
          }
        }
        await players[0]!.page.waitForTimeout(150);
      }
      expect(capturedCountdowns.size).toBe(totalTurns);
      expect(capturedDraws.size).toBe(totalTurns);
      for (const player of players) {
        await expect(player.page.getByRole("heading", { name: /Vote for the fake artist/i })).toBeVisible({
          timeout: 20_000,
        });
        if (player.name === "Alice") {
          await players[0]!.page.screenshot({
            path: path.join(SHOTS_DIR, "fake-artist-05-vote.png"),
            fullPage: true,
          });
        }
        await player.page.getByRole("button", { name: fakePlayer!.name }).click();
      }
    });

    await test.step("The fake artist gets the guess phase and the game reveals", async () => {
      await expect(fakePlayer!.page.getByRole("heading", { name: /Fake Artist Guess/i })).toBeVisible({
        timeout: 10_000,
      });
      await fakePlayer!.page.getByPlaceholder("Guess the drawing prompt").fill("banana");
      await fakePlayer!.page.screenshot({
        path: path.join(SHOTS_DIR, "fake-artist-06-guess.png"),
        fullPage: true,
      });
      await fakePlayer!.page.getByRole("button", { name: /Submit guess/i }).click();
      for (const player of players) {
        await expect(player.page.getByRole("heading", { name: /Fake Artist Reveal/i })).toBeVisible({
          timeout: 15_000,
        });
      }
      await players[0]!.page.screenshot({
        path: path.join(SHOTS_DIR, "fake-artist-07-reveal.png"),
        fullPage: true,
      });
    });
  } finally {
    for (const player of players) {
      await player.ctx.close().catch(() => {});
    }
  }
});
