export type GuideSection = {
  title: string;
  body: string;
  bullets: string[];
};

export type GameLobbyGuide = {
  title: string;
  intro: string;
  sections: GuideSection[];
  footer?: string;
};

export type MafiaRoleGuide = {
  roleId: string;
  name: string;
  goal: string;
  night: string;
  day: string;
  tip: string;
};

export const mafiaRoleGuides: MafiaRoleGuide[] = [
  {
    roleId: "mafia",
    name: "Mafia",
    goal: "Win when Mafia equals or outnumbers the town.",
    night: "Choose one living non-Mafia player to eliminate. Coordinate with your Mafia teammate when there is more than one.",
    day: "Blend in, redirect suspicion, nominate town players, and avoid being voted out.",
    tip: "Act like a helpful town member. Push believable suspicions instead of defending too hard.",
  },
  {
    roleId: "detective",
    name: "Detective",
    goal: "Help the town find every Mafia member.",
    night: "Investigate one living player. You learn whether they are Mafia or Town, or their exact role if that setting is enabled.",
    day: "Use your information carefully. Reveal too early and Mafia may target you; wait too long and the town may vote wrong.",
    tip: "Track your results privately and guide nominations without immediately exposing yourself.",
  },
  {
    roleId: "doctor",
    name: "Doctor",
    goal: "Keep town power roles and trusted players alive.",
    night: "Protect one living player. If Mafia attacks that player, the kill fails. Self-protect depends on the room setting.",
    day: "Help the town reason through deaths and saves without making yourself an obvious night target.",
    tip: "Protect likely Mafia targets, not just the loudest player. A successful save can swing the game.",
  },
  {
    roleId: "mayor",
    name: "Mayor",
    goal: "Use your voting power to help town eliminate Mafia.",
    night: "You sleep at night and do not take a private action.",
    day: "Your vote counts as three. In mayor-breaks-tie mode, you may be the only player who can resolve a tied vote.",
    tip: "Stay alive, listen carefully, and use your weighted vote when the room is split.",
  },
  {
    roleId: "citizen",
    name: "Citizen",
    goal: "Find and vote out the Mafia using discussion and voting.",
    night: "You sleep at night and receive no private information.",
    day: "Ask questions, compare stories, nominate suspicious players, and vote with the town.",
    tip: "You have no power role, so your strength is reading behavior and protecting confirmed town players.",
  },
];

const fallbackMafiaRoleGuide: MafiaRoleGuide = {
  roleId: "unknown",
  name: "your role",
  goal: "Learn your role and help your team win.",
  night: "Follow the prompt shown during the night phase.",
  day: "Discuss, nominate, and vote based on the information you have.",
  tip: "Keep your role private unless revealing it helps your team.",
};

