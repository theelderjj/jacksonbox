# Jackson Box Restoration Prompt

Use this document after a branch reset to restore the intended Jackson Box platform and game behavior.

## Restoration Brief

Jackson Box should be restored as a party-game platform, not a single hardwired game. The room flow should start in a shared lobby/game picker, let the leader choose a game, configure settings, collect ready states, launch the selected game, return to a results screen, and allow the leader to choose the next game. The architecture should keep the existing Go single-writer room actor, the phase DAG engine, the shared primitives, and the protocol lockstep rule between backend and client.

The game picker should present currently playable games and planned games clearly. Jrawful, Fake Artist, Split the Vote, and Reaction Duel should be treated as real game modules. Minigame Madness should remain in the picker, but for now the focus should be on shared touch-control infrastructure and reusable minigame UI/input primitives rather than fully implementing the entire microgame list. Planned games should still have clean seams and metadata if they are not yet fully playable.

The shared platform should support future tournament and account features without implementing them yet. That means the codebase should have clear seams for playlist mode, tournament mode, invites, access grants and revocation, accounts, profile pictures, persistent stats, and head-to-head records, but none of those should be falsely presented as complete. The supporting architecture should make those additions straightforward later.

## Platform / Picker

Jackson Box should behave like a party game pack with a leader-driven lobby shell.

Required platform behavior:
- Players join a room and land in a lobby/game picker shell.
- The leader chooses the game.
- Only the leader can change game settings and start the game.
- Changing game settings should clear ready states.
- Players ready up after settings are chosen.
- The leader starts the selected game explicitly.
- After a game ends, players should see a results screen.
- The leader can choose another game from results.
- Planned / disabled games can appear in the picker, but unavailable ones must be clearly marked.
- The picker and lobby should look intentional and portfolio-worthy, not like placeholder admin UI.
- Mobile support matters throughout.

Future-facing seams to preserve:
- Tournament mode
- Playlist mode
- Winner chooses next game
- Timed game selection modes
- Account creation and login
- Invite links
- Access enable / revoke
- Profile pictures
- Stored stats
- Head-to-head win records

## Jrawful

Jrawful should remain fully playable and preserve the behavior you shaped over time.

Rules and flow:
- Players draw a short prompt.
- Other players create fake prompts for the drawings.
- Players vote for the truth.
- Reveal should happen one card at a time.
- The leader clicks next to reveal each item.
- After all reveal items are shown and points are allocated, the leader clicks to start the next round.
- Reveal should visibly highlight the true answer.
- Points should be awarded when someone picks your fake prompt.

Prompt design:
- Prompts should be shorter.
- Prompts should be just a subject combined with a verb or scene.
- Remove props / extra objects from the prompt system.
- Add early-2000s Nickelodeon / Cartoon Network / Marvel / DC heroes as subjects.
- Generate much larger prompt pools per category.

Settings explicitly requested:
- Number of rounds
- Number of additional generated fake prompts
- Drawing timer length
- Fake-prompt-writing timer length
- Voting timer length
- One reroll per drawer
- Drawing color selection for players

UI / UX requirements:
- Rules intro should be click-through, not auto-cycling.
- Do not highlight one section of the rules card.
- Banner errors should not persist into later phases or during drawing.
- The game should work on mobile-sized windows.

## Fake Artist

This should be implemented exactly around the rules you described.

Core rules:
- Everyone gets the prompt except the fake artist.
- One fake artist is chosen secretly.
- Players are randomly selected one at a time to draw.
- Before each drawer starts, show a 3-second countdown.
- Show the current drawer’s name at the top of the canvas.
- Everyone should see the current drawer adding to the shared drawing live.
- After all turns, replay the drawing before voting.
- Replay should show who was drawing at each point in the recording.
- Voting time is also the discussion time.
- Players vote for who they think is the fake artist.
- If the majority votes for the wrong person, the fake artist wins.
- If the majority correctly identifies the fake artist, the fake artist gets one chance to guess the prompt.
- If the fake artist guesses correctly, the fake artist wins.
- If the fake artist guesses incorrectly, the true artists win.

