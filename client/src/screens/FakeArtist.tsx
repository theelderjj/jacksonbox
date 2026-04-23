import { useEffect, useMemo, useRef, useState } from "react";
import Canvas, { type CanvasHandle } from "../components/Canvas";
import Countdown from "../components/Countdown";
import DrawingView from "../components/DrawingView";
import { C2S, type StrokesData } from "../proto";
import { client, selfPlayer, useGameState } from "../store";

export default function FakeArtist(): JSX.Element {
  const g = useGameState();
  const me = selfPlayer(g);
  const canvasRef = useRef<CanvasHandle>(null);
  const lastSendRef = useRef(0);
  const [guess, setGuess] = useState("");
  const [replayFrame, setReplayFrame] = useState(0);
  const [replayLoops, setReplayLoops] = useState(0);
  const [submittedVote, setSubmittedVote] = useState(false);

  const turn = g.fakeArtistTurn;
  const isCurrentDrawer = !!me && !!turn && turn.drawer_id === me.id && turn.phase === "draw";
  const liveCountdown = useVisibleCountdown(
    g.deadlineMs,
    g.paused,
    g.pauseRemainingMs,
    g.phase.startsWith("fake_artist_countdown"),
  );
  const underlay = parseStrokes(turn?.data ?? g.fakeArtistCanvas?.data ?? "");
  const replaySegments = g.fakeArtistReplay?.segments ?? [];
  const isFakeInstruction = !!g.myPrompt && g.myPrompt.startsWith("You are the fake artist.");
  const visiblePrompt = !isFakeInstruction && !g.myPrompt?.includes("steal the round") ? g.myPrompt : null;
  const replayTurnIndex = replaySegments.length === 0 ? 0 : Math.floor(replayFrame / 4);
  const replayStep = replayFrame % 4;
  const replayPhaseKind = replayStep < 3 ? "countdown" : "draw";
  const replayCountdown = replayPhaseKind === "countdown" ? 3 - replayStep : null;
  const replaySegment = replaySegments[Math.min(replayTurnIndex, Math.max(0, replaySegments.length - 1))] ?? null;
  const replayData = useMemo(() => {
    if (replaySegments.length === 0) return `{"strokes":[]}`;
    const visibleTurns = replayPhaseKind === "draw" ? replayTurnIndex + 1 : replayTurnIndex;
    return mergeStrokeData(replaySegments.slice(0, visibleTurns).map((segment) => segment.data));
  }, [replayPhaseKind, replaySegments, replayTurnIndex]);

  useEffect(() => {
    if (!(g.phase.startsWith("fake_artist_replay") || g.phase.startsWith("fake_artist_vote"))) {
      setReplayFrame(0);
      setReplayLoops(0);
      return;
    }
    if (replaySegments.length === 0) return;
    const timer = window.setInterval(() => {
      setReplayFrame((current) => {
        const next = current + 1;
        const finalFrame = replaySegments.length * 4 - 1;
        if (next > finalFrame) {
          const replayCount = Math.max(1, g.fakeArtistReplay?.replay_count ?? 1);
          if (g.phase.startsWith("fake_artist_replay")) {
            if (g.fakeArtistReplay?.continuous_replay) return 0;
            if (replayLoops + 1 < replayCount) {
              setReplayLoops((loops) => loops + 1);
              return 0;
            }
            return current;
          }
          return g.fakeArtistReplay?.continuous_replay ? 0 : current;
        }
        return next;
      });
    }, 700);
    return () => window.clearInterval(timer);
  }, [g.fakeArtistReplay?.continuous_replay, g.fakeArtistReplay?.replay_count, g.phase, replayLoops, replaySegments.length]);

  useEffect(() => {
    setSubmittedVote(false);
  }, [g.phase]);

  function sendLiveDrawing(strokes: StrokesData): void {
    if (!isCurrentDrawer) return;
    const now = Date.now();
    if (now-lastSendRef.current < 180) return;
    lastSendRef.current = now;
    client.send(C2S.SubmitDrawing, {
      format: "strokes",
      data: JSON.stringify(strokes),
    });
  }

  if (g.phase.startsWith("fake_artist_countdown") || g.phase.startsWith("fake_artist_draw")) {
    return (
      <div className="stack">
        <div className="card">
          <div className="row" style={{ justifyContent: "space-between" }}>
            <h2>Fake Artist</h2>
            <Countdown deadlineMs={g.deadlineMs} paused={g.paused} remainingMs={g.pauseRemainingMs} />
          </div>
          <p className="muted">
            {turn?.phase === "countdown" ? "Get ready for the next drawer." : "Everyone watches the canvas build one turn at a time."}
          </p>
          <div className="card" style={{ background: "#0f1720", marginBottom: 16 }}>
            <strong style={{ fontSize: 20 }} aria-label="Current drawer">{turn?.drawer_name ?? "..."}</strong>
            <div className="muted">{turn?.phase === "countdown" ? "draws next" : "is drawing now"}</div>
          </div>
          {visiblePrompt && <div className="intro-note">{visiblePrompt}</div>}
          {isFakeInstruction && (
            <div className="intro-note">
              You are the fake artist. Watch what everyone adds, copy the style, and avoid standing out.
            </div>
          )}
          {turn?.phase === "countdown" && (
            <div className="card" style={{ background: "#0f1720", textAlign: "center", marginBottom: 16 }}>
              <div className="muted">Drawing starts in</div>
              <strong style={{ fontSize: 56, lineHeight: 1 }}>{liveCountdown}</strong>
            </div>
          )}
          {turn?.phase === "draw" && isCurrentDrawer ? (
            <>
              <Canvas ref={canvasRef} color={turn.color} lineWidth={5} underlay={underlay} onChange={sendLiveDrawing} />
              <div className="row" style={{ marginTop: 12 }}>
                <button onClick={() => canvasRef.current?.clear()}>Clear your turn</button>
              </div>
            </>
          ) : (
            <DrawingView
              data={g.fakeArtistCanvas?.data ?? turn?.data ?? `{"strokes":[]}`}
              format={(g.fakeArtistCanvas?.format ?? turn?.format ?? "strokes") as "strokes" | "png"}
            />
          )}
        </div>
      </div>
    );
  }

  if (g.phase.startsWith("fake_artist_replay")) {
    return (
      <div className="stack">
        <div className="card">
          <h2>Replay the drawing</h2>
          <p className="muted">Watch each turn back before the room decides who faked it.</p>
          {replaySegment && (
            <p className="muted" aria-label="Replay status">
              {replayPhaseKind === "countdown"
                ? `Turn ${replaySegment.turn} of ${replaySegments.length}: ${replaySegment.drawer_name} draws in ${replayCountdown ?? 3}`
                : `Turn ${replaySegment.turn} of ${replaySegments.length}: ${replaySegment.drawer_name} drawing replays now`}
            </p>
          )}
          {replaySegment && replayPhaseKind === "countdown" && (
            <div className="card" style={{ background: "#0f1720", textAlign: "center", marginBottom: 16 }}>
              <div className="muted">{replaySegment.drawer_name} starts in</div>
              <strong style={{ fontSize: 44, lineHeight: 1 }}>{replayCountdown ?? 3}</strong>
            </div>
          )}
          <DrawingView data={replayData} format="strokes" />
        </div>
      </div>
    );
  }

  if (g.phase.startsWith("fake_artist_vote")) {
    return (
      <div className="stack">
        <div className="card">
          <div className="row" style={{ justifyContent: "space-between" }}>
            <h2>Vote for the fake artist</h2>
            <Countdown deadlineMs={g.deadlineMs} paused={g.paused} remainingMs={g.pauseRemainingMs} />
          </div>
          <p className="muted">Use the replay and pick the player who looked like they were guessing.</p>
          {replaySegment && replayPhaseKind === "countdown" && (
            <div className="card" style={{ background: "#0f1720", textAlign: "center", marginBottom: 16 }}>
              <div className="muted">{replaySegment.drawer_name} starts in</div>
              <strong style={{ fontSize: 44, lineHeight: 1 }}>{replayCountdown ?? 3}</strong>
            </div>
          )}
          <DrawingView data={replayData} format="strokes" />
        </div>
        <div className="grid">
          {g.players
            .filter((player) => player.connected)
            .map((player) => (
              <button
                key={player.id}
                className="choice"
                disabled={submittedVote}
                onClick={() => {
                  client.send(C2S.SubmitVote, { drawing_id: "fake_artist", choice_id: player.id });
                  setSubmittedVote(true);
                }}
              >
                {player.name}
              </button>
            ))}
        </div>
      </div>
    );
  }

  if (g.phase.startsWith("fake_artist_guess")) {
    const needsGuess = g.fakeArtistReveal?.majority_caught ?? true;
    const amFake = !!me && !!g.fakeArtistReveal && g.fakeArtistReveal.fake_id === me.id;
    return (
      <div className="stack">
        <div className="card">
          <div className="row" style={{ justifyContent: "space-between" }}>
            <h2>Fake Artist Guess</h2>
            <Countdown deadlineMs={g.deadlineMs} paused={g.paused} remainingMs={g.pauseRemainingMs} />
          </div>
          <p className="muted">
            {amFake ? "The room caught you. Guess the original prompt to steal the round." : "The fake artist gets one last guess."}
          </p>
          {amFake || g.myPrompt?.includes("steal the round") ? (
            <div className="row">
              <input value={guess} onChange={(e) => setGuess(e.target.value)} placeholder="Guess the drawing prompt" />
              <button
                className="primary"
                disabled={guess.trim() === ""}
                onClick={() => client.send(C2S.SubmitFakeArtistGuess, { prompt: guess.trim() })}
              >
                Submit guess
              </button>
            </div>
          ) : (
            <p className="muted">{needsGuess ? "Waiting for the fake artist to guess." : "Round is resolving."}</p>
          )}
        </div>
      </div>
    );
  }

  if (g.phase.startsWith("fake_artist_reveal")) {
    return (
      <div className="stack">
        <div className="card">
          <h2>Fake Artist Reveal</h2>
          <p className="muted">
            Fake artist: {g.fakeArtistReveal?.fake_name ?? "..."}. Prompt: {g.fakeArtistReveal?.prompt ?? "..."}
          </p>
          <p className="muted">
            {g.fakeArtistReveal?.fake_wins
              ? "The fake artist wins the round."
              : "The real artists win the round."}
          </p>
          {g.fakeArtistReveal?.accused_name && (
            <p className="muted">Most votes landed on: {g.fakeArtistReveal.accused_name}</p>
          )}
          {g.fakeArtistReveal?.fake_guess && (
            <p className="muted">
              Fake guess: {g.fakeArtistReveal.fake_guess}{" "}
              {g.fakeArtistReveal.fake_guessed_prompt ? "(correct)" : "(wrong)"}
            </p>
          )}
        </div>
      </div>
    );
  }

  return <div className="card">Preparing Fake Artist…</div>;
}

function parseStrokes(raw: string): StrokesData | null {
  if (!raw) return null;
  try {
    return JSON.parse(raw) as StrokesData;
  } catch {
    return null;
  }
}

function mergeStrokeData(frames: string[]): string {
  const merged: StrokesData = { strokes: [] };
  for (const frame of frames) {
    const parsed = parseStrokes(frame);
    if (!parsed) continue;
    merged.strokes.push(...parsed.strokes);
  }
  return JSON.stringify(merged);
}

function useVisibleCountdown(
  deadlineMs: number | null,
  paused: boolean,
  pauseRemainingMs: number | null,
  enabled: boolean,
): number {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!enabled || paused || deadlineMs === null) return;
    const timer = window.setInterval(() => setNow(Date.now()), 100);
    return () => window.clearInterval(timer);
  }, [deadlineMs, enabled, paused]);

  if (!enabled) return 0;
  if (paused) {
    return Math.max(1, Math.ceil((pauseRemainingMs ?? 0) / 1000));
  }
  if (deadlineMs === null) return 3;
  return Math.max(1, Math.ceil((deadlineMs - now) / 1000));
}
