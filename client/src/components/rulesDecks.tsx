import type { RulesIntroDeck } from "./RulesIntro";

function drawScene(): JSX.Element {
  return (
    <div className="intro-phone">
      <div className="intro-phone-bar" />
      <div className="intro-canvas">
        <div className="intro-stroke stroke-a" />
        <div className="intro-stroke stroke-b" />
        <div className="intro-stroke stroke-c" />
      </div>
      <div className="intro-caption">Draw this: astronaut at brunch</div>
    </div>
  );
}

function bluffScene(): JSX.Element {
  return (
    <>
      <div className="intro-stack">
        <div className="intro-choice fake">"dog at a piano"</div>
        <div className="intro-choice true">"astronaut at brunch"</div>
        <div className="intro-choice fake ghost">"space chef selfie"</div>
      </div>
      <div className="intro-badge">Fake picks pay you</div>
    </>
  );
}

function scoreScene(): JSX.Element {
  return (
    <div className="intro-scoreboard">
      <div className="score-row">
        <span>You fooled Bob</span>
        <span className="ok">+500</span>
      </div>
      <div className="score-row">
        <span>You picked truth</span>
        <span className="ok">+1000</span>
      </div>
      <div className="score-row">
        <span>Your drawing got 2 true votes</span>
        <span className="ok">+2000</span>
      </div>
    </div>
  );
}

function fakeArtistRoleScene(): JSX.Element {
  return (
    <div className="artist-sequence">
      <div className="artist-role-card truth">
        <div className="artist-role-tag">Most players</div>
        <strong>Prompt: Roller Skate</strong>
        <span>Draw like you know exactly what this is.</span>
      </div>
      <div className="artist-role-card fake">
        <div className="artist-role-tag">One fake artist</div>
        <strong>No prompt shown</strong>
        <span>Blend in by copying the group's style.</span>
      </div>
    </div>
  );
}

function fakeArtistTurnsScene(): JSX.Element {
  return (
    <div className="artist-turn-board">
      <div className="artist-turn-banner">Jordan draws next</div>
      <div className="artist-turn-countdown">3</div>
      <div className="artist-canvas-stage">
        <div className="artist-canvas-name">Jordan is drawing now</div>
        <div className="artist-canvas-grid">
          <span className="artist-stroke blue" />
          <span className="artist-stroke yellow" />
          <span className="artist-stroke red" />
        </div>
      </div>
      <div className="artist-turn-queue">
        <span className="pill ready">Jordan</span>
        <span className="pill">Maya</span>
        <span className="pill">Chris</span>
      </div>
    </div>
  );
}

function fakeArtistReplayScene(): JSX.Element {
  return (
    <div className="artist-replay-shell">
      <div className="artist-replay-header">Replay the drawing from the beginning</div>
      <div className="artist-replay-strip">
        <div className="artist-replay-step">
          <strong>Turn 1</strong>
          <span>Jordan countdown</span>
        </div>
        <div className="artist-replay-step active">
          <strong>Turn 2</strong>
          <span>Maya draws</span>
        </div>
        <div className="artist-replay-step">
          <strong>Turn 3</strong>
          <span>Chris countdown</span>
        </div>
      </div>
      <div className="artist-canvas-grid replay">
        <span className="artist-stroke blue" />
        <span className="artist-stroke green" />
        <span className="artist-stroke pink" />
      </div>
    </div>
  );
}

function fakeArtistVoteScene(): JSX.Element {
  return (
    <div className="artist-vote-scene">
      <div className="artist-vote-header">Who looked like they were guessing?</div>
      <div className="artist-vote-grid">
        <div className="artist-vote-card">
          <strong>Jordan</strong>
          <span>Confident strokes</span>
        </div>
        <div className="artist-vote-card suspect">
          <strong>Maya</strong>
          <span>Hesitant additions</span>
        </div>
        <div className="artist-vote-card">
          <strong>Chris</strong>
          <span>Matched the prompt well</span>
        </div>
      </div>
    </div>
  );
}

