// Voting screen: shown during `voting_rN`. For each drawing that isn't ours,
// pick the true prompt from the list of merged choices. Server enforces the
// self-vote / self-fake-vote constraints; the UI just hides drawings the
// server wouldn't accept a vote for.
//
// The payload does carry `is_true` on each choice (the server uses it to
// finalize scoring), but this screen MUST NOT surface that fact — otherwise
// voting is trivial. We render the text only, and shuffle the order so TRUE
// isn't always last.

import { useEffect, useMemo, useState } from "react";
import Countdown from "../components/Countdown";
import DrawingView from "../components/DrawingView";
import { C2S, type DrawingSummary, type VotingChoice } from "../proto";
import { client, useGameState } from "../store";
import { shuffleStable } from "../util/shuffle";

type BallotEntry = {
  drawing: DrawingSummary;
  choices: readonly VotingChoice[];
};

export default function Voting(): JSX.Element {
  const g = useGameState();

  // Drawings we can vote on: not our own, and present in both `drawings` and
  // `votingChoices`. Use `votingChoices` keys as the driver since they're the
  // ballot source of truth.
  const entries: BallotEntry[] = useMemo(() => {
    const out: BallotEntry[] = [];
    for (const vc of Object.values(g.votingChoices)) {
      const drawing = g.drawings[vc.drawing_id];
      if (!drawing) continue;
      if (drawing.author_id === g.playerId) continue;
      // Deterministic per-drawing shuffle so "TRUE" isn't always last.
      out.push({ drawing, choices: shuffleStable(vc.choices, vc.drawing_id) });
    }
    return out;
  }, [g.drawings, g.votingChoices, g.playerId]);

  const [votes, setVotes] = useState<Record<string, string>>({});
  const [pendingMsgIds, setPendingMsgIds] = useState<Record<string, string>>({});
  const [errors, setErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    const msgId = g.lastError?.for_msg_id;
    if (!msgId) return;
    const failed = Object.entries(pendingMsgIds).find(([, pending]) => pending === msgId);
    if (!failed) return;
    const [drawingId] = failed;
    setPendingMsgIds((prev) => {
      const next = { ...prev };
      delete next[drawingId];
      return next;
    });
    setVotes((prev) => {
      const next = { ...prev };
      delete next[drawingId];
      return next;
    });
    setErrors((prev) => ({
      ...prev,
      [drawingId]: g.lastError?.message ?? "Vote rejected. Pick a different choice.",
    }));
  }, [g.lastError, pendingMsgIds]);

  function castVote(drawingId: string, choiceId: string): void {
    if (votes[drawingId]) return; // already locked in; server would reject anyway
    const id = client.send(C2S.SubmitVote, { drawing_id: drawingId, choice_id: choiceId });
    if (id == null) {
      setErrors((e) => ({ ...e, [drawingId]: "Not connected — reconnecting…" }));
      return;
    }
    setErrors((e) => ({ ...e, [drawingId]: "" }));
    setVotes((v) => ({ ...v, [drawingId]: choiceId }));
    setPendingMsgIds((p) => ({ ...p, [drawingId]: id }));
  }

  const remaining = entries.filter((e) => !votes[e.drawing.drawing_id]).length;

  return (
    <div className="stack">
      <div className="card">
        <div className="row" style={{ justifyContent: "space-between" }}>
          <h2>Vote</h2>
          <Countdown deadlineMs={g.deadlineMs} paused={g.paused} remainingMs={g.pauseRemainingMs} />
        </div>
        <p className="muted">
          {entries.length === 0
            ? "Nothing to vote on yet — hold tight."
            : `${remaining} of ${entries.length} left. Pick the real prompt.`}
        </p>
      </div>

      {entries.map(({ drawing, choices }) => {
        const picked = votes[drawing.drawing_id];
        const err = errors[drawing.drawing_id];
        return (
          <div key={drawing.drawing_id} className="card">
            <DrawingView data={drawing.data} format={drawing.format} size={260} />
            <div className="stack" style={{ marginTop: 12 }}>
              {choices.map((c) => {
                const isPicked = picked === c.choice_id;
                // We intentionally do NOT surface c.is_true here — that would
                // spoil the vote. Server still uses it internally for scoring.
                const disabled = !!picked;
                return (
                  <button
                    key={c.choice_id}
                    className={isPicked ? "primary" : ""}
                    disabled={disabled}
                    onClick={() => castVote(drawing.drawing_id, c.choice_id)}
                    style={{ textAlign: "left" }}
                  >
                    {c.text}
                    {isPicked ? " ✓" : ""}
                  </button>
                );
              })}
              {err && <span className="error">{err}</span>}
            </div>
          </div>
        );
      })}
    </div>
  );
}

