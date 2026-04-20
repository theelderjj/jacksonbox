// Reveal screen: shown during `reveal_rN` and `scoring_rN`. One card per
// drawing, showing the drawing, each choice with who voted for it and
// (critically) who authored each fake — the reveal is when fakery gets paid
// off. Round deltas, if present, render at the bottom.
//
// There's no user input here; this is pure read-out.

import { useMemo } from "react";
import DrawingView from "../components/DrawingView";
import { useGameState } from "../store";

export default function Reveal(): JSX.Element {
  const g = useGameState();

  const nameOf = useMemo(() => {
    const map = new Map<string, string>();
    for (const p of g.players) map.set(p.id, p.name);
    return (id: string): string => map.get(id) ?? id.slice(0, 6);
  }, [g.players]);

  // Stable reveal ordering so scoring updates don't reshuffle cards mid-round.
  const reveals = g.reveals;

  return (
    <div className="stack">
      <div className="card">
        <h2>Reveal</h2>
        <p className="muted">
          Round {g.round}. The true prompt is green. Votes and authorship show below each choice.
        </p>
      </div>

      {reveals.map((r) => {
        const drawing = g.drawings[r.drawing_id];
        return (
          <div key={r.drawing_id} className="card">
            <div className="row" style={{ alignItems: "flex-start", gap: 16 }}>
              {drawing && <DrawingView data={drawing.data} format={drawing.format} size={220} />}
              <div style={{ flex: 1, minWidth: 220 }}>
                <div className="muted" style={{ marginBottom: 8 }}>
                  Drawn by <strong>{nameOf(r.author_id)}</strong>
                </div>
                {r.choices.map((c) => (
                  <div key={c.choice_id} className={`choice${c.is_true ? " true" : ""}`}>
                    <div>
                      <div>{c.text}</div>
                      <div className="muted" style={{ fontSize: 12, marginTop: 4 }}>
                        {c.is_true
                          ? "TRUE PROMPT"
                          : c.author_ids && c.author_ids.length > 0
                          ? `by ${c.author_ids.map(nameOf).join(", ")}`
                          : "default"}
                      </div>
                    </div>
                    <div className="muted" style={{ fontSize: 12, textAlign: "right" }}>
                      {c.voters.length === 0
                        ? "—"
                        : c.voters.map(nameOf).join(", ")}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        );
      })}

      {g.lastDeltas && (
        <div className="card">
          <h3>Round scores</h3>
          {Object.entries(g.lastDeltas)
            .sort((a, b) => b[1] - a[1])
            .map(([pid, delta]) => (
              <div key={pid} className="score-row">
                <span>{nameOf(pid)}</span>
                <span className={delta > 0 ? "ok" : "muted"}>
                  {delta > 0 ? "+" : ""}
                  {delta}
                </span>
              </div>
            ))}
        </div>
      )}
    </div>
  );
}