function fakeArtistGuessScene(): JSX.Element {
  return (
    <div className="artist-guess-scene">
      <div className="artist-guess-branch win">
        <strong>Room misses fake</strong>
        <span>Fake artist wins immediately.</span>
      </div>
      <div className="artist-guess-branch steal">
        <strong>Room catches fake</strong>
        <span>Fake artist gets one prompt guess for the steal.</span>
      </div>
      <div className="artist-guess-input">Guess the prompt: roller skate</div>
    </div>
  );
}

function reactionCountdownScene(): JSX.Element {
  return (
    <div className="guide-stack">
      <div className="guide-pillbar">
        <span className="pill ready">Visible countdown</span>
        <span className="pill">Random wait</span>
        <span className="pill">Green light</span>
      </div>
      <div className="signal-card red">
        <div className="signal-badge">Red button</div>
        <strong>3... 2... 1...</strong>
        <span>Do not press yet.</span>
      </div>
    </div>
  );
}

function reactionGoScene(): JSX.Element {
  return (
    <div className="guide-stack">
      <div className="signal-card green">
        <div className="signal-badge">Go signal</div>
        <strong>TAP NOW</strong>
        <span>Fastest legal tap wins.</span>
      </div>
      <div className="guide-flow">
        <div className="guide-flow-step danger">
          <strong>False start</strong>
          <span>Press red and you lose.</span>
        </div>
        <div className="guide-flow-step accent">
          <strong>Server timing</strong>
          <span>Both reaction times are revealed.</span>
        </div>
      </div>
    </div>
  );
}

function splitAuthorScene(): JSX.Element {
  return (
    <div className="guide-stack">
      <div className="guide-window">
        <div className="guide-window-title">Splitter screen</div>
        <div className="guide-prompt-line">Prompt: Which sounds like the better road trip snack?</div>
        <div className="guide-option-row">
          <span className="guide-option-chip">Spicy chips</span>
          <span className="guide-option-chip">Chocolate pretzels</span>
        </div>
      </div>
      <div className="guide-note-box">Only the splitter sees the hidden target in variable mode.</div>
    </div>
  );
}

function splitVoteScene(): JSX.Element {
  return (
    <div className="guide-stack">
      <div className="vote-columns">
        <div className="vote-column">
          <strong>A</strong>
          <span>Spicy chips</span>
          <div className="vote-bar left" />
        </div>
        <div className="vote-column">
          <strong>B</strong>
          <span>Chocolate pretzels</span>
          <div className="vote-bar right" />
        </div>
      </div>
      <div className="guide-pillbar">
        <span className="pill ready">Strict split</span>
        <span className="pill">Variable</span>
        <span className="pill">Odd one</span>
      </div>
    </div>
  );
}

function splitScoreScene(): JSX.Element {
  return (
    <div className="guide-stack">
      <div className="ratio-board">
        <div className="ratio-track">
          <span className="ratio-fill" />
          <span className="ratio-target" />
        </div>
        <div className="ratio-labels">
          <span>Actual room split</span>
          <span>Target</span>
        </div>
      </div>
      <div className="intro-scoreboard compact">
        <div className="score-row">
          <span>Closer to target</span>
          <span className="ok">more points</span>
        </div>
        <div className="score-row">
          <span>Splitter never votes</span>
          <span className="ok">designs chaos</span>
        </div>
      </div>
    </div>
  );
}

function priceProductScene(): JSX.Element {
  return (
    <div className="guide-stack">
      <div className="price-card">
        <div className="price-photo" />
        <strong>Vintage stereo receiver</strong>
        <span>Only items above $100 this round</span>
      </div>
      <div className="guide-note-box">Threshold mode can stay fixed or rise by x10 each round.</div>
    </div>
  );
}

function priceGuessScene(): JSX.Element {
  return (
    <div className="guide-stack">
      <div className="guess-strip">
        <div className="guess-chip safe">$182</div>
        <div className="guess-chip over">$245</div>
        <div className="guess-chip safe">$199</div>
      </div>
      <div className="guide-flow">
        <div className="guide-flow-step accent">
          <strong>Closest under wins</strong>
          <span>Safe guesses matter.</span>
        </div>
        <div className="guide-flow-step danger">
          <strong>Overbid</strong>
          <span>Instantly out of the round.</span>
        </div>
      </div>
    </div>
  );
}

