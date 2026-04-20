import { useEffect, useState, type ReactNode } from "react";

export type RulesIntroSlide = {
  id: string;
  eyebrow: string;
  title: string;
  body: string;
  points: string;
  notes: string[];
  scene: ReactNode;
};

export type RulesIntroDeck = {
  gameId: string;
  gameName: string;
  previewTitle: string;
  slides: readonly RulesIntroSlide[];
};

type RulesIntroProps = {
  deck: RulesIntroDeck;
};

export default function RulesIntro({ deck }: RulesIntroProps): JSX.Element {
  const [active, setActive] = useState(0);
  const slides = deck.slides;

  useEffect(() => {
    setActive(0);
  }, [deck.gameId]);

  const slide = slides[active]!;
  const canGoBack = active > 0;
  const canGoNext = active < slides.length - 1;

  return (
    <div className="card intro-shell" data-game-id={deck.gameId}>
      <div className="intro-copy">
        <div className="intro-eyebrow">{slide.eyebrow}</div>
        <h3>{slide.title}</h3>
        <p className="muted">{slide.body}</p>
        <div className="intro-points">{slide.points}</div>
        <div className="stack" style={{ gap: 8 }}>
          {slide.notes.map((note) => (
            <div key={note} className="intro-note">
              {note}
            </div>
          ))}
        </div>
        <div className="intro-nav">
          <button type="button" disabled={!canGoBack} onClick={() => setActive((prev) => prev - 1)}>
            Back
          </button>
          <div className="intro-dots" aria-label={`${deck.gameName} rules intro progress`}>
            {slides.map((item, idx) => (
              <span
                key={item.id}
                className={`intro-dot${idx === active ? " active" : ""}`}
                aria-label={`${item.eyebrow}${idx === active ? " current" : ""}`}
              />
            ))}
          </div>
          <button
            type="button"
            className="primary"
            disabled={!canGoNext}
            onClick={() => setActive((prev) => prev + 1)}
          >
            Next
          </button>
        </div>
      </div>

      <div className="intro-stage" aria-hidden="true">
        {slides.map((item, idx) => (
          <div key={item.id} className={`intro-scene${idx === active ? " active" : ""}`}>
            {item.scene}
          </div>
        ))}
      </div>
    </div>
  );
}
