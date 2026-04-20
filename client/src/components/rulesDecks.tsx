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

export const drawfulRulesDeck: RulesIntroDeck = {
  gameId: "drawful",
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

const decks: Record<string, RulesIntroDeck> = {
  [drawfulRulesDeck.gameId]: drawfulRulesDeck,
};

export function getRulesDeck(gameId: string): RulesIntroDeck {
  return decks[gameId] ?? drawfulRulesDeck;
}