function wordLetterScene(): JSX.Element {
  return (
    <div className="guide-stack">
      <div className="letter-burst">C</div>
      <div className="guide-pillbar">
        <span className="pill ready">camel</span>
        <span className="pill ready">cabin</span>
        <span className="pill">clocked</span>
      </div>
      <div className="guide-note-box">Letters rotate every few seconds, so speed matters.</div>
    </div>
  );
}

function wordClaimScene(): JSX.Element {
  return (
    <div className="guide-stack">
      <div className="word-feed">
        <div className="word-feed-row">
          <strong>Maya</strong>
          <span className="ok">claims "candle"</span>
        </div>
        <div className="word-feed-row">
          <strong>Jordan</strong>
          <span>Already entered</span>
        </div>
        <div className="word-feed-row">
          <strong>Chris</strong>
          <span className="ok">tentative +793</span>
        </div>
      </div>
      <div className="guide-note-box">First claim locks the word for the rest of the game.</div>
    </div>
  );
}

function wordCleanupScene(): JSX.Element {
  return (
    <div className="guide-stack">
      <div className="cleanup-list">
        <div className="cleanup-row keep">cabin</div>
        <div className="cleanup-row keep">camera</div>
        <div className="cleanup-row cut">crombulous</div>
      </div>
      <div className="guide-note-box">Official scoring happens at the end after invalid words are crossed out.</div>
    </div>
  );
}

function drawDuelArtistsScene(): JSX.Element {
  return (
    <div className="guide-stack">
      <div className="duel-lane">
        <div className="duel-card">
          <strong>Artist A</strong>
          <span>Blue line, bold stroke</span>
        </div>
        <div className="duel-card">
          <strong>Artist B</strong>
          <span>Yellow line, thin stroke</span>
        </div>
      </div>
      <div className="guide-note-box">Both artists draw the same prompt before the judges step in.</div>
    </div>
  );
}

function drawDuelJudgeScene(): JSX.Element {
  return (
    <div className="guide-stack">
      <div className="guide-window">
        <div className="guide-window-title">Judge view</div>
        <div className="duel-vote-row">
          <span className="guide-option-chip">Vote Artist A</span>
          <span className="guide-option-chip">Vote Artist B</span>
        </div>
      </div>
      <div className="guide-flow">
        <div className="guide-flow-step accent">
          <strong>At least 3 players</strong>
          <span>You need someone free to judge.</span>
        </div>
        <div className="guide-flow-step">
          <strong>Rounds rotate</strong>
          <span>Different players can become artists.</span>
        </div>
      </div>
    </div>
  );
}

function mafiaNightScene(): JSX.Element {
  return (
    <div className="guide-stack">
      <div className="mafia-board">
        <div className="mafia-chip">Doctor protects</div>
        <div className="mafia-chip danger">Mafia kills</div>
        <div className="mafia-chip accent">Detective investigates</div>
      </div>
      <div className="guide-note-box">Night always resolves in this exact order.</div>
    </div>
  );
}

function mafiaDayScene(): JSX.Element {
  return (
    <div className="guide-stack">
      <div className="guide-window">
        <div className="guide-window-title">Day phase</div>
        <div className="guide-prompt-line">Discuss suspicious behavior, nominate a player, then vote.</div>
      </div>
      <div className="guide-pillbar">
        <span className="pill">Discussion</span>
        <span className="pill ready">Nomination</span>
        <span className="pill">Vote</span>
      </div>
    </div>
  );
}

function mafiaConfigScene(): JSX.Element {
  return (
    <div className="guide-stack">
      <div className="guide-flow">
        <div className="guide-flow-step">
          <strong>Reveal on death</strong>
          <span>Cleaner, faster deduction.</span>
        </div>
        <div className="guide-flow-step">
          <strong>Hidden deaths</strong>
          <span>More bluffing and ambiguity.</span>
        </div>
      </div>
      <div className="guide-note-box">Exact-role investigations and hidden roles both push the difficulty upward.</div>
    </div>
  );
}

