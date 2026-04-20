// GameEnd screen: terminal view once the server broadcasts S2CGameEnd.
// Shows final standings + a button to return to the lobby. The server has
// already reset Players.Ready=false, so "back to lobby" just means letting
// the Lobby render itself; we surface a restart button by calling Ready
// on behalf of the user (they still have to click, no auto-ready).

import { useMemo } from "react";
import { C2S } from "../proto";
import { client, useGameState } from "../store";

export default function GameEnd(): JSX.Element {
  const g = useGameState();

  const rows = useMemo(() => {
    return g.players
      .map((p) => ({ id: p.id, name: p.name, score: g.scores[p.id] ?? 0 }))
      .sort((a, b) => b.score - a.score || a.name.localeCompare(b.name));
  }, [g.players, g.scores]);

  const winner = rows[0];

  return (
    <div className="stack">
      <div className="card">
        <h2>Game over</h2>
        {winner && (
          <p>
            <strong>{winner.name}</strong> wins with {winner.score}.
          </p>
        )}
      </div>

      <div className="card">
        <h3>Final scores</h3>
        {rows.map((r, i) => (
          <div key={r.id} className="score-row">
            <span>
              {i + 1}. {r.name}
              {r.id === g.playerId ? " (you)" : ""}
            </span>
            <span style={{ fontVariantNumeric: "tabular-nums" }}>{r.score}</span>
          </div>
        ))}
      </div>

      <div className="row">
        <button className="primary" onClick={() => client.send(C2S.Ready, { ready: true })}>
          Ready for another
        </button>
        <button onClick={() => client.disconnect()}>Leave</button>
      </div>
    </div>
  );
}