Settings explicitly requested:
- Draw time length per drawer
- Voting / discussion time length
- Number of rounds up to 60
- Option to color-code each drawer with a unique color
- Number of replays, or continuous replay during voting

Presentation requirements:
- Dedicated gameplay spec
- Dedicated gameplay GIF
- Dedicated E2E spec

## Split the Vote

Split the Vote needs multiple authoring and scoring modes, and target visibility rules matter.

Core rules:
- The room votes between two options.
- Scoring depends on how close the result is to the target split.
- In variable-target mode, the target should only be visible to the splitter, not the whole room.
- Variable targets must always be achievable with the number of players in the room.

Requested modes:
- `strictly split`: target is always 50/50
- `variable`: target varies based on player count
- `get the odd one`: make the room as lopsided as possible while still having at least one vote on one side

Additional setting:
- Prompt/option creation mode should be configurable:
  - game generates a prompt and two vote options
  - or the splitter creates a prompt and the two options

So Split the Vote needs both:
- target mode
- prompt/option authoring mode

Special requirement:
- The group should not see the target in variable mode if that mode is supposed to be secret to the splitter.

## Reaction Duel

Core rules:
- Leader can configure how long the visible countdown lasts before the duel is armed.
- During countdown the button is red.
- After the countdown, there is a random wait from 1 to 8 seconds.
- Then the button turns green.
- If you press while red, you lose / false start.
- Fastest valid green press wins.

Requirements:
- Clear mobile/touch-safe button behavior
- Visual clarity between countdown/red/green/live states

## Word Storm

Core rules and expectations:
- Accept dictionary-valid words.
- Show valid submitted words in results.
- Explain scoring clearly.
- Score by points per letter.

## Price is Right

Core rules:
- Show a random Amazon product image.
- Players guess the price.
- Closest without going over wins.

Requirements:
- Product image should be visible in gameplay
- The rule should be explicit in UI and tests

## Draw Duel

Core requirements:
- Timer setting
- Different drawing colors
- Different line weight settings

General gameplay:
- Two artists draw head-to-head
- Others judge / vote

## Minigame Madness

Hold off on implementing the full set of minigames for now, but build the shared touch-control infrastructure based on the Minigame-Madness doc:

[Minigame-Madness.docx](</C:/Users/johnj/Downloads/Minigame-Madness.docx>)

Build now:
- Mobile-first touch input layer
- Large tap-target primitives
- Shared minigame board/container
- Touch-safe press handling
- Common timer / result / feedback scaffolding
- Shared client action patterns for tap-based minigames
- Reusable visual styling / sprite handling pipeline
- Handicap/lives/match-length/survival infrastructure at the mode level

Do not build every microgame from the doc yet unless explicitly requested.

Minigame Madness mode requirements:
- Players challenge each other in a match of X randomized games
- All players start the same mini game at the same time
- IOS touch controls should be a design target
- Styling should stay consistent
- Settings should include:
  - number of games
  - survival mode
  - lives
  - handicap settings

## Other Games Mentioned Earlier

Preserve these as future seams or partial specs:
- Mafia
- Blitz Quiz
- Word Storm
- Reaction Duel
- Split the Vote
- Fake Artist
- Draw Duel
- Chain Reaction
- Price is Right
- Memory Grid
- Debate Club
- Blind Artist
- Riddle Rush
- Emoji Charades
- Speed Math
- Silent Disco

For Chain Reaction:
- Last-letter word chain
- 5 seconds per turn
- Validate last-letter matching
- Detect repeats
- Turn-based elimination behavior

For Price is Right:
- Closest without going over
- Tie handling
- Amazon product image requirement

## Testing / Asset Requirements

- Unit tests for new game logic, settings validation, and scoring.
- Integration tests for real actor/game flow transitions.
- Playwright E2E for each playable game.
- Readable Playwright output with `test.step`.
- Dedicated specs where asked for, especially Fake Artist.
- Gameplay GIF generation for currently playable games.
- Demo shots for all currently working games.
- Fake Artist GIF restoration specifically.

