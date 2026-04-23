// Leader-only control band. Renders pause / resume + pencils-down toggle
// for the party leader; renders a small status pill for everyone else so
// non-leaders know who's in charge and why the game is frozen.
//
// Kept as a dumb component: reads leaderId / paused / pencilsDown straight
// off the store, dispatches C2S messages on click. The server is the single
// source of truth — we never optimistically flip our own paused/pencilsDown
// locally, we wait for pause_state to echo back.

import { useEffect, useState } from "react";
import { C2S } from "../proto";
import { client, useGameState } from "../store";

export default function LeaderControls(): JSX.Element | null {
  const g = useGameState();
  const [menuOpen, setMenuOpen] = useState(false);

  // Hide in lobby (no meaningful state to pause) and in game_end. Empty
  // phase → mid-join; also hide. Unknown/empty leaderId → also hide.
  const inGame = g.phase !== "" && g.phase !== "game_end" && !g.phase.startsWith("lobby");

  useEffect(() => {
    if (!g.paused) {
      setMenuOpen(false);
    }
  }, [g.paused]);

  if (!inGame || !g.leaderId) return null;

  const iAmLeader = g.leaderId === g.playerId;
  const leaderName = g.players.find((p) => p.id === g.leaderId)?.name ?? "?";
  const inLeaderboard = g.phase.startsWith("leaderboard");
  const allRevealed = g.reveals.length > 0 && g.reveals.every((r) => g.revealFinals[r.drawing_id]);

  function togglePause(): void {
    client.send(C2S.SetPause, { paused: !g.paused });
  }
  function togglePencils(): void {
    client.send(C2S.SetPencilsDown, { disabled: !g.pencilsDown });
  }
  function advanceReveal(): void {
    client.send(C2S.AdvanceReveal, {});
  }
  function returnToPicker(): void {
    setMenuOpen(false);
    client.send(C2S.ReturnToPicker, {});
  }

  return (
    <div
      className="card"
      style={{
        padding: "8px 12px",
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        gap: 12,
      }}
    >
      <div className="row" style={{ gap: 8, alignItems: "center" }}>
        <span className="muted" style={{ fontSize: 12 }}>Leader:</span>
        <strong>{leaderName}{iAmLeader ? " (you)" : ""}</strong>
        {g.paused && <span className="pill offline">paused</span>}
        {g.pencilsDown && <span className="pill offline">pencils down</span>}
        {inLeaderboard && <span className="pill">leader-controlled reveal</span>}
      </div>
      {iAmLeader && (
        <div className="row" style={{ gap: 8 }}>
          {inLeaderboard ? (
            <button className="primary" onClick={advanceReveal}>
              {allRevealed ? "Start next round" : "Next reveal"}
            </button>
          ) : (
            <>
              <button onClick={togglePause}>
                {g.paused ? "Resume" : "Pause"}
              </button>
              <button onClick={togglePencils}>
                {g.pencilsDown ? "Pencils up" : "Pencils down"}
              </button>
              {g.paused && (
                <div className="leader-menu-wrap">
                  <button
                    type="button"
                    aria-label="Game menu"
                    aria-expanded={menuOpen}
                    onClick={() => setMenuOpen((open) => !open)}
                  >
                    ⚙
                  </button>
                  {menuOpen && (
                    <div className="leader-menu card">
                      <button type="button" onClick={returnToPicker}>
                        Exit to picker
                      </button>
                    </div>
                  )}
                </div>
              )}
            </>
          )}
        </div>
      )}
    </div>
  );
}
