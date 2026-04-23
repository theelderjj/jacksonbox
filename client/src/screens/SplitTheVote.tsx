import { useState } from "react";
import { C2S } from "../proto";
import { client, selfPlayer, useGameState } from "../store";

export default function SplitTheVote(): JSX.Element {
  const g = useGameState();
  const me = selfPlayer(g);
  const isSetupPhase = g.phase.startsWith("split_setup");
  const isSplitter = !!me && (isSetupPhase ? g.myPrompt !== null : g.splitVotePrompt?.splitter_id === me.id);
  const canVote = !!me && !isSplitter;
  const [prompt, setPrompt] = useState("");
  const [optionA, setOptionA] = useState("");
  const [optionB, setOptionB] = useState("");
  const authoringMode = String(g.settings.game_options?.authoring_mode ?? "splitter_prompt_and_options");
  const targetMode = String(g.settings.game_options?.target_mode ?? "strict_split");

  if (isSetupPhase) {
    return (
      <div className="stack">
        <div className="card">
          <h2>Split the Vote Setup</h2>
          <p className="muted">
            {isSplitter
              ? "You are the splitter this round. Set the vote up and aim the room at the target."
              : "Waiting for the splitter to set the prompt and options."}
          </p>
          {me?.id && g.playerId === me.id && g.myPrompt && (
            <div className="intro-note">
              {g.myPrompt.split("\n").map((line) => (
                <div key={line}>{line}</div>
              ))}
            </div>
          )}
        </div>

        {isSplitter ? (
          <div className="card">
            {authoringMode === "splitter_prompt_and_options" && (
              <label className="stack">
                <span>Prompt</span>
                <input value={prompt} onChange={(e) => setPrompt(e.target.value)} placeholder="Ask the room something" />
              </label>
            )}
            <div className="settings-grid">
              <label className="stack">
                <span>Option A</span>
                <input value={optionA} onChange={(e) => setOptionA(e.target.value)} placeholder="First choice" />
              </label>
              <label className="stack">
                <span>Option B</span>
                <input value={optionB} onChange={(e) => setOptionB(e.target.value)} placeholder="Second choice" />
              </label>
            </div>
            <div className="row">
              <button
                className="primary"
                disabled={(authoringMode === "splitter_prompt_and_options" && prompt.trim() === "") || optionA.trim() === "" || optionB.trim() === ""}
                onClick={() =>
                  client.send(C2S.SubmitSplitSetup, {
                    prompt: prompt.trim(),
                    option_a: optionA.trim(),
                    option_b: optionB.trim(),
                  })
                }
              >
                Start vote
              </button>
            </div>
          </div>
        ) : null}
      </div>
    );
  }

  if (g.phase.startsWith("split_vote")) {
    const promptText = g.splitVotePrompt?.prompt ?? "Which option wins the room?";
    return (
      <div className="stack">
        <div className="card">
          <h2>Split the Vote</h2>
          <p className="muted">{promptText}</p>
          <p className="muted">
            Splitter: {g.splitVotePrompt?.splitter_name ?? "..."}. Goal mode: {targetMode.replaceAll("_", " ")}.
          </p>
          {!canVote && <p className="muted">You are the splitter this round, so you do not vote.</p>}
        </div>
        <div className="grid">
          <button
            className="choice split-choice"
            disabled={!canVote}
            onClick={() => client.send(C2S.SubmitSplitChoice, { choice_id: "A" })}
          >
            <strong>{g.splitVotePrompt?.option_a ?? "Option A"}</strong>
          </button>
          <button
            className="choice split-choice"
            disabled={!canVote}
            onClick={() => client.send(C2S.SubmitSplitChoice, { choice_id: "B" })}
          >
            <strong>{g.splitVotePrompt?.option_b ?? "Option B"}</strong>
          </button>
        </div>
      </div>
    );
  }

  if (g.phase.startsWith("split_reveal")) {
    return (
      <div className="stack">
        <div className="card">
          <h2>Split the Vote Reveal</h2>
          <p className="muted">{g.splitReveal?.prompt}</p>
          <p className="muted">
            {g.splitReveal?.achieved ? "Target hit." : "Target missed."}{" "}
            {g.splitReveal
              ? `${g.splitReveal.option_a}: ${g.splitReveal.count_a} • ${g.splitReveal.option_b}: ${g.splitReveal.count_b}`
              : ""}
          </p>
          <p className="muted">
            Target: {g.splitReveal?.target_a ?? 0} / {g.splitReveal?.target_b ?? 0} ({g.splitReveal?.target_mode?.replaceAll("_", " ")})
          </p>
        </div>
      </div>
    );
  }

  return <div className="card">Waiting for Split the Vote…</div>;
}
