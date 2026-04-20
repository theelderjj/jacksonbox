// Leaderboard is a persistent side panel shown under every in-game screen.
// It reads g.scores (authoritative from the server) and g.players for the
// names. Sorted descending by score, ties broken by name for determinism.

import { useMemo } from "react";
import { useGameState } from "../store";

export default function Leaderboard(): JSX.Element | null {
  const g = useGameState();

  const rows = useMemo(() => {
    const out = g.players
      .map((p) => ({
        id: p.id,
        name: p.name,
        connected: p.connected,
        score: g.scores[p.id] ?? 0,
      }))
      .sort((a, b) => {
        if (b.score !== a.score) return b.score - a.score;
        return a.name.localeCompare(b.name);
      });
    return out;
  }, [g.players, g.scores]);

  if (rows.length === 0) return null;

  return (
    <div className="card">
      <h3>Leaderboard</h3>
      {rows.map((r, i) => (
        <div key={r.id} className="score-row">
          <span>
            {i + 1}. {r.name}
            {r.id === g.playerId ? " (you)" : ""}
            {!r.connected && <span className="pill offline" style={{ marginLeft: 8 }}>offline</span>}
          </span>
          <span style={{ fontVariantNumeric: "tabular-nums" }}>{r.score}</span>
        </div>
      ))}
    </div>
  );
}
