import Countdown from "../components/Countdown";
import { C2S } from "../proto";
import { client, selfPlayer, useGameState } from "../store";

export default function Mafia(): JSX.Element {
  const g = useGameState();
  const me = selfPlayer(g);
  const state = g.mafiaState;
  const reveal = g.mafiaReveal;

  if (g.phase === "mafia_role_assign") {
    return (
      <div className="stack">
        <div className="card">
          <div className="row" style={{ justifyContent: "space-between" }}>
            <h2>Mafia</h2>
            <Countdown deadlineMs={g.deadlineMs} paused={g.paused} remainingMs={g.pauseRemainingMs} />
          </div>
          <p className="muted">Roles are going out. Keep your card private.</p>
          <h3>Your role: {formatRole(state?.your_role)}</h3>
          {state?.note && <p className="muted">{state.note}</p>}
          {state?.team_ids?.length ? (
            <p className="muted">Mafia team: {namesFor(g, state.team_ids).join(", ")}</p>
          ) : null}
          <RoleGuide role={state?.your_role} phase="role" />
          <AliveList />
        </div>
      </div>
    );
  }

  if (g.phase.startsWith("mafia_night_collect")) {
    const canAct = Boolean(state?.can_act);
    return (
      <div className="stack">
        <div className="card">
          <div className="row" style={{ justifyContent: "space-between" }}>
            <h2>Night falls</h2>
            <Countdown deadlineMs={g.deadlineMs} paused={g.paused} remainingMs={g.pauseRemainingMs} />
          </div>
          <p className="muted">{state?.note ?? "The town goes to sleep."}</p>
          <h3>Your role: {formatRole(state?.your_role)}</h3>
          {state?.team_ids?.length ? (
            <p className="muted">Mafia team: {namesFor(g, state.team_ids).join(", ")}</p>
          ) : null}
          <RoleGuide role={state?.your_role} phase="night" />
          {canAct ? (
            <>
              <p className="muted">{state?.locked_in ? "Action locked in. You can change it before time runs out." : "Choose your target."}</p>
              <div className="grid">
                {(state?.target_ids ?? []).map((targetID) => (
                  <button
                    key={targetID}
                    className="choice"
                    onClick={() => client.send(C2S.SubmitVote, { drawing_id: "mafia_night", choice_id: targetID })}
                  >
                    {nameFor(g, targetID)}
                  </button>
                ))}
              </div>
            </>
          ) : (
            <p className="muted">You have no night action this turn.</p>
          )}
          <AliveList />
        </div>
      </div>
    );
  }

  if (g.phase.startsWith("mafia_night_reveal")) {
    const deaths = reveal?.deaths ?? [];
    return (
      <div className="stack">
        <div className="card">
          <h2>Dawn breaks</h2>
          <p className="muted">
            {deaths.length === 0 ? "No one died overnight." : `The town lost ${namesFor(g, deaths).join(", ")}.`}
          </p>
          {reveal?.winner && <p className="muted">{winnerLine(reveal.winner)}</p>}
          <AliveList />
        </div>
      </div>
    );
  }

  if (g.phase.startsWith("mafia_day_discuss")) {
    return (
      <div className="stack">
        <div className="card">
          <div className="row" style={{ justifyContent: "space-between" }}>
            <h2>Day discussion</h2>
            <Countdown deadlineMs={g.deadlineMs} paused={g.paused} remainingMs={g.pauseRemainingMs} />
          </div>
          <p className="muted">{state?.note ?? "Talk through the night and decide who to eliminate."}</p>
          <RoleGuide role={state?.your_role} phase="day" />
          <AliveList />
        </div>
      </div>
    );
  }

  if (g.phase.startsWith("mafia_day_nominate")) {
    const canNominate = Boolean(me && isAlive(state?.alive_players, me.id));
    return (
      <div className="stack">
        <div className="card">
          <div className="row" style={{ justifyContent: "space-between" }}>
            <h2>Nominations</h2>
            <Countdown deadlineMs={g.deadlineMs} paused={g.paused} remainingMs={g.pauseRemainingMs} />
          </div>
          <p className="muted">{state?.note ?? "Nominate one player for the elimination vote."}</p>
          <RoleGuide role={state?.your_role} phase="nominate" />
          {canNominate ? (
            <>
              <p className="muted">
                {state?.locked_in ? "Nomination locked in. You can still change it before time ends." : "Pick one suspect."}
              </p>
              <div className="grid">
                {(state?.target_ids ?? []).map((targetID) => (
                  <button
                    key={targetID}
                    className="choice"
                    onClick={() => client.send(C2S.SubmitVote, { drawing_id: "mafia_nominate", choice_id: targetID })}
                  >
                    {nameFor(g, targetID)}
                  </button>
                ))}
              </div>
            </>
          ) : (
            <p className="muted">You are out of the game, so you do not nominate.</p>
          )}
          <AliveList />
        </div>
      </div>
    );
  }

  if (g.phase.startsWith("mafia_day_vote")) {
    const canVote = Boolean(me && isAlive(state?.alive_players, me.id));
    return (
      <div className="stack">
        <div className="card">
          <div className="row" style={{ justifyContent: "space-between" }}>
            <h2>Day vote</h2>
            <Countdown deadlineMs={g.deadlineMs} paused={g.paused} remainingMs={g.pauseRemainingMs} />
          </div>
          <p className="muted">{state?.note ?? "Vote to eliminate one player."}</p>
          <RoleGuide role={state?.your_role} phase="vote" />
          {canVote ? (
            <>
              <p className="muted">{state?.locked_in ? "Vote locked in. You can still change it before time ends." : "Pick one suspect."}</p>
              <div className="grid">
                {(state?.target_ids ?? []).map((targetID) => (
                  <button
                    key={targetID}
                    className="choice"
                    onClick={() => client.send(C2S.SubmitVote, { drawing_id: "mafia_day", choice_id: targetID })}
                  >
                    {nameFor(g, targetID)}
                  </button>
                ))}
              </div>
            </>
          ) : (
            <p className="muted">You are out of the game, so you do not vote.</p>
          )}
          <AliveList />
        </div>
      </div>
    );
  }

  if (g.phase.startsWith("mafia_day_revote")) {
    const canVote = Boolean(state?.can_act);
    return (
      <div className="stack">
        <div className="card">
          <div className="row" style={{ justifyContent: "space-between" }}>
            <h2>Revote</h2>
            <Countdown deadlineMs={g.deadlineMs} paused={g.paused} remainingMs={g.pauseRemainingMs} />
          </div>
          <p className="muted">{state?.note ?? "Resolve the tie before the day ends."}</p>
          {canVote ? (
            <>
              <p className="muted">
                {state?.locked_in ? "Choice locked in. You can still change it before time ends." : "Pick one tied suspect."}
              </p>
              <div className="grid">
                {(state?.target_ids ?? []).map((targetID) => (
                  <button
                    key={targetID}
                    className="choice"
                    onClick={() => client.send(C2S.SubmitVote, { drawing_id: "mafia_revote", choice_id: targetID })}
                  >
                    {nameFor(g, targetID)}
                  </button>
                ))}
              </div>
            </>
          ) : (
            <p className="muted">Wait for the tied vote to resolve.</p>
          )}
          <AliveList />
        </div>
      </div>
    );
  }

  if (g.phase.startsWith("mafia_day_reveal")) {
    return (
      <div className="stack">
        <div className="card">
          <h2>Vote reveal</h2>
          <p className="muted">
            {reveal?.eliminated_name
              ? `${reveal.eliminated_name} was eliminated${reveal.eliminated_role ? ` and revealed as ${formatRole(reveal.eliminated_role)}` : ", but their role stayed hidden"}.`
              : "The room tied its votes, so nobody was eliminated."}
          </p>
          {reveal?.winner && <p className="muted">{winnerLine(reveal.winner)}</p>}
          {reveal?.role_map && (
            <div className="stack">
              <h3>Final roles</h3>
              <div className="grid">
                {Object.entries(reveal.role_map).map(([playerID, role]) => (
                  <div key={playerID} className="choice">
                    <strong>{nameFor(g, playerID)}</strong>
                    <div className="muted">{formatRole(role)}</div>
                  </div>
                ))}
              </div>
            </div>
          )}
          <AliveList />
        </div>
      </div>
    );
  }

  return <div className="card">Preparing Mafia...</div>;
}