## Master Implementation Prompt

Use the following prompt after reset:

```text
You are working in the Jackson Box repo.

Read AGENTS.md and docs/DESIGN.md first. Preserve the architecture:
- Go backend with single-writer room actor in internal/room/room.go
- Phase DAG engine in internal/engine/
- Shared primitives in internal/primitives/
- Client is Vite/React/TypeScript strict
- Protocol lockstep is mandatory:
  1. internal/proto/messages.go
  2. internal/proto/validate.go
  3. client/src/proto.ts
  4. client/src/ws.ts
  5. client/src/store.ts

Goal:
Restore Jackson Box as a party-game platform with a leader-driven picker/lobby and make the intended game behavior match the product rules below. Prioritize Jrawful, Fake Artist, Split the Vote, Reaction Duel, and Minigame Madness infrastructure. Do not overclaim unfinished features.

GENERAL PLATFORM REQUIREMENTS

Room / picker flow:
- Players join room and land in a shared lobby / game picker shell.
- Leader chooses the game.
- Leader configures settings.
- Players ready up.
- Leader explicitly starts the game.
- On game end, show a shared results screen.
- Leader can choose another game.
- Planned games may appear in picker but must be visibly disabled / coming soon if not implemented.
- UI must be mobile-friendly and portfolio-worthy.

Leader control rules:
- Only leader can change game settings.
- Only leader can start the game.
- Changing settings clears ready states.
- Joining mid-room should render current mode and current game state correctly.
- Preserve reconnect behavior.

Future seams only, not full implementation:
- tournament mode
- playlist mode
- winner chooses next game
- accounts
- profile pictures
- invites
- access grants / revocation
- persistent stats
- head-to-head records

Do not claim future seams are implemented unless they truly are.

JRAWFUL REQUIREMENTS

Keep Jrawful fully playable and preserve these rules:
- Players draw a prompt.
- Others create fake prompts.
- Players vote for the truth.
- Points are scored when someone picks your fake prompt.
- Reveal is one card at a time.
- Leader clicks next to reveal each item.
- After all reveal items and points are allocated, leader clicks to start the next round.
- True answer must be visibly highlighted on reveal.
- Must work on mobile-sized windows.

Jrawful intro / UI:
- Rules intro should be click-through, not auto-cycling.
- Do not visually highlight one rules section.
- Banner errors should not persist into later rounds or show during drawing.
- Allow player color selection for drawing.

Jrawful prompt system:
- Prompts should be shorter.
- Format should be subject + verb OR subject + scene.
- Remove props / extra objects from prompt generation.
- Add common early-2000s Nickelodeon, Cartoon Network, Marvel, and DC heroes as subjects.
- Expand prompt pools heavily.

Jrawful settings:
- round_count
- generated_fake_count
- drawing_seconds
- fake_prompt_seconds
- voting_seconds
- one reroll per drawer

Jrawful phase expectations:
- lobby/settings
- prompt distribution
- drawing submit
- fake prompt submit
- voting
- reveal
- scoring / round result
- final result / game end

FAKE ARTIST REQUIREMENTS

Implement Fake Artist exactly around these rules.

Core rules:
- Everyone gets the prompt except the fake artist.
- Randomly choose one fake artist.
- Randomly choose the drawing order each round.
- One player draws at a time on the shared drawing.
- Before each drawer starts, show a 3-second countdown.
- Show the current drawer’s name at the top of the canvas.
- Everyone sees the current drawer adding to the shared drawing live.
- After all drawers finish, replay the drawing before voting.
- Replay must show which drawer contributed at each point / segment.
- Voting time is also discussion time.
- Players vote for the fake artist.
- If majority votes the wrong player, fake artist wins.
- If majority correctly votes the fake artist, fake artist gets one guess at the prompt.
- If fake artist guesses correctly, fake artist wins.
- If fake artist guesses incorrectly, the true artists win.

Fake Artist settings:
- fake_artist_rounds up to 60
- drawing_seconds per drawer
- voting_seconds
- fake_artist_palette toggle for unique drawer colors
- fake_artist_replays count
- continuous replay option during voting if that is cleaner than a finite replay count implementation

Fake Artist phase structure:
- fakeartist_intro_rN
  - privately send prompt to true artists
  - privately send “you are the fake artist” to fake artist
  - show round overview
- fakeartist_countdown_rN_tM
  - show current drawer name
  - show countdown
  - no drawing input allowed
- fakeartist_draw_rN_tM
  - only active drawer can send draw input
  - all clients see shared canvas update live
- fakeartist_replay_rN
  - play back assembled turn-by-turn drawing
  - label each drawer’s contribution
- fakeartist_vote_rN
  - allow voting for any player
  - replay may continue if continuous mode is enabled
- fakeartist_guess_rN
  - only fake artist can guess
  - skip or auto-resolve if fake artist was not correctly identified
- fakeartist_score_rN
- fakeartist_result_rN

Fake Artist implementation instructions:
- Use shared drawing/canvas infra where possible.
- Reuse submit_action if practical for live turn drawing and final guess.
- Store turn-by-turn drawing data so replay can reconstruct contribution order.
- Expose dedicated client state payloads, not ad hoc reuse of Jrawful payloads if it becomes messy.
- Add dedicated E2E spec.
- Add gameplay GIF capture.

SPLIT THE VOTE REQUIREMENTS

Implement Split the Vote with multiple modes and authoring controls.

Base rules:
- Room votes between two options.
- Score based on how close the result is to the target split.

Target rules:
- Variable target must always be achievable with current player count.
- In secret-target variable mode, only the splitter sees the target, not the whole room.

Split the Vote game modes:
- strict
  - target is always 50/50
- variable
  - target depends on player count and must be achievable
- odd_one
  - objective is to make the room as lopsided as possible while still leaving at least one vote on one side

Split the Vote authoring mode setting:
- generated
  - game generates prompt and both options
- splitter_authored
  - splitter writes prompt and the two options

Split the Vote settings:
- split_vote_mode = strict | variable | odd_one
- split_vote_authoring_mode = generated | splitter_authored
- any target visibility setting needed to support secret target logic cleanly

Split the Vote phase structure:
- splitvote_prompt_rN
  - generate or collect prompt/options depending on authoring mode
  - assign target
  - only show target to the splitter if mode requires secrecy
- splitvote_vote_rN
  - collect votes from all players
- splitvote_score_rN
- splitvote_result_rN

Split the Vote implementation instructions:
- If splitter_authored mode exists, define who the splitter is each round and how authorship rotates.
- If target is secret, use targeted server events or equivalent per-player state delivery.
- Keep target math deterministic and testable.
- Add tests for target achievability and target secrecy.

REACTION DUEL REQUIREMENTS

Core rules:
- Leader controls visible countdown seconds before the duel is armed.
- During countdown the button is red.
- After countdown there is a random 1–8 second wait.
- Then button turns green.
- Pressing while red is a false start / loss.
- Fastest valid press after green wins.

Reaction Duel settings:
- visible_countdown_seconds
- round_count if multi-round
- any existing reaction settings that support repeated rounds cleanly

Reaction Duel phase structure:
- reactionduel_prompt_rN
  - explain rules / get ready
- reactionduel_countdown_rN
  - visible countdown, red button
- reactionduel_wait_rN
  - random 1–8 second red wait
- reactionduel_go_rN
  - green / live tap window
- reactionduel_score_rN
- reactionduel_result_rN

Reaction Duel implementation instructions:
- Mobile touch handling must be robust.
- False starts must be clearly visible in results and/or live UI.
- Keep timing server-authoritative.

MINIGAME MADNESS REQUIREMENTS

Do NOT fully implement all minigames now.
Do build the reusable touch-control infrastructure for the minigames described in:
C:/Users/johnj/Downloads/Minigame-Madness.docx

Minigame Madness mode requirements:
- Players challenge each other in a match of X randomized games.
- All players start the same minigame at the same time.
- IOS/touch-first controls are a design target.
- Consistent visual styling and sprite handling.
- Support lives, handicap, number of games, and survival mode.

Build now:
- shared minigame board/container
- large tap-target components
- touch-safe pointer/touch input handling
- reusable timing UI
- shared action submission pattern
- lives / handicap / survival mode config infrastructure
- synchronized round start shell
- reusable result cards and feedback patterns

Do not fully build every microgame from the doc unless explicitly asked later.

WORD STORM REQUIREMENTS TO PRESERVE

Keep these as implementation requirements when Word Storm is restored:
- Accept dictionary words.
- Show valid submitted words.
- Explain scoring clearly.
- Score by points per letter.

PRICE IS RIGHT REQUIREMENTS TO PRESERVE

Keep these as implementation requirements when Price is Right is restored:
- Show photo of a random Amazon product.
- Players guess price.
- Closest without going over wins.

DRAW DUEL REQUIREMENTS TO PRESERVE

Keep these as implementation requirements when Draw Duel is restored:
- timer setting
- color options
- line weight options
- head-to-head drawing plus judging/voting flow

TESTING REQUIREMENTS

Unit tests:
- game rules
- scoring math
- settings validation
- any secret-target delivery logic
- prompt/role assignment logic
- replay assembly logic where applicable

Integration tests:
- use real actor + gateway test conns
- picker / leader control flow
- ready state clearing on settings change
- start-game flow from lobby
- full round path for each implemented game
- result transition back to shared results/game-end state

Required integration coverage:
- Jrawful full round still works
- Fake Artist full round works
- Split the Vote round works in key modes
- Reaction Duel round works
- Minigame Madness infrastructure shell launches correctly

Playwright E2E:
- picker/lobby flow
- Jrawful full game
- Fake Artist dedicated spec
- Split the Vote dedicated spec
- Reaction Duel dedicated spec
- Minigame Madness shell / touch infrastructure spec if meaningful
- readable output with test.step

Demo/GIF:
- reproducible screenshot capture flow
- gameplay GIFs for:
  - Jrawful
  - Fake Artist
  - Split the Vote
  - Reaction Duel
  - Minigame Madness only if the current shell/infrastructure is visually meaningful
- Do not let README capture cleanup delete the per-game demo-shot set

SETTINGS MATRIX TO IMPLEMENT

Platform:
- game_mode
- leader-only game start
- ready reset on settings changes

Jrawful:
- round_count
- generated_fake_count
- drawing_seconds
- fake_prompt_seconds
- voting_seconds
- reroll once per drawer
- drawing color selection

Fake Artist:
- fake_artist_rounds
- drawing_seconds per drawer
- voting_seconds
- fake_artist_palette
- fake_artist_replays
- continuous replay mode if implemented

Split the Vote:
- split_vote_mode = strict | variable | odd_one
- split_vote_authoring_mode = generated | splitter_authored
- secret/public target delivery as needed

Reaction Duel:
- visible_countdown_seconds
- round_count if needed

Minigame Madness:
- game_count
- survival_mode
- lives
- handicap

IMPLEMENTATION ORDER

1. Rebuild / preserve shared picker and leader-driven start flow.
2. Restore Jrawful behavior and settings.
3. Implement Fake Artist fully.
4. Tighten Split the Vote modes and authoring settings.
5. Tighten Reaction Duel countdown/random-wait rules.
6. Build Minigame Madness touch infrastructure only.
7. Restore/update tests.
8. Restore/update gameplay capture and GIF generation.
9. Update README to reflect current playable games honestly.

VERIFICATION

Before final response run:
- go test ./cmd/... ./internal/...
- cd client && npm test -- src/store.test.ts src/ws.test.ts
- cd client && npm run typecheck
- cd client && npm run e2e
- rebuild gameplay GIFs and confirm files exist

Do not use destructive git commands.
Do not claim unfinished games or future seams are complete.
Keep protocol lockstep whenever wire messages change.
```
