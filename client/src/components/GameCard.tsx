import type { GameDefinition } from "../proto";

type Props = {
  game: GameDefinition;
  selected: boolean;
  canSelect: boolean;
  playerCount: number;
  onSelect: () => void;
};

export default function GameCard({ game, selected, canSelect, playerCount, onSelect }: Props): JSX.Element {
  const playerHint = `${game.min_players}-${game.max_players} players`;
  const planned = game.status !== "available";
  const underMin = playerCount < game.min_players;

  return (
    <article className={`card game-card${selected ? " selected" : ""}`}>
      <div className="row" style={{ justifyContent: "space-between", alignItems: "flex-start" }}>
        <div>
          <h3>{game.name}</h3>
          <p className="muted">{game.summary}</p>
        </div>
        <span className={`pill ${planned ? "offline" : "ready"}`}>
          {planned ? "Coming soon" : "Playable"}
        </span>
      </div>
      <div className="row wrap" style={{ gap: 8 }}>
        <span className="pill">{playerHint}</span>
        <span className="pill">{game.estimated_minutes} min</span>
        {game.tags.map((tag) => (
          <span key={tag} className="pill">
            {tag}
          </span>
        ))}
      </div>
      <p className="muted">
        {planned
          ? "This game is planned and visible in the pack, but not playable yet."
          : underMin
            ? `Needs ${game.min_players} players before the host can start.`
            : "Ready to bring into the lobby."}
      </p>
      <button className="primary" disabled={!canSelect} onClick={onSelect}>
        {selected ? `${game.name} selected` : planned ? "Coming soon" : `Choose ${game.name}`}
      </button>
    </article>
  );
}