function RoleGuide({
  role,
  phase,
}: {
  role: string | undefined;
  phase: "role" | "night" | "day" | "nominate" | "vote";
}): JSX.Element {
  const guide = guideForRole(role);
  return (
    <div className="card" style={{ background: "#101a24", marginTop: 12 }} aria-label="Role instructions">
      <h3>How to play {guide.name}</h3>
      <p className="muted">{guide.goal}</p>
      <div className="grid">
        <div className="choice">
          <strong>Night</strong>
          <div className="muted">{guide.night}</div>
        </div>
        <div className="choice">
          <strong>Day</strong>
          <div className="muted">{guide.day}</div>
        </div>
        <div className="choice">
          <strong>{phaseLabel(phase)}</strong>
          <div className="muted">{guide.tip}</div>
        </div>
      </div>
    </div>
  );
}

function AliveList(): JSX.Element {
  const g = useGameState();
  const alivePlayers = (g.mafiaState?.alive_players ?? []).length
    ? g.mafiaState?.alive_players ?? []
    : g.players.map((player) => ({ player_id: player.id, name: player.name, alive: true }));
  return (
    <div className="card" style={{ background: "#0f1720" }}>
      <h3>Players</h3>
      <div className="grid">
        {alivePlayers.map((player) => (
          <div key={player.player_id} className="row" style={{ justifyContent: "space-between" }}>
            <span>{player.name}</span>
            <span className={`pill ${player.alive ? "ready" : "offline"}`}>{player.alive ? "alive" : "out"}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function nameFor(g: ReturnType<typeof useGameState>, playerID: string): string {
  return g.players.find((player) => player.id === playerID)?.name ?? playerID;
}

function namesFor(g: ReturnType<typeof useGameState>, playerIDs: string[]): string[] {
  return playerIDs.map((playerID) => nameFor(g, playerID));
}

function guideForRole(role: string | undefined): {
  name: string;
  goal: string;
  night: string;
  day: string;
  tip: string;
} {
  switch (role) {
    case "mafia":
      return {
        name: "Mafia",
        goal: "Win when Mafia equals or outnumbers the town.",
        night: "Choose one living non-Mafia player to eliminate. Coordinate with your Mafia teammate when there is more than one.",
        day: "Blend in, redirect suspicion, nominate town players, and avoid being voted out.",
        tip: "Act like a helpful town member. Push believable suspicions instead of defending too hard.",
      };
    case "detective":
      return {
        name: "Detective",
        goal: "Help the town find every Mafia member.",
        night: "Investigate one living player. You learn whether they are Mafia or Town, or their exact role if that setting is enabled.",
        day: "Use your information carefully. Reveal too early and Mafia may target you; wait too long and the town may vote wrong.",
        tip: "Track your results privately and guide nominations without immediately exposing yourself.",
      };
    case "doctor":
      return {
        name: "Doctor",
        goal: "Keep town power roles and trusted players alive.",
        night: "Protect one living player. If Mafia attacks that player, the kill fails. Self-protect depends on the room setting.",
        day: "Help the town reason through deaths and saves without making yourself an obvious night target.",
        tip: "Protect likely Mafia targets, not just the loudest player. A successful save can swing the game.",
      };
    case "mayor":
      return {
        name: "Mayor",
        goal: "Use your voting power to help town eliminate Mafia.",
        night: "You sleep at night and do not take a private action.",
        day: "Your vote counts as three. In mayor-breaks-tie mode, you may be the only player who can resolve a tied vote.",
        tip: "Stay alive, listen carefully, and use your weighted vote when the room is split.",
      };
    case "citizen":
      return {
        name: "Citizen",
        goal: "Find and vote out the Mafia using discussion and voting.",
        night: "You sleep at night and receive no private information.",
        day: "Ask questions, compare stories, nominate suspicious players, and vote with the town.",
        tip: "You have no power role, so your strength is reading behavior and protecting confirmed town players.",
      };
    default:
      return {
        name: "your role",
        goal: "Learn your role and help your team win.",
        night: "Follow the prompt shown during the night phase.",
        day: "Discuss, nominate, and vote based on the information you have.",
        tip: "Keep your role private unless revealing it helps your team.",
      };
  }
}

function phaseLabel(phase: string): string {
  switch (phase) {
    case "night":
      return "Right now";
    case "day":
      return "Discussion tip";
    case "nominate":
      return "Nomination tip";
    case "vote":
      return "Voting tip";
    default:
      return "Key tip";
  }
}

function isAlive(players: Array<{ player_id: string; alive: boolean }> | undefined, playerID: string): boolean {
  return players?.find((player) => player.player_id === playerID)?.alive ?? true;
}

function formatRole(role: string | undefined): string {
  switch (role) {
    case "mafia":
      return "Mafia";
    case "detective":
      return "Detective";
    case "doctor":
      return "Doctor";
    case "mayor":
      return "Mayor";
    case "citizen":
      return "Citizen";
    default:
      return "...";
  }
}

function winnerLine(winner: string): string {
  return winner === "mafia" ? "Mafia takes the game." : "The town wins the game.";
}
