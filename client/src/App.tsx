// App is the top-level router. It inspects the store's `phase` string and
// picks a screen. Routing on phase name (rather than a discriminated state
// machine enum) keeps the client stateless — one authoritative phase,
// sourced from the server, drives every render.

import { useEffect, useState } from "react";
import { client, useGameState } from "./store";
import Lobby from "./screens/Lobby";
import GamePicker from "./screens/GamePicker";
import DrawingScreen from "./screens/Drawing";
import FakePrompt from "./screens/FakePrompt";
import Voting from "./screens/Voting";
import Reveal from "./screens/Reveal";
import Leaderboard from "./screens/Leaderboard";
import LeaderboardRound from "./screens/LeaderboardRound";
import LeaderControls from "./components/LeaderControls";
import GameEnd from "./screens/GameEnd";
import PlatformResults from "./screens/PlatformResults";
import ReactionDuel from "./screens/ReactionDuel";
import SplitTheVote from "./screens/SplitTheVote";
import FakeArtist from "./screens/FakeArtist";
import DrawDuel from "./screens/DrawDuel";
import Mafia from "./screens/Mafia";
import PriceIsRight from "./screens/PriceIsRight";
import RulesIntro from "./components/RulesIntro";
import { getRulesDeck } from "./components/rulesDecks";

export default function App(): JSX.Element {
  const g = useGameState();
  const search = typeof window !== "undefined" ? new URLSearchParams(window.location.search) : null;
  const preview = search?.get("preview") ?? null;
  const previewGame = search?.get("game") ?? "drawful";
  const previewDeck = getRulesDeck(previewGame);
  const [joinForm, setJoinForm] = useState<{ name: string; roomId: string }>({
    name: sessionStorage.getItem("jacksonbox.player_name") ?? "",
    roomId: sessionStorage.getItem("jacksonbox.room_id") ?? "MAIN",
  });

  // Auto-reconnect on boot if we have a stashed token.
  useEffect(() => {
    if (preview === "rules") return;
    const tok = sessionStorage.getItem("jacksonbox.session_token");
    const name = sessionStorage.getItem("jacksonbox.player_name");
    if (tok && name && g.conn === "idle") {
      client.connect("MAIN", name);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  if (preview === "rules") {
    return (
      <div className="app">
        <div className="card">
          <h1>{previewDeck.previewTitle}</h1>
          <p className="muted">
            Standalone preview of the lobby explainer animation for {previewDeck.gameName}.
          </p>
        </div>
        <RulesIntro deck={previewDeck} />
      </div>
    );
  }

  if (g.evictingReason) {
    return (
      <div className="app">
        <div className="banner warn">
          Room is being reset: {g.evictingReason}. You'll be disconnected shortly.
        </div>
      </div>
    );
  }

  if (g.conn === "idle") {
    return (
      <div className="app">
        <div className="card">
          <h1>Jackson Box</h1>
          <p className="muted">
            Join a room, pick a game, and let the host launch the party pack.
          </p>
          <div className="row">
            <input
              type="text"
              placeholder="room code"
              value={joinForm.roomId}
              onChange={(e) => setJoinForm({ ...joinForm, roomId: e.target.value.toUpperCase() })}
              maxLength={24}
            />
            <input
              type="text"
              placeholder="your name"
              value={joinForm.name}
              onChange={(e) => setJoinForm({ ...joinForm, name: e.target.value })}
              maxLength={24}
            />
            <button
              className="primary"
              disabled={joinForm.name.trim() === "" || joinForm.roomId.trim() === ""}
              onClick={() => client.connect(joinForm.roomId.trim(), joinForm.name.trim())}
            >
              Join
            </button>
          </div>
        </div>
        <InstructionCard />
      </div>
    );
  }

  if (g.conn === "closed") {
    return (
      <div className="app">
        <div className="banner err">
          Disconnected ({g.closeCode ?? "?"}). {g.closeReason || ""}
        </div>
        <button onClick={() => window.location.reload()}>Start over</button>
      </div>
    );
  }

  const screen = pickScreen(g.roomMode, g.phase);

  return (
    <div className="app">
      {g.lastError && (
        <div className="banner err">
          {g.lastError.code}: {g.lastError.message}
        </div>
      )}
      {g.conn === "connecting" && <div className="banner warn">Reconnecting…</div>}
      <LeaderControls />
      {screen}
      {/* Sidebar scoreboard. Suppressed during leaderboard_rN because the
          LeaderboardRound screen renders its own full scoreboard — two would
          double-count visually. */}
      {!g.phase.startsWith("leaderboard") && <Leaderboard />}
    </div>
  );
}

function InstructionCard(): JSX.Element {
  return (
    <div className="card instruction-card">
      <h2>How Jrawful works</h2>
      <div className="instruction-grid">
        <div>
          <h3>1. Draw</h3>
          <p className="muted">
            Everyone gets a secret prompt and draws it. You can reroll your prompt once before
            submitting if the first one does not spark anything.
          </p>
        </div>
        <div>
          <h3>2. Bluff</h3>
          <p className="muted">
            For every drawing that is not yours, write a fake prompt that could fool the room.
            The host can add generated fake choices for extra chaos.
          </p>
        </div>
        <div>
          <h3>3. Vote</h3>
          <p className="muted">
            Pick the real prompt. You cannot vote on your own drawing or pick your own fake.
          </p>
        </div>
        <div>
          <h3>4. Score</h3>
          <p className="muted">
            Get +1000 for finding the truth, +500 whenever someone chooses your fake, and
            +1000 as drawer for each player who guesses your real prompt.
          </p>
        </div>
      </div>
    </div>
  );
}

function pickScreen(roomMode: string, phase: string): JSX.Element {
  if (roomMode === "game_picker") return <GamePicker />;
  if (roomMode === "game_lobby") return <Lobby />;
  if (roomMode === "results") return <PlatformResults />;
  if (phase.startsWith("reaction_")) return <ReactionDuel key={phase || "reaction"} />;
  if (phase.startsWith("split_")) return <SplitTheVote key={phase || "split"} />;
  if (phase.startsWith("fake_artist_")) return <FakeArtist key={phase || "fake-artist"} />;
  if (phase.startsWith("draw_duel_")) return <DrawDuel key={phase || "draw-duel"} />;
  if (phase.startsWith("mafia_")) return <Mafia key={phase || "mafia"} />;
  if (phase.startsWith("price_")) return <PriceIsRight key={phase || "price-is-right"} />;
  const key = phase || "lobby";
  if (phase.startsWith("drawing_submit")) return <DrawingScreen key={key} />;
  if (phase.startsWith("fake_prompt_submit")) return <FakePrompt key={key} />;
  if (phase.startsWith("voting")) return <Voting key={key} />;
  if (phase.startsWith("reveal")) return <Reveal key={key} />;
  if (phase.startsWith("scoring")) return <Reveal key={key} />; // scoring runs aggregate; reveal UI persists
  if (phase.startsWith("leaderboard")) return <LeaderboardRound key={key} />;
  if (phase === "game_end") return <GameEnd key={key} />;
  return <Lobby key={key} />;
}
