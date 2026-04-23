import { useMemo, useState } from "react";
import Countdown from "../components/Countdown";
import { C2S } from "../proto";
import { client, useGameState } from "../store";

export default function PriceIsRight(): JSX.Element {
  const g = useGameState();
  const [guess, setGuess] = useState("");
  const prompt = g.pricePrompt;
  const result = g.priceResult;
  const winners = useMemo(
    () => g.players.filter((player) => result?.winner_ids.includes(player.id)),
    [g.players, result?.winner_ids],
  );

  if (g.phase.startsWith("price_guess")) {
    return (
      <div className="stack">
        <div className="card">
          <div className="row" style={{ justifyContent: "space-between" }}>
            <div>
              <h2>Price is Right</h2>
              <p className="muted">
                Guess the price. Closest guess without going over wins.
              </p>
            </div>
            <Countdown deadlineMs={g.deadlineMs} paused={g.paused} remainingMs={g.pauseRemainingMs} />
          </div>
        </div>

        <div className="card">
          {prompt && (
            <>
              <div className="row" style={{ justifyContent: "space-between", alignItems: "flex-start", gap: 16 }}>
                <div style={{ flex: 1 }}>
                  <div className="pill" style={{ marginBottom: 12 }}>
                    Price is greater than {formatMoney(prompt.threshold_cents)}
                  </div>
                  <h3>{prompt.product_name}</h3>
                  <p className="muted">
                    Mode: {prompt.threshold_mode === "times_ten" ? "threshold x10 each round" : "fixed threshold"}
                  </p>
                </div>
                <img
                  src={prompt.image_url}
                  alt={prompt.product_name}
                  style={{
                    width: "min(100%, 340px)",
                    maxWidth: 340,
                    borderRadius: 20,
                    border: "1px solid rgba(148, 163, 184, 0.35)",
                    background: "#f8fafc",
                  }}
                />
              </div>

              <div className="row wrap" style={{ marginTop: 18, gap: 12 }}>
                <input
                  type="number"
                  min="0"
                  step="0.01"
                  inputMode="decimal"
                  placeholder="Enter your guess"
                  value={guess}
                  onChange={(e) => setGuess(e.target.value)}
                  style={{ flex: 1, minWidth: 220 }}
                />
                <button
                  className="primary"
                  disabled={guess.trim() === "" || Number.isNaN(Number(guess))}
                  onClick={() =>
                    client.send(C2S.SubmitPriceGuess, {
                      guess_cents: Math.max(0, Math.round(Number(guess) * 100)),
                    })
                  }
                >
                  Lock guess
                </button>
              </div>
            </>
          )}
        </div>
      </div>
    );
  }

  if (g.phase.startsWith("price_reveal")) {
    return (
      <div className="stack">
        <div className="card">
          <h2>Price Reveal</h2>
          {result && (
            <>
              <div className="row" style={{ justifyContent: "space-between", alignItems: "flex-start", gap: 16 }}>
                <div style={{ flex: 1 }}>
                  <h3>{result.product_name}</h3>
                  <p className="muted">Actual price: {formatMoney(result.actual_price_cents)}</p>
                  <p className="muted">
                    {result.winner_ids.length > 0
                      ? `Winner${result.winner_ids.length > 1 ? "s" : ""}: ${winners.map((player) => player.name).join(", ")}`
                      : "Everyone went over. No points this round."}
                  </p>
                </div>
                <img
                  src={result.image_url}
                  alt={result.product_name}
                  style={{
                    width: "min(100%, 340px)",
                    maxWidth: 340,
                    borderRadius: 20,
                    border: "1px solid rgba(148, 163, 184, 0.35)",
                    background: "#f8fafc",
                  }}
                />
              </div>

              <div className="card" style={{ marginTop: 16, background: "#0f1720" }}>
                <h3>Guesses</h3>
                <div className="grid">
                  {g.players.map((player) => (
                    <div key={player.id} className="row" style={{ justifyContent: "space-between" }}>
                      <span>{player.name}</span>
                      <span>
                        {result.guesses[player.id] != null ? formatMoney(result.guesses[player.id]!) : "No guess"}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            </>
          )}
        </div>
      </div>
    );
  }

  return <div className="card">Preparing Price is Right...</div>;
}

function formatMoney(cents: number): string {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 2,
  }).format(cents / 100);
}
