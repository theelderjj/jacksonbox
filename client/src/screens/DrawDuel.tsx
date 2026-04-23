import { useEffect, useMemo, useRef, useState } from "react";
import Canvas, { type CanvasHandle } from "../components/Canvas";
import Countdown from "../components/Countdown";
import DrawingView from "../components/DrawingView";
import { C2S } from "../proto";
import { selfPlayer, client, useGameState } from "../store";

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

const LINE_WIDTHS = [
  { label: "Thin", value: 3 },
  { label: "Medium", value: 5 },
  { label: "Bold", value: 8 },
  { label: "Chunky", value: 12 },
];

export default function DrawDuel(): JSX.Element {
  const g = useGameState();
  const me = selfPlayer(g);
  const canvasRef = useRef<CanvasHandle>(null);
  const [submitted, setSubmitted] = useState(false);
  const [submittedVote, setSubmittedVote] = useState(false);
  const [selectedColor, setSelectedColor] = useState(DRAWING_COLORS[0]!.value);
  const [selectedWidth, setSelectedWidth] = useState(LINE_WIDTHS[1]!.value);

  const round = g.drawDuelRound;
  const isArtistA = !!me && me.id === round?.artist_a_id;
  const isArtistB = !!me && me.id === round?.artist_b_id;
  const isArtist = isArtistA || isArtistB;
  const drawings = useMemo(() => Object.values(g.drawings), [g.drawings]);

  useEffect(() => {
    setSubmitted(false);
    setSubmittedVote(false);
  }, [g.phase]);

  if (!round) {
    return <div className="card">Preparing Draw Duel...</div>;
  }

  if (g.phase.startsWith("draw_duel_prompt") || g.phase.startsWith("draw_duel_draw")) {
    return (
      <div className="stack">
        <div className="card">
          <div className="row" style={{ justifyContent: "space-between" }}>
            <h2>Draw Duel</h2>
            <Countdown deadlineMs={g.deadlineMs} paused={g.paused} remainingMs={g.pauseRemainingMs} />
          </div>
          <p className="muted">Round {round.round}. Two artists draw. Everyone else judges.</p>
          <div className="card" style={{ background: "#0f1720", margin: "8px 0 16px" }}>
            <strong style={{ fontSize: 20 }}>{g.myPrompt ?? round.prompt}</strong>
            <div className="muted" style={{ marginTop: 8 }}>
              {round.artist_a_name} vs {round.artist_b_name}
            </div>
          </div>
          {isArtist ? (
            <>
              <div className="drawing-tools" aria-label="Draw duel tools">
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
                <span className="muted">Line weight</span>
                <div className="row wrap">
                  {LINE_WIDTHS.map((width) => (
                    <button
                      key={width.value}
                      type="button"
                      className={selectedWidth === width.value ? "primary" : ""}
                      disabled={submitted}
                      onClick={() => setSelectedWidth(width.value)}
                    >
                      {width.label}
                    </button>
                  ))}
                </div>
              </div>
              <Canvas
                ref={canvasRef}
                disabled={g.pencilsDown}
                color={selectedColor}
                lineWidth={selectedWidth}
              />
            </>
          ) : (
            <div className="card">
              <p className="muted">You are judging this round. Watch the duelists work, then vote for the stronger drawing.</p>
            </div>
          )}
        </div>

        {isArtist ? (
          <div className="row">
            <button
              className="primary"
              disabled={submitted}
              onClick={() => {
                const strokes = canvasRef.current?.getStrokes();
                if (!strokes || strokes.strokes.length === 0) return;
                client.send(C2S.SubmitDrawing, {
                  format: "strokes",
                  data: JSON.stringify(strokes),
                });
                setSubmitted(true);
              }}
            >
              {submitted ? "Drawing submitted" : "Submit drawing"}
            </button>
            <button onClick={() => canvasRef.current?.clear()} disabled={submitted}>
              Clear
            </button>
          </div>
        ) : null}
      </div>
    );
  }

  if (g.phase.startsWith("draw_duel_vote")) {
    return (
      <div className="stack">
        <div className="card">
          <div className="row" style={{ justifyContent: "space-between" }}>
            <h2>Judge the duel</h2>
            <Countdown deadlineMs={g.deadlineMs} paused={g.paused} remainingMs={g.pauseRemainingMs} />
          </div>
          <p className="muted">{round.prompt}</p>
        </div>
        <div className="grid">
          {drawings.map((drawing) => {
            const artistName =
              drawing.author_id === round.artist_a_id ? round.artist_a_name : round.artist_b_name;
            return (
              <div key={drawing.drawing_id} className="card">
                <strong>{artistName}</strong>
                <DrawingView data={drawing.data} format={drawing.format as "strokes" | "png"} />
                <button
                  className="primary"
                  disabled={isArtist || submittedVote}
                  onClick={() => {
                    client.send(C2S.SubmitVote, {
                      drawing_id: "draw_duel",
                      choice_id: drawing.drawing_id,
                    });
                    setSubmittedVote(true);
                  }}
                >
                  {isArtist ? "Artists do not judge" : submittedVote ? "Vote locked" : `Vote for ${artistName}`}
                </button>
              </div>
            );
          })}
        </div>
      </div>
    );
  }

  if (g.phase.startsWith("draw_duel_reveal")) {
    const reveal = g.drawDuelReveal;
    if (!reveal) return <div className="card">Resolving the duel...</div>;
    return (
      <div className="stack">
        <div className="card">
          <h2>Draw Duel Reveal</h2>
          <p className="muted">{reveal.prompt}</p>
          <p className="muted">
            {reveal.tied
              ? "The judges called it a tie."
              : `${reveal.winner_artist_name} wins the duel.`}
          </p>
        </div>
        <div className="grid">
          {reveal.drawings.map((drawing) => {
            const artistName =
              drawing.author_id === reveal.artist_a_id ? reveal.artist_a_name : reveal.artist_b_name;
            const votes = reveal.vote_count_by_drawing[drawing.drawing_id] ?? 0;
            const winner = !reveal.tied && reveal.winner_drawing_id === drawing.drawing_id;
            return (
              <div key={drawing.drawing_id} className={`card${winner ? " selected" : ""}`}>
                <strong>{artistName}</strong>
                <DrawingView data={drawing.data} format={drawing.format as "strokes" | "png"} />
                <div className="muted">{votes} judge vote{votes === 1 ? "" : "s"}</div>
                {winner && <div className="pill ready">Winner</div>}
                {reveal.tied && <div className="pill">Tie</div>}
              </div>
            );
          })}
        </div>
      </div>
    );
  }

  return <div className="card">Preparing Draw Duel...</div>;
}
