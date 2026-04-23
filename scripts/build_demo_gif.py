from __future__ import annotations

import argparse
from pathlib import Path

from PIL import Image


def build_gif(frames_dir: Path, pattern: str, output: Path, width: int, duration: int) -> None:
    frame_paths = sorted(frames_dir.glob(pattern))
    if not frame_paths:
        raise SystemExit(f"No frames matched {pattern} in {frames_dir}")

    prepared: list[Image.Image] = []
    max_height = 0
    for frame_path in frame_paths:
        image = Image.open(frame_path).convert("RGBA")
        ratio = width / image.width
        resized = image.resize((width, int(image.height * ratio)), Image.Resampling.LANCZOS)
        prepared.append(resized)
        max_height = max(max_height, resized.height)

    padded: list[Image.Image] = []
    for image in prepared:
        canvas = Image.new("RGBA", (width, max_height), (10, 15, 20, 255))
        top = (max_height - image.height) // 2
        canvas.alpha_composite(image, (0, top))
        padded.append(canvas.convert("P", palette=Image.Palette.ADAPTIVE))

    output.parent.mkdir(parents=True, exist_ok=True)
    padded[0].save(
        output,
        save_all=True,
        append_images=padded[1:],
        duration=duration,
        loop=0,
        optimize=False,
        disposal=2,
    )


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--frames-dir", type=Path, default=Path("docs/demo-shots"))
    parser.add_argument("--pattern", required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--width", type=int, default=1280)
    parser.add_argument("--duration", type=int, default=900)
    args = parser.parse_args()
    build_gif(args.frames_dir, args.pattern, args.output, args.width, args.duration)


if __name__ == "__main__":
    main()
