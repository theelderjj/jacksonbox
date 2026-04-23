import { expect, test, type Browser, type BrowserContext, type Page } from "@playwright/test";
import fs from "node:fs/promises";
import path from "node:path";

type Player = { name: string; ctx: BrowserContext; page: Page };
type RoleName = "citizen" | "mafia" | "detective" | "doctor" | "mayor";

const SHOTS_DIR = path.resolve(process.cwd(), "../docs/demo-shots");
const PLAYER_NAMES = ["Alice", "Bob", "Cara", "Drew", "Eli", "Fran", "Gabe", "Hope"] as const;
const ROLES: RoleName[] = ["citizen", "mafia", "detective", "doctor", "mayor"];

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

async function readRole(page: Page): Promise<RoleName> {
  const text = await page.locator("h3").filter({ hasText: "Your role:" }).first().textContent();
  const normalized = (text ?? "").replace("Your role:", "").trim().toLowerCase();
  if (normalized === "mafia") return "mafia";
  if (normalized === "detective") return "detective";
  if (normalized === "doctor") return "doctor";
  if (normalized === "mayor") return "mayor";
  return "citizen";
}

async function clearRoleShots(): Promise<void> {
  const files = await fs.readdir(SHOTS_DIR);
  await Promise.all(
    files
      .filter((name) => ROLES.some((role) => name.startsWith(`mafia-${role}-`)) && name.endsWith(".png"))
      .map((name) => fs.unlink(path.join(SHOTS_DIR, name))),
  );
}

async function captureRoleFrames(
  rolePlayers: Map<RoleName, Player>,
  step: string,
  fullPage = true,
): Promise<void> {
  await Promise.all(
    ROLES.map(async (role) => {
      const player = rolePlayers.get(role);
      if (!player) return;
      await player.page.screenshot({
        path: path.join(SHOTS_DIR, `mafia-${role}-${step}.png`),
        fullPage,
      });
    }),
  );
}

function playersByRole(players: Player[], rolesByName: Map<string, RoleName>): Map<RoleName, Player> {
  const out = new Map<RoleName, Player>();
  for (const player of players) {
    const role = rolesByName.get(player.name);
    if (role && !out.has(role)) out.set(role, player);
  }
  return out;
}

function playersWithRole(players: Player[], rolesByName: Map<string, RoleName>, role: RoleName): Player[] {
  return players.filter((player) => rolesByName.get(player.name) === role);
}

async function clickIfPresent(page: Page, name: string): Promise<boolean> {
  const button = page.getByRole("button", { name, exact: true }).first();
  if ((await button.count()) === 0) return false;
  await button.click();
  return true;
}

test("capture Mafia role-perspective screenshots for gameplay GIFs", async ({ browser }) => {
  test.setTimeout(360_000);
  const roomCode = `MR${Date.now().toString().slice(-6)}`;
  const players = await Promise.all(PLAYER_NAMES.map((name) => makePlayer(browser, name)));

  try {
    await fs.mkdir(SHOTS_DIR, { recursive: true });
    await clearRoleShots();

    await test.step("Start an 8-player Mafia room so every role is represented", async () => {
      for (const player of players) {
        await joinRoom(player, roomCode);
      }
      await players[0]!.page.getByRole("button", { name: /Choose Mafia/i }).click();
      await expect(players[0]!.page.getByRole("heading", { name: /Mafia Lobby/i })).toBeVisible();
      await players[0]!.page.getByLabel("Max day cycles").selectOption("1");
      await players[0]!.page.getByLabel("Night action time").selectOption("10");
      await players[0]!.page.getByLabel("Day discussion time").selectOption("20");
      await players[0]!.page.getByLabel("Day vote time").selectOption("10");

      for (const player of players) {
        await player.page.getByRole("button", { name: /I'm ready/i }).click();
      }
      await expect(players[0]!.page.locator(".pill.ready")).toHaveCount(players.length, { timeout: 15_000 });
      await players[0]!.page.getByRole("button", { name: /Start game/i }).click();
    });

    const rolesByName = new Map<string, RoleName>();
    let rolePlayers = new Map<RoleName, Player>();

    await test.step("Capture private role cards and role instructions", async () => {
      for (const player of players) {
        await expect(player.page.getByRole("heading", { name: /^Mafia$/i })).toBeVisible({ timeout: 15_000 });
        await expect(player.page.getByLabel("Role instructions")).toBeVisible();
        rolesByName.set(player.name, await readRole(player.page));
      }
      rolePlayers = playersByRole(players, rolesByName);
      for (const role of ROLES) {
        expect(rolePlayers.get(role), `missing role ${role}`).toBeTruthy();
      }
      await captureRoleFrames(rolePlayers, "01-role");
    });

    await test.step("Capture each role's night controls and submit night actions", async () => {
      for (const player of players) {
        await expect(player.page.getByRole("heading", { name: /Night falls/i })).toBeVisible({ timeout: 15_000 });
      }
      await captureRoleFrames(rolePlayers, "02-night");

      const mafiaPlayers = playersWithRole(players, rolesByName, "mafia");
      const detectivePlayer = rolePlayers.get("detective");
      const doctorPlayer = rolePlayers.get("doctor");
      const citizenPlayer = rolePlayers.get("citizen");
      if (!detectivePlayer || !doctorPlayer || !citizenPlayer || mafiaPlayers.length === 0) {
        throw new Error(`unexpected role layout: ${JSON.stringify(Object.fromEntries(rolesByName))}`);
      }

      for (const mafia of mafiaPlayers) {
        await mafia.page.getByRole("button", { name: citizenPlayer.name }).click();
      }
      await doctorPlayer.page.getByRole("button", { name: citizenPlayer.name }).click();
      await detectivePlayer.page.getByRole("button", { name: mafiaPlayers[0]!.name }).click();
    });

    await test.step("Capture dawn, discussion, nomination, vote, and reveal from every role perspective", async () => {
      for (const player of players) {
        await expect(player.page.getByRole("heading", { name: /Dawn breaks/i })).toBeVisible({ timeout: 20_000 });
      }
      await captureRoleFrames(rolePlayers, "03-dawn");

      for (const player of players) {
        await expect(player.page.getByRole("heading", { name: /Day discussion/i })).toBeVisible({ timeout: 20_000 });
      }
      await captureRoleFrames(rolePlayers, "04-discussion");

      const firstMafia = playersWithRole(players, rolesByName, "mafia")[0];
      const citizenPlayer = rolePlayers.get("citizen");
      if (!firstMafia || !citizenPlayer) throw new Error("missing target players");

      for (const player of players) {
        await expect(player.page.getByRole("heading", { name: /Nominations/i })).toBeVisible({ timeout: 25_000 });
      }
      await captureRoleFrames(rolePlayers, "05-nomination");
      for (const player of players) {
        const nominateTarget = rolesByName.get(player.name) === "mafia" ? citizenPlayer.name : firstMafia.name;
        await clickIfPresent(player.page, nominateTarget);
      }

      for (const player of players) {
        await expect(player.page.getByRole("heading", { name: /Day vote/i })).toBeVisible({ timeout: 25_000 });
      }
      await captureRoleFrames(rolePlayers, "06-vote");
      for (const player of players) {
        await clickIfPresent(player.page, firstMafia.name);
      }

      for (const player of players) {
        await expect(player.page.getByRole("heading", { name: /Vote reveal/i })).toBeVisible({ timeout: 15_000 });
      }
      await captureRoleFrames(rolePlayers, "07-reveal");
    });
  } finally {
    for (const player of players) {
      await player.ctx.close().catch(() => {});
    }
  }
});
