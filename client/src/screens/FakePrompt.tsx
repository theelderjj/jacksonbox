// FakePrompt screen: shown during `fake_prompt_submit_rN`. For each drawing
// that isn't ours, we type a plausible-looking prompt. The server enforces
// uniqueness vs. the true prompt and merges collisions across players.
//
// Trade-offs:
//   - Local submittedIds set is a UI concern — the server is still the source
//     of truth and will reject a second submission for the same drawing.
//   - We render one card per drawing and submit them independently so a slow
//     typer doesn't block others on this device (phones tend to have one
//     player, but keep the shape right).

import { useMemo, useState } from "react";
import Countdown from "../components/Countdown";
import DrawingView from "../components/DrawingView";
import { C2S, type DrawingSummary } from "../proto";
import { client, useGameState } from "../store";

export default function FakePrompt(): JSX.Element {
  const g = useGameState();

  // Filter out our own drawing — we can't fake for it (server will reject).
  const targets: DrawingSummary[] = useMemo(() => {
    return Object.values(g.drawings).filter((d) => d.author_id !== g.playerId);
  }, [g.drawings, g.playerId]);

  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [submitted, setSubmitted] = useState<Set<string>>(new Set());
  const [errors, setErrors] = useState<Record<string, string>>({});

  function submit(drawingId: string): void {
    const text = (drafts[drawingId] ?? "").trim();
    if (text.length === 0) {
      setErrors((e) => ({ ...e, [drawingId]: "Type something first." }));
      return;
    }
    if (text.length > 80) {
      setErrors((e) => ({ ...e, [drawingId]: "80 characters max." }));
      return;
    }
    const id = client.send(C2S.SubmitFakePrompt, { drawing_id: drawingId, text });
    if (id == null) {
      setErrors((e) => ({ ...e, [drawingId]: "Not connected — reconnecting…" }));
      return;
    }
    setErrors((e) => ({ ...e, [drawingId]: "" }));
    setSubmitted((s) => new Set([...s, drawingId]));
  }

  const remaining = targets.filter((d) => !submitted.has(d.drawing_id)).length;

  return (
    <div className="stack">
      <div className="card">
        <div className="row" style={{ justifyContent: "space-between" }}>
          <h2>Fake a prompt</h2>
          <Countdown deadlineMs={g.deadlineMs} paused={g.paused} remainingMs={g.pauseRemainingMs} />
        </div>
        <p className="muted">
          {targets.length === 0
            ? "No drawings to fake yet — hang tight."
            : `${remaining} of ${targets.length} left. Make it sound like the real answer.`}
        </p>
      </div>

      <div className="grid">
        {targets.map((d) => {
          const done = submitted.has(d.drawing_id);
          const err = errors[d.drawing_id];
          return (
            <div key={d.drawing_id} className="card">
              <DrawingView data={d.data} format={d.format} size={220} />
              {done ? (
                <p className="ok" style={{ marginTop: 12 }}>Submitted — waiting on others</p>
              ) : (
                <div className="stack" style={{ marginTop: 12 }}>
                  <input
                    type="text"
                    placeholder="What could this be?"
                    value={drafts[d.drawing_id] ?? ""}
                    maxLength={80}
                    onChange={(e) =>
                      setDrafts((ds) => ({ ...ds, [d.drawing_id]: e.target.value }))
                    }
                  />
                  {err && <span className="error">{err}</span>}
                  <button className="primary" onClick={() => submit(d.drawing_id)}>
                    Submit fake
                  </button>
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
