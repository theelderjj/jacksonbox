import { FormEvent, useEffect, useState } from "react";
import Countdown from "../components/Countdown";
import { C2S, type WordEntryPayload, type WordLetterWindow } from "../proto";
import { client, useGameState } from "../store";

export default function WordStorm(): JSX.Element {
  const g = useGameState();
  const [draft, setDraft] = useState("");
  const [now, setNow] = useState(() => Date.now());
  const prompt = g.wordPrompt;
  const result = g.wordResult;

  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 250);
    return () => window.clearInterval(id);
  }, []);

  if (g.phase.startsWith("word_submit")) {
    const current = currentLetter(prompt?.letters ?? [], prompt?.letter_seconds ?? 20, g.deadlineMs, now);
    const mine = g.wordEntries.filter((entry) => entry.player_id === g.playerId).slice(-8).reverse();
    return (
      <div className="stack">
        <div className="card">
          <div className="row" style={{ justifyContent: "space-between" }}>
            <div>
              <h2>Word Storm</h2>
              <p className="muted">
                Enter as many dictionary words as you can for the current letter. Each word can
                only be used once all game.
              </p>
            </div>
            <Countdown deadlineMs={g.deadlineMs} paused={g.paused} remainingMs={g.pauseRemainingMs} />
          </div>
        </div>

        <div className="card storm-card">
          <div className="pill">Letter changes every {prompt?.letter_seconds ?? 20}s</div>
          <div className="storm-letter">{current.letter.toUpperCase()}</div>
          <p className="muted">
            Scoring happens at the end. Start with {prompt?.base_points_per_letter ?? 100} points
            per letter, then +{prompt?.growth_percent ?? 15}% for every letter beyond 4.
          </p>
          <form className="row wrap" onSubmit={(event) => submitWord(event, draft, setDraft)}>
            <input
              value={draft}
              onChange={(event) => setDraft(event.target.value)}
              placeholder={`Word starting with ${current.letter.toUpperCase()}`}
              autoFocus
              inputMode="text"
              autoCapitalize="none"
            />
            <button className="primary" disabled={normalizeWord(draft) === ""}>
              Enter
            </button>
          </form>
          <div className="word-chip-row">
            {(prompt?.letters ?? []).map((letter) => (
              <span
                key={letter.index}
                className={`pill ${letter.index === current.index ? "ready" : ""}`}
              >
                {letter.letter.toUpperCase()}
              </span>
            ))}
          </div>
        </div>

        <div className="card">
          <h3>Your recent entries</h3>
          <p className="muted">
            These are only tentatively accepted. Official points arrive after dictionary cleanup.
          </p>
          {mine.length === 0 ? (
            <p className="muted">No words entered yet.</p>
          ) : (
            <div className="word-entry-list">
              {mine.map((entry, idx) => (
                <EntryLine key={`${entry.word}-${idx}`} entry={entry} />
              ))}
            </div>
          )}
        </div>
      </div>
    );
  }

  if (g.phase.startsWith("word_reveal")) {
    return (
      <div className="stack">
        <div className="card">
          <h2>Word Storm Results</h2>
          <p className="muted">
            Invalid words are crossed out now. Valid words scored with the 15% length bonus.
          </p>
          {result && (
            <>
              <div className="word-chip-row">
                {result.accepted_words.length === 0 ? (
                  <span className="pill">No accepted words this game</span>
                ) : (
                  result.accepted_words.map((word) => (
                    <span key={word} className="pill ready">
                      {word}
                    </span>
                  ))
                )}
              </div>
              <div className="grid">
                {result.results.map((player) => (
                  <div key={player.player_id} className="card" style={{ background: "#0f1720" }}>
                    <div className="row" style={{ justifyContent: "space-between" }}>
                      <strong>{player.player_name}</strong>
                      <span className="pill ready">Final score: {player.score}</span>
                    </div>
                    <p className="muted">
                      Made-up words removed: {player.made_up_count}
                    </p>
                    <div className="word-entry-list">
                      {(player.entries ?? []).map((entry, idx) => (
                        <EntryLine key={`${player.player_id}-${entry.word}-${idx}`} entry={entry} final />
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            </>
          )}
        </div>
      </div>
    );
  }

  return <div className="card">Preparing Word Storm...</div>;
}

function submitWord(
  event: FormEvent<HTMLFormElement>,
  draft: string,
  setDraft: (value: string) => void,
): void {
  event.preventDefault();
  const word = normalizeWord(draft);
  if (!word) return;
  client.send(C2S.SubmitWordList, { words: [word] });
  setDraft("");
}

function EntryLine({ entry, final = false }: { entry: WordEntryPayload; final?: boolean }): JSX.Element {
  const bad = entry.status === "invalid" || entry.status === "duplicate";
  return (
    <div className={`word-entry ${bad && final ? "crossed" : ""}`}>
      <span className={`pill ${entry.status === "pending" || entry.status === "valid" ? "ready" : "offline"}`}>
        {entry.letter.toUpperCase()}
      </span>
      <strong>{entry.word}</strong>
      <span className="muted">{entry.message}</span>
      {entry.points != null && entry.points > 0 && <span className="pill ready">+{entry.points}</span>}
    </div>
  );
}

function currentLetter(
  letters: WordLetterWindow[],
  letterSeconds: number,
  deadlineMs: number | null,
  now: number,
): WordLetterWindow {
  if (letters.length === 0) return { letter: "c", index: 0 };
  if (!deadlineMs) return letters[0]!;
  const totalMs = letters.length * letterSeconds * 1000;
  const startMs = deadlineMs - totalMs;
  const elapsedMs = Math.max(0, now - startMs);
  const index = Math.min(letters.length - 1, Math.floor(elapsedMs / (letterSeconds * 1000)));
  return letters[index]!;
}

function normalizeWord(raw: string): string {
  return raw.toLowerCase().replace(/[^a-z]/g, "");
}