export const drawfulRulesDeck: RulesIntroDeck = {
  gameId: "jrawful",
  gameName: "Jrawful",
  previewTitle: "Jrawful Rules Preview",
  slides: [
    {
      id: "draw",
      eyebrow: "1. Draw",
      title: "Turn your prompt into a quick sketch",
      body: "Everyone gets a secret prompt and submits one drawing before the timer runs out.",
      points: "No points yet - this is the setup.",
      notes: [
        "You only need one clear idea, not a masterpiece.",
        "The drawing becomes the bait for everyone else's fake answers.",
      ],
      scene: drawScene(),
    },
    {
      id: "fake",
      eyebrow: "2. Bluff",
      title: "Write a fake prompt that sounds real",
      body: "For every drawing that is not yours, submit a believable fake answer and try to lure other players in.",
      points: "+500 for each player who picks your fake prompt.",
      notes: [
        "If two players submit the same fake, those fake points get split.",
        "Picking your own fake is not allowed.",
      ],
      scene: bluffScene(),
    },
    {
      id: "score",
      eyebrow: "3. Score",
      title: "Truth pays too",
      body: "When voting starts, pick the real prompt. The reveal shows who fooled whom and where every point landed.",
      points: "+1000 if you pick the true prompt. Drawer gets +1000 for each true vote on their drawing.",
      notes: [
        "Wrong guesses score zero for the guesser.",
        "The leaderboard updates after every round, then the next prompt starts.",
      ],
      scene: scoreScene(),
    },
  ],
};

export const fakeArtistRulesDeck: RulesIntroDeck = {
  gameId: "fake_artist",
  gameName: "Fake Artist",
  previewTitle: "Fake Artist Rules Preview",
  slides: [
    {
      id: "roles",
      eyebrow: "1. Secret Roles",
      title: "Everyone but one player gets the prompt",
      body: "The real artists see the round prompt. The fake artist sees no prompt and has to survive by imitating the group.",
      points: "Goal: avoid getting voted as the fake, or steal the win with the final guess.",
      notes: [
        "Only the fake artist is kept in the dark.",
        "Prompt knowledge matters more than drawing skill.",
      ],
      scene: fakeArtistRoleScene(),
    },
    {
      id: "turns",
      eyebrow: "2. Turn Order",
      title: "Players draw one at a time with a visible countdown",
      body: "Before each turn, the next drawer's name appears with a 3 second countdown. Then everyone watches that player add to the shared canvas.",
      points: "The whole room sees who is drawing and what they add.",
      notes: [
        "Color coding can give each drawer a unique color.",
        "The fake artist has to guess from what is already on the board.",
      ],
      scene: fakeArtistTurnsScene(),
    },
    {
      id: "replay",
      eyebrow: "3. Replay",
      title: "The drawing replays from start to finish before voting",
      body: "Once the drawing turns end, the game replays each countdown and each player's additions in sequence so the room can discuss suspicious moments.",
      points: "Replay count and continuous replay are host settings.",
      notes: [
        "The replay should make hesitation and style changes easier to spot.",
        "Everyone gets the same evidence before voting.",
      ],
      scene: fakeArtistReplayScene(),
    },
    {
      id: "vote",
      eyebrow: "4. Vote",
      title: "Vote for the player who looked like they were guessing",
      body: "After the replay, the room votes for the fake artist. Discussion happens during the vote timer.",
      points: "Majority catches the fake. Wrong accusation gives the fake the round.",
      notes: [
        "Use the replay, not just the final image.",
        "The fake artist wins immediately if the room points at the wrong player.",
      ],
      scene: fakeArtistVoteScene(),
    },
    {
      id: "guess",
      eyebrow: "5. Final Guess",
      title: "Caught fake artists can still steal the round",
      body: "If the room catches the fake artist, that player gets one last chance to guess the prompt. A correct guess steals the round back.",
      points: "Caught + wrong final guess means the true artists win.",
      notes: [
        "Missing the fake ends the round right away in the fake artist's favor.",
        "Catching the fake is only half the job.",
      ],
      scene: fakeArtistGuessScene(),
    },
  ],
};

