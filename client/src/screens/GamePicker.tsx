import { C2S } from "../proto";
import { client, selfPlayer, useGameState } from "../store";
import GameCard from "../components/GameCard";

export default function GamePicker(): JSX.Element {
  const g = useGameState();
  const me = selfPlayer(g);
  const isLeader = !!me && g.leaderId === me.id;
  const connectedPlayers = g.players.filter((p) => p.connected).length;

  return (
    <div className="stack">
      <div className="card">
        <h2>Jackson Box</h2>
        <p className="muted">
          Pick the next party game. Jrawful, Fake Artist, Draw Duel, Reaction Duel, Price is
          Right, Split the Vote, and Mafia are all live in the current pack.
        </p>
        <p className="muted">
          {isLeader
            ? "Choose a game to move everyone into its lobby."
            : "Waiting for the host to choose a game."}
        </p>
      </div>

      <div className="grid">
        {g.gameCatalog.map((game) => (
          <GameCard
            key={game.id}
            game={game}
            selected={g.selectedGameId === game.id}
            canSelect={isLeader && game.status === "available"}
            playerCount={connectedPlayers}
            onSelect={() => client.send(C2S.SelectGame, { game_id: game.id })}
          />
        ))}
      </div>

      <div className="card">
        <h3>Tournament Mode</h3>
        <p className="muted">
          Playlist mode, winner-chooses-next flow, persistent stats, accounts, invites, and access
          controls are documented as future seams and are not wired yet in this branch.
        </p>
      </div>
    </div>
  );
}
