import { C2S } from "../proto";
import { client, selfPlayer, useGameState } from "../store";

export default function PlatformResults(): JSX.Element {
  const g = useGameState();
  const me = selfPlayer(g);
  const isLeader = !!me && g.leaderId === me.id;
  const standings = [...g.players]
    .sort((a, b) => (g.scores[b.id] ?? 0) - (g.scores[a.id] ?? 0))
    .map((player, index) => ({
      rank: index + 1,
      player,
      score: g.scores[player.id] ?? 0,
    }));

  return (
    <div className="stack">
      <div className="card">
        <h2>Final scores</h2>
        <p className="muted">
          The round set is over. The host can head back to the picker and line up the next game.
        </p>
      </div>

      <div className="card">
        {standings.map((entry) => (
          <div key={entry.player.id} className="row" style={{ justifyContent: "space-between" }}>
            <strong>
              #{entry.rank} {entry.player.name}
              {entry.player.id === g.playerId ? " (you)" : ""}
            </strong>
            <span>{entry.score} pts</span>
          </div>
        ))}
      </div>

      <div className="row">
        {isLeader ? (
          <button className="primary" onClick={() => client.send(C2S.ReturnToPicker, {})}>
            Choose another game
          </button>
        ) : (
          <div className="muted">Waiting for the host to return to the game picker.</div>
        )}
      </div>
    </div>
  );
}
