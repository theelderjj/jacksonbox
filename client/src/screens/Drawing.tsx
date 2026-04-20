// Drawing screen: shown during `drawing_submit_rN`. Displays the player's
// private prompt (pushed via `prompt_issued`), the shared Canvas, a deadline
// countdown, and a submit button. The button is the only escape — no partial
// saves. Once submitted, the button flips to "Submitted" and we wait for the
// phase to advance.

import { useRef, useState } from "react";
import Canvas, { type CanvasHandle } from "../components/Canvas";
import Countdown from "../components/Countdown";
import { C2S } from "../proto";
import { client, useGameState } from "../store";

const DRAWING_COLORS = [
  { label: "Black", value: "#0b0f14" },
  { label: "Red", value: "#ef4444" },
  { label: "Orange", value: "#f97316" },
  { label: "Yellow", value: "#eab308" },
  { label: "Green", value: "#22c55e" },
  { label: "Blue", value: "#38bdf8" },
  { label: "Indigo", value: "#6366f1" },
  { label: "Pink", value: "#ec4899" },
];

export default function DrawingScreen(): JSX.Element {
  const g = useGameState();
  const canvasRef = useRef<CanvasHandle>(null);
  const [submitted, setSubmitted] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [selectedColor, setSelectedColor] = useState(DRAWING_COLORS[0].value);
  const [rerolled, setRerolled] = useState(false);

  function submit(): void {
    const handle = canvasRef.current;
    if (!handle) return;
    const strokes = handle.getStrokes();
    if (strokes.strokes.length === 0) {
      setErr("Draw something first — even one stroke is fine.");
      return;
    }
    setErr(null);
    setBusy(true);
    const id = client.send(C2S.SubmitDrawing, {
      format: "strokes",
      data: JSON.stringify(strokes),
    });
    if (id == null) {
      setBusy(false);
      setErr("Not connected — reconnecting…");
      return;
    }
    setSubmitted(true);
    // The actual state transition is driven by the server's phase_change.
    // Clear busy after a beat so the button isn't stuck if the round runs long.
    window.setTimeout(() => setBusy(false), 250);
  }

  function rerollPrompt(): void {
    if (rerolled || submitted || !g.myPrompt) return;
    const id = client.send(C2S.RerollPrompt, {});
    if (id == null) {
      setErr("Not connected - reconnecting...");
      return;
    }
    setErr(null);
    setRerolled(true);
    canvasRef.current?.clear();
  }

  return (
    <div className="stack">
      <div className="card">
        <div className="row" style={{ justifyContent: "space-between" }}>
          <h2>Draw this</h2>
          <Countdown deadlineMs={g.deadlineMs} paused={g.paused} remainingMs={g.pauseRemainingMs} />
        </div>
        <p className="muted">Round {g.round}. You have one shot — make it obvious or make it weird.</p>
        <div className="card" style={{ background: "#0f1720", margin: "8px 0 16px" }}>
          <strong style={{ fontSize: 20 }}>{g.myPrompt ?? "…waiting for your prompt"}</strong>
        </div>
        <div className="row" style={{ marginBottom: 12 }}>
          <button type="button" disabled={submitted || rerolled || !g.myPrompt} onClick={rerollPrompt}>
            {rerolled ? "Prompt rerolled" : "Reroll prompt"}
          </button>
          <span className="muted">One reroll per round. Rerolling clears your canvas.</span>
        </div>
        <div className="drawing-tools" aria-label="Drawing tools">
          <span className="muted">Color</span>
          <div className="color-palette">
            {DRAWING_COLORS.map((color) => (
              <button
                key={color.value}
                type="button"
                className={`color-swatch${selectedColor === color.value ? " selected" : ""}`}
                style={{ background: color.value }}
                aria-label={`Use ${color.label}`}
                aria-pressed={selectedColor === color.value}
                disabled={submitted}
                onClick={() => setSelectedColor(color.value)}
              />
            ))}
          </div>
        </div>
        <Canvas ref={canvasRef} disabled={g.pencilsDown} color={selectedColor} />
      </div>

      {err && <div className="banner err">{err}</div>}

      <div className="row">
        <button
          className="primary"
          disabled={busy || submitted || !g.myPrompt}
          onClick={submit}
        >
          {submitted ? "Submitted — waiting" : "Submit drawing"}
        </button>
        <button onClick={() => canvasRef.current?.clear()} disabled={submitted}>
          Clear
        </button>
      </div>
    </div>
  );
}