const gameLobbyGuides: Record<string, GameLobbyGuide> = {
  jrawful: {
    title: "Draw, bluff, vote",
    intro: "Jrawful works best when everyone keeps their prompts secret, draws quickly, and writes fake answers that sound just plausible enough to trap a friend.",
    sections: [
      {
        title: "1. Draw",
        body: "Each round starts with a secret prompt. Sketch it before the drawing timer runs out.",
        bullets: [
          "Clear beats pretty.",
          "Your drawing becomes the bait for every fake answer.",
        ],
      },
      {
        title: "2. Bluff and vote",
        body: "For drawings that are not yours, write believable fake prompts. Then vote for the real prompt when all choices appear.",
        bullets: [
          "You score when someone picks your fake prompt.",
          "You also score for finding the real answer yourself.",
        ],
      },
      {
        title: "3. Score",
        body: "The reveal shows who fooled whom, highlights the true answer, and awards drawer points for true votes.",
        bullets: [
          "Fake picks pay the faker.",
          "True votes pay both the guesser and the drawer.",
        ],
      },
    ],
    footer: "Ready up once the timers and round count feel right. The host starts the game after everyone is ready.",
  },
  mafia: {
    title: "Survive the lies",
    intro: "Mafia alternates between hidden night actions and public daytime debate. Town wins by eliminating every Mafia member; Mafia wins when they match or outnumber town.",
    sections: [
      {
        title: "Night",
        body: "Alive players with night powers act in secret. Resolution always runs in order: protect, kill, then investigate.",
        bullets: [
          "Doctor protection can cancel one Mafia kill.",
          "Detective always gets a result, even if the target dies that night.",
        ],
      },
      {
        title: "Day",
        body: "Everyone discusses, nominates suspicious players, and then votes publicly.",
        bullets: [
          "Tie handling follows the room setting.",
          "The mayor has extra voting power when assigned.",
        ],
      },
      {
        title: "Win condition",
        body: "After every death, the game checks whether all Mafia are gone or whether Mafia now equal or outnumber town.",
        bullets: [
          "Dead players are out of chat and actions.",
          "Role reveal on death depends on the lobby setting.",
        ],
      },
    ],
    footer: "Use the role buttons below to preview every role before the host starts the match.",
  },
  fake_artist: {
    title: "Spot the faker",
    intro: "Everyone except the fake artist knows the prompt. Players take turns adding to the same drawing, then the room watches the replay and votes on who was faking it.",
    sections: [
      {
        title: "Turns",
        body: "Each drawer gets a visible countdown, then a short drawing window. Everyone sees who is drawing and what they add.",
        bullets: [
          "Color coding can assign each drawer a unique color.",
          "The shared canvas keeps building across turns.",
        ],
      },
      {
        title: "Replay and vote",
        body: "Before voting, the game replays the full drawing from start to finish so the room can discuss suspicious turns.",
        bullets: [
          "Replay count and continuous replay are host settings.",
          "Voting time doubles as discussion time.",
        ],
      },
      {
        title: "Outcome",
        body: "If the room misses the fake artist, the fake wins. If the room finds them, the fake gets one last prompt guess for the steal.",
        bullets: [
          "Correct fake guess means fake artist still wins.",
          "Wrong fake guess means the true artists win together.",
        ],
      },
    ],
  },
  split_vote: {
    title: "Manipulate the room",
    intro: "One player acts as the splitter and creates or finishes the choices. Everyone else votes, and the score depends on how close the final distribution lands to the mode's target.",
    sections: [
      {
        title: "Authoring",
        body: "Depending on settings, the splitter either writes the full prompt and both options or fills in options for a generated prompt.",
        bullets: [
          "The splitter does not vote.",
          "Only the splitter sees the variable target when that mode is enabled.",
        ],
      },
      {
        title: "Target modes",
        body: "Strict split aims for 50-50. Variable chooses an achievable target for the player count. Odd one rewards making the vote as lopsided as possible with at least one vote on one side.",
        bullets: [
          "Targets always respect the number of eligible voters.",
          "The rest of the room only sees the prompt and options.",
        ],
      },
      {
        title: "Scoring",
        body: "Closer distributions score better. The reveal shows the counts, the achieved ratio, and how much the round paid out.",
        bullets: [
          "A perfect split is ideal in strict mode.",
          "Odd one mode wants one lonely vote and a giant pile on the other side.",
        ],
      },
    ],
  },
  reaction_duel: {
    title: "Wait for green",
    intro: "Reaction Duel is all nerve. The button stays red during the visible countdown, then a hidden random delay decides when it turns green.",
    sections: [
      {
        title: "Countdown",
        body: "The host chooses how long the visible countdown lasts before the random wait begins.",
        bullets: [
          "The red button is a trap.",
          "The green light can arrive anytime in the random window.",
        ],
      },
      {
        title: "Tap rules",
        body: "Pressing red counts as a false start and loses the duel. Pressing after green records your reaction time.",
        bullets: [
          "Server timing decides the winner.",
          "Both players' reaction times are shown after the round.",
        ],
      },
      {
        title: "Win",
        body: "Fastest legal tap wins the round and the reveal makes the timing difference obvious.",
        bullets: [
          "Patience is part of the game.",
          "One greedy tap can throw the entire round.",
        ],
      },
    ],
  },
  price_is_right: {
    title: "Closest without going over",
    intro: "Each round shows a real product image and a price floor. Everyone guesses, but only prices at or under the true value can win.",
    sections: [
      {
        title: "Product reveal",
        body: "A product card appears with the item image, title, and the minimum price threshold for that round.",
        bullets: [
          "The threshold can stay fixed or scale upward by round.",
          "Rounds can jump into wildly more expensive categories.",
        ],
      },
      {
        title: "Guessing",
        body: "Submit one price guess before the timer ends. Overbids are out immediately.",
        bullets: [
          "Closest under wins.",
          "A safe guess can beat an overconfident one.",
        ],
      },
      {
        title: "Reveal",
        body: "The round result shows the true price, who stayed under, and who took the point.",
        bullets: [
          "Going over scores nothing.",
          "Consistent underbids can win a full game.",
        ],
      },
    ],
  },
  word_storm: {
    title: "Beat the dictionary",
    intro: "A letter stays live for a short window, then the game rotates to the next one. Players race to claim valid words before anyone else does.",
    sections: [
      {
        title: "Claim words fast",
        body: "Enter dictionary words that begin with the current letter. The first player to claim a word locks it for the rest of the game.",
        bullets: [
          "Duplicate claims are rejected as already entered.",
          "Wrong-letter entries never lock the word.",
        ],
      },
      {
        title: "Tentative scoring",
        body: "Words show provisional acceptance during play, but the official cleanup happens at the end of the match.",
        bullets: [
          "Words longer than four letters gain 15% more value per extra letter.",
          "The leaderboard becomes official only after validation.",
        ],
      },
      {
        title: "Final cleanup",
        body: "End-of-game scoring crosses out invalid or made-up words, counts removed entries, and totals each player's final score.",
        bullets: [
          "Made-up words can still block later duplicate attempts if they were claimed first.",
          "Only validated words survive the final scoreboard.",
        ],
      },
    ],
  },
  draw_duel: {
    title: "Artists versus judges",
    intro: "Each round picks two artists and at least one judge. The artists draw the same prompt, then the rest of the room decides whose sketch wins.",
    sections: [
      {
        title: "Draw",
        body: "The host sets the draw timer, and artists can use different colors and line weights while they race the clock.",
        bullets: [
          "A short timer rewards a strong silhouette.",
          "A longer timer gives room for style.",
        ],
      },
      {
        title: "Judge",
        body: "Non-artists become judges and vote for the stronger drawing after both submissions are in.",
        bullets: [
          "Artists do not judge their own duel.",
          "You need at least three players so someone can judge.",
        ],
      },
      {
        title: "Rounds",
        body: "The match can run multiple rounds so more players rotate into the artist seats.",
        bullets: [
          "Judges decide the winner each round.",
          "The overall leader is whoever stacks the most round wins.",
        ],
      },
    ],
  },
};

export function getLobbyGuide(gameId: string | null | undefined): GameLobbyGuide {
  return gameLobbyGuides[gameId ?? ""] ?? gameLobbyGuides.jrawful;
}

export function getMafiaRoleGuide(role: string | undefined): MafiaRoleGuide {
  return mafiaRoleGuides.find((entry) => entry.roleId === role) ?? fallbackMafiaRoleGuide;
}
