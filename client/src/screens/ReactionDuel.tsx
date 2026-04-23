import Countdown from "../components/Countdown";
import { C2S } from "../proto";
import { client, useGameState } from "../store";

export default function ReactionDuel(): JSX.Element {
  const g = useGameState();
  const phase = g.phase;
  const countdownSeconds = Number(g.settings.game_options?.countdown_seconds ?? 3);
  const inWait = phase.startsWith("reaction_wait");
  const inTap = phase.startsWith("reaction_tap");
  const inReveal = phase.startsWith("reaction_reveal");

  if (inReveal) {
    return (
      <div className="stack">
        <div className="card">
          <h2>Reaction Duel Results</h2>
          <p className="muted">Fastest legal tap wins. False starts are out for the round.</p>
        </div>
        <div className="card">
          {g.reactionResults.map((result) => (
            <div key={result.player_id} className="row" style={{ justifyContent: "space-between" }}>
              <div>
                <strong>{result.player_name}</strong>
                <div className="muted">
                  {result.false_start
                    ? "False start"
                    : result.rank
                      ? `Rank #${result.rank}${result.reaction_ms != null ? ` | ${result.reaction_ms} ms` : ""}`
                      : "Did not tap in time"}
                </div>
              </div>
              <span>{result.points} pts</span>
            </div>
          ))}
        </div>
      </div>
    );
  }

  const title = inTap ? "Tap now" : inWait ? "Wait for green" : `Countdown: ${countdownSeconds}s`;
  const subtitle = inTap
    ? "The button is green. Tap once as fast as you can."
    : "The button is red. If you press before green, you lose the round.";

  return (
    <div className="stack">
      <div className="card">
        <div className="row" style={{ justifyContent: "space-between" }}>
          <div>
            <h2>{title}</h2>
            <p className="muted">{subtitle}</p>
          </div>
          <Countdown deadlineMs={g.deadlineMs} paused={g.paused} remainingMs={g.pauseRemainingMs} />
        </div>
      </div>

      <div className="card">
        <button
          className={`duel-button ${inTap ? "go" : "wait"}`}
          onClick={() => client.send(C2S.SubmitTap, {})}
        >
          {inTap ? "TAP" : "DON'T TAP"}
        </button>
      </div>
    </div>
  );
}