export const reactionDuelRulesDeck: RulesIntroDeck = {
  gameId: "reaction_duel",
  gameName: "Reaction Duel",
  previewTitle: "Reaction Duel Rules Preview",
  slides: [
    {
      id: "countdown",
      eyebrow: "1. Red Light",
      title: "The duel starts with a visible countdown",
      body: "The host sets how long the red-button countdown stays on screen before the hidden random wait begins.",
      points: "Red means wait. Touching early loses the round.",
      notes: [
        "The countdown is visible to both players.",
        "The actual green-light moment is still unpredictable.",
      ],
      scene: reactionCountdownScene(),
    },
    {
      id: "go",
      eyebrow: "2. Green Light",
      title: "Tap only after the button turns green",
      body: "After the random delay, the button flips green. The server records the first legal tap and reveals both reaction times.",
      points: "Fastest legal tap wins. False starts lose immediately.",
      notes: [
        "This is a timing game, not a guessing game.",
        "Patience is part of your reaction time.",
      ],
      scene: reactionGoScene(),
    },
  ],
};

export const splitVoteRulesDeck: RulesIntroDeck = {
  gameId: "split_vote",
  gameName: "Split the Vote",
  previewTitle: "Split the Vote Rules Preview",
  slides: [
    {
      id: "author",
      eyebrow: "1. Splitter",
      title: "One player creates the trap",
      body: "The splitter writes the full prompt and options, or fills in options for a generated prompt depending on the lobby setting.",
      points: "The splitter does not vote.",
      notes: [
        "Variable targets are only shown to the splitter.",
        "Targets are always achievable with the current voter count.",
      ],
      scene: splitAuthorScene(),
    },
    {
      id: "vote",
      eyebrow: "2. Room Vote",
      title: "Everyone else tries to land the distribution",
      body: "The room votes on the two options. Strict split wants 50-50, variable aims for a hidden target, and odd one wants one lonely vote on one side.",
      points: "Your score depends on how closely the final vote matches the mode's target.",
      notes: [
        "The splitter designs the situation but is excluded from the count.",
        "Odd one rewards dramatic lopsided results.",
      ],
      scene: splitVoteScene(),
    },
    {
      id: "score",
      eyebrow: "3. Reveal",
      title: "Scoring is all about distance from target",
      body: "The reveal shows the actual vote ratio against the target and pays out more when the room lands closer to the ideal outcome.",
      points: "Perfect distributions score best.",
      notes: [
        "Strict split wants balance.",
        "Odd one wants maximum imbalance without shutting one side out entirely.",
      ],
      scene: splitScoreScene(),
    },
  ],
};

export const priceIsRightRulesDeck: RulesIntroDeck = {
  gameId: "price_is_right",
  gameName: "Price is Right",
  previewTitle: "Price is Right Rules Preview",
  slides: [
    {
      id: "product",
      eyebrow: "1. Product",
      title: "A real product appears with a minimum threshold",
      body: "Each round shows a product image and title. The host can keep the minimum item price fixed or increase it by a factor of ten across rounds.",
      points: "Only products at or above the threshold can appear.",
      notes: [
        "Higher thresholds push the room into wilder categories.",
        "The item is the same for every player that round.",
      ],
      scene: priceProductScene(),
    },
    {
      id: "guess",
      eyebrow: "2. Guess",
      title: "Closest without going over wins",
      body: "Players submit one price guess before the timer ends. Any guess above the real price is busted immediately.",
      points: "Closest legal guess takes the round.",
      notes: [
        "A careful underbid can beat a flashy overbid.",
        "You are playing the true price, not the threshold.",
      ],
      scene: priceGuessScene(),
    },
  ],
};

export const wordStormRulesDeck: RulesIntroDeck = {
  gameId: "word_storm",
  gameName: "Word Storm",
  previewTitle: "Word Storm Rules Preview",
  slides: [
    {
      id: "letters",
      eyebrow: "1. Letter Window",
      title: "Each round rotates through live starting letters",
      body: "A letter stays active for a short timer, then the game advances to the next one. Players race to type valid words before the window closes.",
      points: "Longer words are worth more, especially past four letters.",
      notes: [
        "Speed matters because the letter does not wait.",
        "Every player sees the same live letter at the same time.",
      ],
      scene: wordLetterScene(),
    },
    {
      id: "claims",
      eyebrow: "2. First Claim",
      title: "The first valid claim owns the word",
      body: "Once a player claims a word first, nobody else can use it later in the game. Duplicate entries get rejected as already entered.",
      points: "Being first matters as much as being correct.",
      notes: [
        "Wrong-letter entries do not claim the word.",
        "Made-up first claims can still block later duplicates until cleanup.",
      ],
      scene: wordClaimScene(),
    },
    {
      id: "cleanup",
      eyebrow: "3. Final Cleanup",
      title: "Official scoring happens after dictionary cleanup",
      body: "At the end of the game, invalid or made-up words are crossed out and only the validated list survives on the final scoreboard.",
      points: "Provisional acceptance is not final acceptance.",
      notes: [
        "The reveal shows removed words and made-up word counts.",
        "Only validated words pay official points.",
      ],
      scene: wordCleanupScene(),
    },
  ],
};

