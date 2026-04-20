import { useMemo, type CSSProperties } from "react";
import DrawingView from "../components/DrawingView";
import type { RevealAward, RevealChoice, RevealPayload } from "../proto";
import { useGameState } from "../store";

export default function LeaderboardRound(): JSX.Element {
  const g = useGameState();

  const nameOf = useMemo(() => {
    const map = new Map<string, string>();
    for (const p of g.players) map.set(p.id, p.name);
    return (id: string): string => map.get(id) ?? id.slice(0, 6);
  }, [g.players]);

  const reveals = g.reveals;
  const runningDeltas = useMemo(() => {
    const acc: Record<string, number> = {};
    for (const r of reveals) {
      const f = g.revealFinals[r.drawing_id];
      if (!f) continue;
      for (const [pid, pts] of Object.entries(f.deltas)) {
        acc[pid] = (acc[pid] ?? 0) + pts;
      }
    }
    return acc;
  }, [reveals, g.revealFinals]);

  const allFinal = reveals.length > 0 && reveals.every((r) => g.revealFinals[r.drawing_id]);
  const activeReveal =
    reveals.find((r) => r.drawing_id === g.revealCurrentDrawing) ??
    reveals.find((r) => !g.revealFinals[r.drawing_id]) ??
    reveals[0];
  const activeIndex = activeReveal
    ? reveals.findIndex((r) => r.drawing_id === activeReveal.drawing_id)
    : -1;

  return (
    <div className="stack">
      <div className="card">
        <h2>Round {g.round} · Results</h2>
        <p className="muted">
          {allFinal
            ? "Every reveal is complete. The leader can start the next round when the room is ready."
            : "One card at a time. The leader controls each reveal step."}
        </p>
        {activeReveal && (
          <div className="pill" style={{ marginTop: 8 }}>
            Reveal {activeIndex + 1} of {reveals.length}
          </div>
        )}
      </div>

      {activeReveal ? (
        <DrawingCard
          reveal={activeReveal}
          eliminated={g.revealEliminated[activeReveal.drawing_id] ?? []}
          finals={g.revealFinals[activeReveal.drawing_id]}
          nameOf={nameOf}
        />
      ) : (
        <div className="card">
          <p className="muted">Waiting for reveal data...</p>
        </div>
      )}

      {allFinal && (
        <Scoreboard
          players={g.players}
          scores={g.scores}
          runningDeltas={runningDeltas}
          playerId={g.playerId}
        />
      )}
    </div>
  );
}

function DrawingCard({
  reveal,
  eliminated,
  finals,
  nameOf,
}: {
  reveal: RevealPayload;
  eliminated: string[];
  finals: { deltas: Record<string, number>; awards: RevealAward[] } | undefined;
  nameOf: (id: string) => string;
}): JSX.Element {
  return (
    <div className="card" style={{ border: `2px solid ${finals ? "#3a4" : "#5af"}` }}>
      <div className="stack" style={{ gap: 16 }}>
        <DrawingThumb drawingId={reveal.drawing_id} />
        <div style={{ minWidth: 0 }}>
          <div className="muted" style={{ marginBottom: 8 }}>
            Drawn by <strong>{nameOf(reveal.author_id)}</strong>
            {!finals && <span className="pill" style={{ marginLeft: 8 }}>revealing</span>}
            {finals && <span className="pill ready" style={{ marginLeft: 8 }}>revealed</span>}
          </div>
          {reveal.choices.map((c) => (
            <ChoiceRow
              key={c.choice_id}
              choice={c}
              eliminated={eliminated.includes(c.choice_id)}
              allFinalized={!!finals}
              nameOf={nameOf}
            />
          ))}
          {finals && <AwardsBlock awards={finals.awards} nameOf={nameOf} />}
        </div>
      </div>
    </div>
  );
}

function DrawingThumb({ drawingId }: { drawingId: string }): JSX.Element | null {
  const g = useGameState();
  const d = g.drawings[drawingId];
  if (!d) return null;
  return <DrawingView data={d.data} format={d.format} size={300} />;
}

function ChoiceRow({
  choice,
  eliminated,
  allFinalized,
  nameOf,
}: {
  choice: RevealChoice;
  eliminated: boolean;
  allFinalized: boolean;
  nameOf: (id: string) => string;
}): JSX.Element {
  const showTruth = allFinalized && choice.is_true;
  const className = showTruth ? "choice true" : "choice";
  const style: CSSProperties = eliminated
    ? { opacity: 0.4, textDecoration: "line-through" }
    : {};

  return (
    <div className={className} style={style}>
      <div>
        <div>{choice.text}</div>
        <div className="muted" style={{ fontSize: 12, marginTop: 4 }}>
          {showTruth
            ? "TRUE PROMPT"
            : choice.author_ids && choice.author_ids.length > 0
            ? `by ${choice.author_ids.map(nameOf).join(", ")}`
            : eliminated
            ? "eliminated"
            : "-"}
        </div>
      </div>
      <div className="muted" style={{ fontSize: 12, textAlign: "right" }}>
        {choice.voters.length === 0 ? "-" : choice.voters.map(nameOf).join(", ")}
      </div>
    </div>
  );
}

function AwardsBlock({
  awards,
  nameOf,
}: {
  awards: RevealAward[];
  nameOf: (id: string) => string;
}): JSX.Element | null {
  if (awards.length === 0) return null;
  return (
    <div className="stack" style={{ marginTop: 12, gap: 4 }}>
      <div className="muted" style={{ fontSize: 12 }}>Points this drawing:</div>
      {awards.map((a, i) => (
        <div key={i} className="score-row">
          <span>
            {nameOf(a.player_id)} <span className="muted" style={{ fontSize: 12 }}>({reasonLabel(a.reason)})</span>
          </span>
          <span className="ok" style={{ fontVariantNumeric: "tabular-nums" }}>
            +{a.points}
          </span>
        </div>
      ))}
    </div>
  );
}

function reasonLabel(reason: string): string {
  switch (reason) {
    case "drawer_truth":
      return "drawer - guessed truth";
    case "guesser_truth":
      return "guessed the truth";
    case "faker_fool":
      return "fooled a guesser";
    default:
      return reason;
  }
}

function Scoreboard({
  players,
  scores,
  runningDeltas,
  playerId,
}: {
  players: { id: string; name: string; connected: boolean }[];
  scores: Record<string, number>;
  runningDeltas: Record<string, number>;
  playerId: string | null;
}): JSX.Element {
  const rows = useMemo(() => {
    return players
      .map((p) => ({
        id: p.id,
        name: p.name,
        connected: p.connected,
        score: scores[p.id] ?? 0,
        delta: runningDeltas[p.id] ?? 0,
      }))
      .sort((a, b) => {
        if (b.score !== a.score) return b.score - a.score;
        return a.name.localeCompare(b.name);
      });
  }, [players, scores, runningDeltas]);

  return (
    <div className="card">
      <h3>Scoreboard</h3>
      {rows.map((r, i) => (
        <div key={r.id} className="score-row">
          <span>
            {i + 1}. {r.name}
            {r.id === playerId ? " (you)" : ""}
            {!r.connected && <span className="pill offline" style={{ marginLeft: 8 }}>offline</span>}
          </span>
          <span style={{ fontVariantNumeric: "tabular-nums" }}>
            {r.score}
            {r.delta > 0 && <span className="ok" style={{ marginLeft: 8, fontSize: 12 }}>+{r.delta}</span>}
          </span>
        </div>
      ))}
    </div>
  );
}
