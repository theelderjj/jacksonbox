import { useState } from "react";
import { getLobbyGuide, getMafiaRoleGuide, mafiaRoleGuides } from "../components/gameGuides";
import RulesIntro from "../components/RulesIntro";
import { getRulesDeck } from "../components/rulesDecks";
import { C2S } from "../proto";
import { client, selfPlayer, useGameState } from "../store";

export default function Lobby(): JSX.Element {
  const g = useGameState();
  const me = selfPlayer(g);
  const [busy, setBusy] = useState(false);
  const [mafiaRolePreview, setMafiaRolePreview] = useState("mafia");
  const isLeader = !!me && g.leaderId === me.id;
  const connectedPlayers = g.players.filter((p) => p.connected);
  const readyPlayers = connectedPlayers.filter((p) => p.ready);
  const selectedGame = g.gameCatalog.find((game) => game.id === g.selectedGameId) ?? null;
  const lobbyGuide = getLobbyGuide(g.selectedGameId);
  const visualRulesDeck = g.selectedGameId ? getRulesDeck(g.selectedGameId) : null;
  const previewedRole = getMafiaRoleGuide(mafiaRolePreview);
  const mafiaDefaults = defaultMafiaLobbyOptions(connectedPlayers.length);
  const roundOptions = g.selectedGameId === "fake_artist"
    ? Array.from({ length: 60 }, (_, idx) => idx + 1)
    : [1, 2, 3, 4, 5, 6, 7, 8, 9, 10];

  function toggleReady(): void {
    if (!me) return;
    setBusy(true);
    client.send(C2S.Ready, { ready: !me.ready });
    setTimeout(() => setBusy(false), 200);
  }

  function updateSettings(next: {
    round_count?: number;
    generated_fake_count?: number;
    drawing_seconds?: number;
    fake_prompt_seconds?: number;
    voting_seconds?: number;
    game_options?: Record<string, unknown>;
  }): void {
    if (!isLeader) return;
    client.send(C2S.UpdateSettings, {
      game_id: g.selectedGameId ?? g.settings.game_id,
      round_count: next.round_count ?? g.settings.round_count,
      generated_fake_count: next.generated_fake_count ?? g.settings.generated_fake_count,
      drawing_seconds: next.drawing_seconds ?? g.settings.drawing_seconds,
      fake_prompt_seconds: next.fake_prompt_seconds ?? g.settings.fake_prompt_seconds,
      voting_seconds: next.voting_seconds ?? g.settings.voting_seconds,
      game_options: next.game_options ?? g.settings.game_options ?? {},
    });
  }

  return (
    <div className="stack">
      <div className="card">
        <h2>{selectedGame?.name ?? "Game"} Lobby</h2>
        <p className="muted">
          Room {g.room?.room_id ?? "..."}. Everyone readies here, then the host explicitly starts
          the match when the room is set.
        </p>
        {selectedGame && (
          <p className="muted">
            {selectedGame.summary} Supports {selectedGame.min_players}-{selectedGame.max_players}{" "}
            players.
          </p>
        )}
        <div className="grid">
          {g.players.map((p) => (
            <div key={p.id} className="row" style={{ justifyContent: "space-between" }}>
              <span>
                {p.name}
                {p.id === g.playerId ? " (you)" : ""}
              </span>
              <span>
                {!p.connected && <span className="pill offline">offline</span>}
                {p.ready && <span className="pill ready">ready</span>}
              </span>
            </div>
          ))}
        </div>
      </div>

      <div className="card">
        <h2>Game settings</h2>
        <p className="muted">
          {isLeader
            ? "You are the host. Changing settings clears ready checks so everyone can confirm."
            : "Only the host can change these settings."}
        </p>
        <div className="settings-grid">
          <label>
            <span>Rounds</span>
            <select
              value={g.settings.round_count}
              disabled={!isLeader}
              onChange={(e) => updateSettings({ round_count: Number(e.target.value) })}
            >
              {roundOptions.map((n) => (
                <option key={n} value={n}>
                  {n}
                </option>
              ))}
            </select>
          </label>
          {g.selectedGameId === "jrawful" && (
            <>
              <label>
                <span>Generated fake prompts per drawing</span>
                <select
                  value={g.settings.generated_fake_count}
                  disabled={!isLeader}
                  onChange={(e) => updateSettings({ generated_fake_count: Number(e.target.value) })}
                >
                  {[0, 1, 2, 3, 4, 5, 6].map((n) => (
                    <option key={n} value={n}>
                      {n}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                <span>Drawing time</span>
                <select
                  value={g.settings.drawing_seconds}
                  disabled={!isLeader}
                  onChange={(e) => updateSettings({ drawing_seconds: Number(e.target.value) })}
                >
                  {[30, 45, 60, 75, 90, 120, 180, 240, 300].map((seconds) => (
                    <option key={seconds} value={seconds}>
                      {formatSeconds(seconds)}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                <span>Fake prompt time</span>
                <select
                  value={g.settings.fake_prompt_seconds}
                  disabled={!isLeader}
                  onChange={(e) => updateSettings({ fake_prompt_seconds: Number(e.target.value) })}
                >
                  {[30, 45, 60, 75, 90, 120, 180, 240, 300].map((seconds) => (
                    <option key={seconds} value={seconds}>
                      {formatSeconds(seconds)}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                <span>Voting time</span>
                <select
                  value={g.settings.voting_seconds}
                  disabled={!isLeader}
                  onChange={(e) => updateSettings({ voting_seconds: Number(e.target.value) })}
                >
                  {[10, 15, 20, 30, 45, 60, 90, 120, 180].map((seconds) => (
                    <option key={seconds} value={seconds}>
                      {formatSeconds(seconds)}
                    </option>
                  ))}
                </select>
              </label>
            </>
          )}
          {g.selectedGameId === "reaction_duel" && (
            <label>
              <span>Visible countdown</span>
              <select
                value={Number(g.settings.game_options?.countdown_seconds ?? 3)}
                disabled={!isLeader}
                onChange={(e) =>
                  updateSettings({
                    game_options: {
                      ...(g.settings.game_options ?? {}),
                      countdown_seconds: Number(e.target.value),
                    },
                  })
                }
              >
                {[1, 2, 3, 4, 5, 6, 7, 8, 9, 10].map((seconds) => (
                  <option key={seconds} value={seconds}>
                    {formatSeconds(seconds)}
                  </option>
                ))}
              </select>
            </label>
          )}
          {g.selectedGameId === "word_storm" && (
            <>
              <div className="card" style={{ gridColumn: "1 / -1", background: "#0f1720" }}>
                <h3>How scoring works</h3>
                <p className="muted">
                  Submit as many dictionary words as you can for each letter. Letters rotate on a
                  timer, words can only be used once per game, and scoring happens at the end.
                </p>
              </div>
              <label>
                <span>Letter timer</span>
                <select
                  value={Number(g.settings.game_options?.letter_seconds ?? 20)}
                  disabled={!isLeader}
                  onChange={(e) =>
                    updateSettings({
                      game_options: {
                        ...(g.settings.game_options ?? {}),
                        letter_seconds: Number(e.target.value),
                      },
                    })
                  }
                >
                  {[10, 15, 20, 25, 30, 45, 60].map((seconds) => (
                    <option key={seconds} value={seconds}>
                      {formatSeconds(seconds)}
                    </option>
                  ))}
                </select>
              </label>
            </>
          )}
          {g.selectedGameId === "price_is_right" && (
            <>
              <label>
                <span>Guess time</span>
                <select
                  value={g.settings.voting_seconds}
                  disabled={!isLeader}
                  onChange={(e) => updateSettings({ voting_seconds: Number(e.target.value) })}
                >
                  {[10, 15, 20, 30, 45, 60].map((seconds) => (
                    <option key={seconds} value={seconds}>
                      {formatSeconds(seconds)}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                <span>Minimum item price</span>
                <select
                  value={Number(g.settings.game_options?.minimum_price_dollars ?? 5)}
                  disabled={!isLeader}
                  onChange={(e) =>
                    updateSettings({
                      game_options: {
                        ...(g.settings.game_options ?? {}),
                        minimum_price_dollars: Number(e.target.value),
                      },
                    })
                  }
                >
                  {[5, 10, 25, 50, 100, 500].map((dollars) => (
                    <option key={dollars} value={dollars}>
                      ${dollars}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                <span>Threshold mode</span>
                <select
                  value={String(g.settings.game_options?.threshold_mode ?? "fixed")}
                  disabled={!isLeader}
                  onChange={(e) =>
                    updateSettings({
                      game_options: {
                        ...(g.settings.game_options ?? {}),
                        threshold_mode: e.target.value,
                      },
                    })
                  }
                >
                  <option value="fixed">Fixed each round</option>
                  <option value="times_ten">Increase by x10 each round</option>
                </select>
              </label>
            </>
          )}
          {g.selectedGameId === "draw_duel" && (
            <>
              <label>
                <span>Draw time</span>
                <select
                  value={g.settings.drawing_seconds}
                  disabled={!isLeader}
                  onChange={(e) => updateSettings({ drawing_seconds: Number(e.target.value) })}
                >
                  {[20, 30, 45, 60, 75, 90, 120, 180].map((seconds) => (
                    <option key={seconds} value={seconds}>
                      {formatSeconds(seconds)}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                <span>Judging time</span>
                <select
                  value={g.settings.voting_seconds}
                  disabled={!isLeader}
                  onChange={(e) => updateSettings({ voting_seconds: Number(e.target.value) })}
                >
                  {[10, 15, 20, 30, 45, 60, 90].map((seconds) => (
                    <option key={seconds} value={seconds}>
                      {formatSeconds(seconds)}
                    </option>
                  ))}
                </select>
              </label>
            </>
          )}
          {g.selectedGameId === "fake_artist" && (
            <>
              <label>
                <span>Draw time per player</span>
                <select
                  value={g.settings.drawing_seconds}
                  disabled={!isLeader}
                  onChange={(e) => updateSettings({ drawing_seconds: Number(e.target.value) })}
                >
                  {[10, 15, 20, 30, 45, 60].map((seconds) => (
                    <option key={seconds} value={seconds}>
                      {formatSeconds(seconds)}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                <span>Voting time</span>
                <select
                  value={g.settings.voting_seconds}
                  disabled={!isLeader}
                  onChange={(e) => updateSettings({ voting_seconds: Number(e.target.value) })}
                >
                  {[15, 20, 30, 45, 60, 90, 120].map((seconds) => (
                    <option key={seconds} value={seconds}>
                      {formatSeconds(seconds)}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                <span>Replay count</span>
                <select
                  value={Number(g.settings.game_options?.replay_count ?? 1)}
                  disabled={!isLeader}
                  onChange={(e) =>
                    updateSettings({
                      game_options: {
                        ...(g.settings.game_options ?? {}),
                        replay_count: Number(e.target.value),
                      },
                    })
                  }
                >
                  {[1, 2, 3, 4].map((count) => (
                    <option key={count} value={count}>
                      {count}
                    </option>
                  ))}
                </select>
              </label>
              <label className="stack">
                <span>Color code each drawer</span>
                <button
                  type="button"
                  disabled={!isLeader}
                  onClick={() =>
                    updateSettings({
                      game_options: {
                        ...(g.settings.game_options ?? {}),
                        color_coded_drawers: !Boolean(g.settings.game_options?.color_coded_drawers),
                      },
                    })
                  }
                >
                  {Boolean(g.settings.game_options?.color_coded_drawers) ? "On" : "Off"}
                </button>
              </label>
              <label className="stack">
                <span>Continuous replay during voting</span>
                <button
                  type="button"
                  disabled={!isLeader}
                  onClick={() =>
                    updateSettings({
                      game_options: {
                        ...(g.settings.game_options ?? {}),
                        continuous_replay: !Boolean(g.settings.game_options?.continuous_replay),
                      },
                    })
                  }
                >
                  {Boolean(g.settings.game_options?.continuous_replay) ? "On" : "Off"}
                </button>
              </label>
            </>
          )}
          {g.selectedGameId === "split_vote" && (
            <>
              <label>
                <span>Target mode</span>
                <select
                  value={String(g.settings.game_options?.target_mode ?? "strict_split")}
                  disabled={!isLeader}
                  onChange={(e) =>
                    updateSettings({
                      game_options: {
                        ...(g.settings.game_options ?? {}),
                        target_mode: e.target.value,
                      },
                    })
                  }
                >
                  <option value="strict_split">Strict split</option>
                  <option value="variable">Variable target</option>
                  <option value="odd_one">Get the odd one</option>
                </select>
              </label>
              <label>
                <span>Prompt authoring</span>
                <select
                  value={String(g.settings.game_options?.authoring_mode ?? "splitter_prompt_and_options")}
                  disabled={!isLeader}
                  onChange={(e) =>
                    updateSettings({
                      game_options: {
                        ...(g.settings.game_options ?? {}),
                        authoring_mode: e.target.value,
                      },
                    })
                  }
                >
                  <option value="splitter_prompt_and_options">Splitter writes prompt + options</option>
                  <option value="generated_prompt_splitter_options">Generated prompt, splitter writes options</option>
                </select>
              </label>
            </>
          )}
          {g.selectedGameId === "mafia" && (
            <>
              <div className="card" style={{ gridColumn: "1 / -1", background: "#0f1720" }}>
                <h3>Auto-scaled setup</h3>
                <p className="muted">
                  Mafia roles scale automatically with the connected player count. Current room setup:
                  {" "}
                  {mafiaRoleSummary(connectedPlayers.length)}.
                </p>
              </div>
              <label>
                <span>Max day cycles</span>
                <select
                  value={g.settings.round_count}
                  disabled={!isLeader}
                  onChange={(e) => updateSettings({ round_count: Number(e.target.value) })}
                >
                  {Array.from({ length: 15 }, (_, idx) => idx + 1).map((count) => (
                    <option key={count} value={count}>
                      {count}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                <span>Night action time</span>
                <select
                  value={g.settings.drawing_seconds}
                  disabled={!isLeader}
                  onChange={(e) => updateSettings({ drawing_seconds: Number(e.target.value) })}
                >
                  {[10, 15, 20, 25, 30, 45, 60].map((seconds) => (
                    <option key={seconds} value={seconds}>
                      {formatSeconds(seconds)}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                <span>Day discussion time</span>
                <select
                  value={g.settings.fake_prompt_seconds}
                  disabled={!isLeader}
                  onChange={(e) => updateSettings({ fake_prompt_seconds: Number(e.target.value) })}
                >
                  {[20, 30, 45, 60, 90, 120, 180].map((seconds) => (
                    <option key={seconds} value={seconds}>
                      {formatSeconds(seconds)}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                <span>Day vote time</span>
                <select
                  value={g.settings.voting_seconds}
                  disabled={!isLeader}
                  onChange={(e) => updateSettings({ voting_seconds: Number(e.target.value) })}
                >
                  {[10, 15, 20, 30, 45, 60].map((seconds) => (
                    <option key={seconds} value={seconds}>
                      {formatSeconds(seconds)}
                    </option>
                  ))}
                </select>
              </label>
              <label className="stack">
                <span>Reveal roles on death</span>
                <button
                  type="button"
                  disabled={!isLeader}
                  onClick={() =>
                    updateSettings({
                      game_options: {
                        ...(g.settings.game_options ?? {}),
                        reveal_on_death: !Boolean(
                          g.settings.game_options?.reveal_on_death ?? mafiaDefaults.revealOnDeath,
                        ),
                      },
                    })
                  }
                >
                  {Boolean(g.settings.game_options?.reveal_on_death ?? mafiaDefaults.revealOnDeath) ? "On" : "Off"}
                </button>
                <span className="muted">
                  Turn this on for newer groups or faster deduction games, since every elimination confirms useful information.
                  Leave it off when you want longer bluffing, shakier reads, and more room for Mafia to hide after a kill.
                </span>
              </label>
              <label className="stack">
                <span>Doctor can self-protect</span>
                <button
                  type="button"
                  disabled={!isLeader}
                  onClick={() =>
                    updateSettings({
                      game_options: {
                        ...(g.settings.game_options ?? {}),
                        self_protect: !Boolean(
                          g.settings.game_options?.self_protect ?? mafiaDefaults.selfProtect,
                        ),
                      },
                    })
                  }
                >
                  {Boolean(g.settings.game_options?.self_protect ?? mafiaDefaults.selfProtect) ? "On" : "Off"}
                </button>
              </label>
              <label>
                <span>Tie resolution</span>
                <select
                  value={String(g.settings.game_options?.tie_rule ?? mafiaDefaults.tieRule)}
                  disabled={!isLeader}
                  onChange={(e) =>
                    updateSettings({
                      game_options: {
                        ...(g.settings.game_options ?? {}),
                        tie_rule: e.target.value,
                      },
                    })
                  }
                >
                  <option value="no_elimination">No elimination</option>
                  <option value="revote">Revote</option>
                  <option value="mayor_breaks">Mayor breaks tie</option>
                </select>
              </label>
              <label>
                <span>Day voting style</span>
                <select
                  value={String(g.settings.game_options?.day_vote_mode ?? "reveal_end")}
                  disabled={!isLeader}
                  onChange={(e) =>
                    updateSettings({
                      game_options: {
                        ...(g.settings.game_options ?? {}),
                        day_vote_mode: e.target.value,
                      },
                    })
                  }
                >
                  <option value="reveal_end">Hidden until reveal</option>
                  <option value="live_public">Live public switching</option>
                  <option value="sequential_public">Public one by one</option>
                </select>
                <span className="muted">
                  <strong> Hidden until reveal:</strong> everybody chooses in private, then votes are shown at the end.
                  <strong> Live public switching:</strong> votes stay visible and can change until the timer expires.
                  <strong> Public one by one:</strong> players vote in order and each vote locks immediately.
                </span>
              </label>
              <label>
                <span>Detective investigation</span>
                <select
                  value={String(g.settings.game_options?.investigation_mode ?? mafiaDefaults.investigationMode)}
                  disabled={!isLeader}
                  onChange={(e) =>
                    updateSettings({
                      game_options: {
                        ...(g.settings.game_options ?? {}),
                        investigation_mode: e.target.value,
                      },
                    })
                  }
                >
                  <option value="faction">Mafia or Town</option>
                  <option value="exact_role">Exact role</option>
                </select>
                <span className="muted">
                  <strong> Mafia or Town:</strong> safer, swingier baseline play where the detective only learns allegiance.
                  <strong> Exact role:</strong> much stronger information that is better for larger rooms or harder town setups.
                </span>
              </label>
            </>
          )}
        </div>
      </div>

      <div className="row">
        <button className="primary" disabled={!me || busy} onClick={toggleReady}>
          {me?.ready ? "Not ready" : "I'm ready"}
        </button>
        {isLeader && (
          <button
            className="primary"
            disabled={
              !selectedGame ||
              connectedPlayers.length < (selectedGame?.min_players ?? 3) ||
              readyPlayers.length !== connectedPlayers.length
            }
            onClick={() => client.send(C2S.StartGame, {})}
          >
            Start game
          </button>
        )}
        {isLeader && (
          <button onClick={() => client.send(C2S.ReturnToPicker, {})}>Back to picker</button>
        )}
        <button onClick={() => client.disconnect()}>Leave</button>
      </div>

      {visualRulesDeck && <RulesIntro deck={visualRulesDeck} />}

      <div className="card instruction-card">
        <h2>{lobbyGuide.title}</h2>
        <p className="muted">{lobbyGuide.intro}</p>
        <div className="lobby-guide-grid">
          {lobbyGuide.sections.map((section) => (
            <div key={section.title} className="choice">
              <strong>{section.title}</strong>
              <div className="muted" style={{ marginTop: 8 }}>
                {section.body}
              </div>
              <ul className="lobby-guide-bullets">
                {section.bullets.map((bullet) => (
                  <li key={bullet}>{bullet}</li>
                ))}
              </ul>
            </div>
          ))}
        </div>
        {g.selectedGameId === "mafia" && (
          <div className="mafia-role-preview">
            <div className="row" style={{ justifyContent: "space-between", alignItems: "center" }}>
              <h3>Role preview</h3>
              <span className="muted">Click any role to see how it plays before the match starts.</span>
            </div>
            <div className="mafia-role-pills">
              {mafiaRoleGuides.map((roleGuide) => (
                <button
                  key={roleGuide.roleId}
                  type="button"
                  className={mafiaRolePreview === roleGuide.roleId ? "primary" : ""}
                  onClick={() => setMafiaRolePreview(roleGuide.roleId)}
                >
                  {roleGuide.name}
                </button>
              ))}
            </div>
            <div className="lobby-role-card">
              <h3>{previewedRole.name}</h3>
              <p className="muted">{previewedRole.goal}</p>
              <div className="lobby-guide-grid">
                <div className="choice">
                  <strong>Night</strong>
                  <div className="muted" style={{ marginTop: 8 }}>
                    {previewedRole.night}
                  </div>
                </div>
                <div className="choice">
                  <strong>Day</strong>
                  <div className="muted" style={{ marginTop: 8 }}>
                    {previewedRole.day}
                  </div>
                </div>
                <div className="choice">
                  <strong>Key tip</strong>
                  <div className="muted" style={{ marginTop: 8 }}>
                    {previewedRole.tip}
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}
        {lobbyGuide.footer && <p className="muted">{lobbyGuide.footer}</p>}
      </div>
    </div>
  );
}

function formatSeconds(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  const mins = Math.floor(seconds / 60);
  const rest = seconds % 60;
  return rest === 0 ? `${mins}m` : `${mins}m ${rest}s`;
}

function defaultMafiaLobbyOptions(playerCount: number): {
  revealOnDeath: boolean;
  selfProtect: boolean;
  tieRule: "no_elimination" | "revote" | "mayor_breaks";
  investigationMode: "faction" | "exact_role";
} {
  if (playerCount <= 6) {
    return {
      revealOnDeath: true,
      selfProtect: false,
      tieRule: "no_elimination",
      investigationMode: "faction",
    };
  }
  if (playerCount <= 8) {
    return {
      revealOnDeath: true,
      selfProtect: true,
      tieRule: "revote",
      investigationMode: "faction",
    };
  }
  if (playerCount <= 10) {
    return {
      revealOnDeath: true,
      selfProtect: true,
      tieRule: "mayor_breaks",
      investigationMode: "faction",
    };
  }
  return {
    revealOnDeath: false,
    selfProtect: true,
    tieRule: "revote",
    investigationMode: "exact_role",
  };
}

function mafiaRoleSummary(playerCount: number): string {
  if (playerCount <= 6) {
    return "1 Mafia, 1 Detective, 1 Doctor, and the rest Citizens";
  }
  if (playerCount <= 8) {
    return "2 Mafia, 1 Detective, 1 Doctor, 1 Mayor, and the rest Citizens";
  }
  if (playerCount <= 10) {
    return "2 Mafia, 1 Detective, 1 Doctor, 1 Mayor, and the rest Citizens";
  }
  return "3 Mafia, 2 Detectives, 1 Doctor, 1 Mayor, and the rest Citizens";
}
