// DrawingView renders a submitted drawing (strokes or base64 PNG) read-only.
// Mirrors Canvas's stroke rendering so dimensions match — coords are still
// normalized 0..1000 and we scale them to whatever our on-screen canvas size is.

import { useEffect, useRef } from "react";
import type { StrokesData } from "../proto";

type Props = {
  data: string;
  format: "strokes" | "png";
  size?: number;
};

const DEFAULT_SIZE = 280;

export default function DrawingView({ data, format, size = DEFAULT_SIZE }: Props): JSX.Element {
  const ref = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const c = ref.current;
    if (!c) return;
    const ctx = c.getContext("2d");
    if (!ctx) return;

    ctx.fillStyle = "#fff";
    ctx.fillRect(0, 0, c.width, c.height);

    if (format === "strokes") {
      let strokes: StrokesData | null = null;
      try {
        strokes = JSON.parse(data) as StrokesData;
      } catch {
        return;
      }
      ctx.lineJoin = "round";
      ctx.lineCap = "round";
      for (const s of strokes.strokes) {
        ctx.strokeStyle = s.color;
        ctx.lineWidth = s.width * (c.width / 1000);
        ctx.beginPath();
        for (let i = 0; i < s.points.length; i++) {
          const [nx, ny] = s.points[i];
          const x = (nx / 1000) * c.width;
          const y = (ny / 1000) * c.height;
          if (i === 0) ctx.moveTo(x, y);
          else ctx.lineTo(x, y);
        }
        ctx.stroke();
      }
      return;
    }

    // PNG fallback — allowed by §10 for future extensibility.
    const img = new Image();
    img.onload = () => ctx.drawImage(img, 0, 0, c.width, c.height);
    img.src = data.startsWith("data:") ? data : `data:image/png;base64,${data}`;
  }, [data, format, size]);

  return (
    <div className="canvas-wrap" style={{ width: size, height: size }}>
      <canvas ref={ref} width={size} height={size} style={{ width: "100%", height: "100%", display: "block" }} />
    </div>
  );
}
