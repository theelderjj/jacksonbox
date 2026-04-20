import { useState } from "react";
import { C2S } from "../proto";
import { client, selfPlayer, useGameState } from "../store";

export default function Lobby(): JSX.Element {
  const g = useGameState();
  const me = selfPlayer(g);
  const [busy, setBusy] = useState(false);
  const isLeader = !!me && g.leaderId === me.id;

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
  }): void {
    if (!isLeader) return;
    client.send(C2S.UpdateSettings, {
      round_count: next.round_count ?? g.settings.round_count,
      generated_fake_count: next.generated_fake_count ?? g.settings.generated_fake_count,
      drawing_seconds: next.drawing_seconds ?? g.settings.drawing_seconds,
      fake_prompt_seconds: next.fake_prompt_seconds ?? g.settings.fake_prompt_seconds,
      voting_seconds: next.voting_seconds ?? g.settings.voting_seconds,
    });
  }

  return (
    <div className="stack">
      <div className="card">
        <h2>Lobby - {g.room?.room_id ?? "..."}</h2>
        <p className="muted">
          Round {g.round || 0}. Waiting for all players to ready up (minimum 3).
        </p>
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
              {[1, 2, 3, 4, 5, 6, 7, 8, 9, 10].map((n) => (
                <option key={n} value={n}>
                  {n}
                </option>
              ))}
            </select>
          </label>
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
        </div>
      </div>

      <div className="row">
        <button className="primary" disabled={!me || busy} onClick={toggleReady}>
          {me?.ready ? "Not ready" : "I'm ready"}
        </button>
        <button onClick={() => client.disconnect()}>Leave</button>
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
