from __future__ import annotations

import shutil
from pathlib import Path

from build_demo_gif import build_gif


ROOT = Path(__file__).resolve().parents[1]
FRAMES = ROOT / "docs" / "demo-shots"
DOCS = ROOT / "docs"


GIFS: list[tuple[str, str, int]] = [
    ("jrawful-*.png", "jrawful-gameplay.gif", 900),
    ("draw-duel-*.png", "draw-duel-gameplay.gif", 900),
    ("fake-artist-*.png", "fake-artist-gameplay.gif", 520),
    ("mafia-[0-9][0-9]-*.png", "mafia-gameplay.gif", 900),
    ("price-is-right-*.png", "price-is-right-gameplay.gif", 900),
    ("reaction-duel-*.png", "reaction-duel-gameplay.gif", 900),
    ("split-vote-*.png", "split-vote-gameplay.gif", 900),
    ("mafia-citizen-*.png", "mafia-citizen.gif", 900),
    ("mafia-mafia-*.png", "mafia-mafia.gif", 900),
    ("mafia-detective-*.png", "mafia-detective.gif", 900),
    ("mafia-doctor-*.png", "mafia-doctor.gif", 900),
    ("mafia-mayor-*.png", "mafia-mayor.gif", 900),
]

ROLE_ALIASES = [
    "mafia-citizen",
    "mafia-mafia",
    "mafia-detective",
    "mafia-doctor",
    "mafia-mayor",
]


def main() -> None:
    for pattern, filename, duration in GIFS:
        build_gif(FRAMES, pattern, DOCS / filename, width=1280, duration=duration)
        print(f"built docs/{filename}")

    for role in ROLE_ALIASES:
        src = DOCS / f"{role}.gif"
        dst = DOCS / f"{role}-gameplay.gif"
        shutil.copyfile(src, dst)
        print(f"synced docs/{dst.name}")


if __name__ == "__main__":
    main()
