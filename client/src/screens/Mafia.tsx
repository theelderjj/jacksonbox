import Countdown from "../components/Countdown";
import { getMafiaRoleGuide } from "../components/gameGuides";
import { C2S } from "../proto";
import { client, selfPlayer, useGameState } from "../store";

const ABSTAIN_CHOICE_ID = "__abstain__";

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
    const canVote = Boolean(state?.can_act);
    return (
      <div className="stack">
        <div className="card">
          <div className="row" style={{ justifyContent: "space-between" }}>
            <h2>Day vote</h2>
            <Countdown deadlineMs={g.deadlineMs} paused={g.paused} remainingMs={g.pauseRemainingMs} />
          </div>
          <p className="muted">{state?.note ?? "Vote to eliminate one player."}</p>
          <RoleGuide role={state?.your_role} phase="vote" />
          <VoteLedger title="Current vote table" />
          {canVote ? (
            <>
              <p className="muted">{voteStatusLine(g, state)}</p>
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
                <button
                  className="choice"
                  onClick={() => client.send(C2S.SubmitVote, { drawing_id: "mafia_day", choice_id: ABSTAIN_CHOICE_ID })}
                >
                  Abstain
                </button>
              </div>
            </>
          ) : (
            <p className="muted">{blockedVoteLine(g, state, me?.id ?? null)}</p>
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
          <VoteLedger title="Current revote table" />
          {canVote ? (
            <>
              <p className="muted">{voteStatusLine(g, state)}</p>
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
                <button
                  className="choice"
                  onClick={() => client.send(C2S.SubmitVote, { drawing_id: "mafia_revote", choice_id: ABSTAIN_CHOICE_ID })}
                >
                  Abstain
                </button>
              </div>
            </>
          ) : (
            <p className="muted">{blockedVoteLine(g, state, me?.id ?? null)}</p>
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
          {reveal?.votes && Object.keys(reveal.votes).length > 0 && (
            <div className="card" style={{ background: "#0f1720" }}>
              <h3>Who voted for whom</h3>
              <div className="grid">
                {Object.entries(reveal.votes).map(([voterID, targetID]) => (
                  <div key={voterID} className="choice">
                    <strong>{nameFor(g, voterID)}</strong>
                    <div className="muted">{voteLabel(g, targetID)}</div>
                  </div>
                ))}
              </div>
            </div>
          )}
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

function VoteLedger({ title }: { title: string }): JSX.Element | null {
  const g = useGameState();
  const state = g.mafiaState;
  const votes = state?.public_votes ?? {};
  const shouldShow =
    state?.vote_mode === "live_public" ||
    state?.vote_mode === "sequential_public" ||
    (state?.phase === "day_vote" && Object.keys(votes).length > 0) ||
    (state?.phase === "day_revote" && Object.keys(votes).length > 0);
  if (!shouldShow) return null;

  return (
    <div className="card" style={{ background: "#0f1720" }}>
      <h3>{title}</h3>
      <div className="grid">
        {(state?.alive_players ?? []).filter((player) => player.alive).map((player) => {
          const targetID = votes[player.player_id];
          const isCurrent = state?.current_voter_id === player.player_id;
          return (
            <div key={player.player_id} className="choice">
              <strong>
                {player.name}
                {isCurrent ? " (voting now)" : ""}
              </strong>
              <div className="muted">
                {targetID
                  ? voteLabel(g, targetID)
                  : state?.vote_mode === "sequential_public"
                    ? "waiting to vote"
                    : "no public vote yet"}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function RoleGuide({
  role,
  phase,
}: {
  role: string | undefined;
  phase: "role" | "night" | "day" | "nominate" | "vote";
}): JSX.Element {
  const guide = getMafiaRoleGuide(role);
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

function voteStatusLine(
  g: ReturnType<typeof useGameState>,
  state: ReturnType<typeof useGameState>["mafiaState"],
): string {
  switch (state?.vote_mode) {
    case "live_public":
      return state.locked_in
        ? "Your vote is visible and currently locked in, but you can still switch it before time ends."
        : "Pick a suspect. Everyone can see live vote changes before the timer runs out.";
    case "sequential_public":
      return state.locked_in
        ? "Your public vote is locked in."
        : state.current_voter_id
          ? `It is your turn to vote publicly${state.current_voter_id === g.playerId ? "" : ` after ${nameFor(g, state.current_voter_id)}`}.`
          : "It is your turn to vote publicly.";
    default:
      return state?.locked_in
        ? "Your vote is locked in. You can still change it before time runs out."
        : "Pick one suspect. Votes stay hidden until the reveal.";
  }
}

function blockedVoteLine(
  g: ReturnType<typeof useGameState>,
  state: ReturnType<typeof useGameState>["mafiaState"],
  playerID: string | null,
): string {
  if (!playerID || !isAlive(state?.alive_players, playerID)) {
    return "You are out of the game, so you do not vote.";
  }
  if (state?.vote_mode === "sequential_public" && state.current_voter_id) {
    return `${nameFor(g, state.current_voter_id)} is currently casting the next public vote.`;
  }
  return "Wait for the vote to resolve.";
}

function voteLabel(g: ReturnType<typeof useGameState>, targetID: string): string {
  if (targetID === ABSTAIN_CHOICE_ID) {
    return "abstained";
  }
  return `voted for ${nameFor(g, targetID)}`;
}