export const drawDuelRulesDeck: RulesIntroDeck = {
  gameId: "draw_duel",
  gameName: "Draw Duel",
  previewTitle: "Draw Duel Rules Preview",
  slides: [
    {
      id: "artists",
      eyebrow: "1. Duel",
      title: "Two artists draw the same prompt head-to-head",
      body: "Artists race the timer using the selected colors and line weights while the rest of the room watches or waits to judge.",
      points: "The host controls timer length and round count.",
      notes: [
        "At least one non-artist judge is required.",
        "A bold simple idea often beats over-detailing.",
      ],
      scene: drawDuelArtistsScene(),
    },
    {
      id: "judges",
      eyebrow: "2. Judges",
      title: "Everyone else votes for the stronger drawing",
      body: "After both drawings are in, the judges decide which artist won the round. Multiple rounds rotate more players into the duel seats.",
      points: "Round wins stack toward the match result.",
      notes: [
        "Artists do not judge their own duel.",
        "Three players is the minimum because someone has to judge.",
      ],
      scene: drawDuelJudgeScene(),
    },
  ],
};

export const mafiaRulesDeck: RulesIntroDeck = {
  gameId: "mafia",
  gameName: "Mafia",
  previewTitle: "Mafia Rules Preview",
  slides: [
    {
      id: "night",
      eyebrow: "1. Night",
      title: "Power roles act in secret every night",
      body: "Mafia choose a target, the doctor can protect, and the detective investigates. Night always resolves in the same order.",
      points: "Protect -> Kill -> Investigate.",
      notes: [
        "The detective still gets a result even if the target dies that night.",
        "Self-protect depends on the room setting.",
      ],
      scene: mafiaNightScene(),
    },
    {
      id: "day",
      eyebrow: "2. Day",
      title: "Discuss, nominate, then vote publicly",
      body: "The room debates suspicious behavior, nominates a candidate, and votes. Tie resolution follows the selected lobby rule.",
      points: "Town wins by eliminating every Mafia member.",
      notes: [
        "The mayor can swing the vote if that role is in play.",
        "Dead players are out of actions and out of the day chat.",
      ],
      scene: mafiaDayScene(),
    },
    {
      id: "settings",
      eyebrow: "3. Difficulty",
      title: "Lobby settings change how readable the game feels",
      body: "Role reveals on death make the game cleaner and faster. Hidden deaths and exact-role investigations make reads more complicated and swingy.",
      points: "Use reveals for quicker deduction and hidden info for heavier bluffing.",
      notes: [
        "Bigger groups can usually handle more hidden information.",
        "The role preview buttons below still explain each role in detail.",
      ],
      scene: mafiaConfigScene(),
    },
  ],
};

const decks: Record<string, RulesIntroDeck> = {
  [drawfulRulesDeck.gameId]: drawfulRulesDeck,
  drawful: drawfulRulesDeck,
  [fakeArtistRulesDeck.gameId]: fakeArtistRulesDeck,
  [reactionDuelRulesDeck.gameId]: reactionDuelRulesDeck,
  [splitVoteRulesDeck.gameId]: splitVoteRulesDeck,
  [priceIsRightRulesDeck.gameId]: priceIsRightRulesDeck,
  [wordStormRulesDeck.gameId]: wordStormRulesDeck,
  [drawDuelRulesDeck.gameId]: drawDuelRulesDeck,
  [mafiaRulesDeck.gameId]: mafiaRulesDeck,
};

export function getRulesDeck(gameId: string): RulesIntroDeck {
  return decks[gameId] ?? drawfulRulesDeck;
}
