// Canvas component for drawing. Captures pointer events into strokes,
// renders a live preview, and exposes a getStrokes() via ref for the
// parent to serialize. Trade-offs:
//   - Coordinates are normalized 0..1000 so the server's cap check (§10)
//     is independent of the client's physical canvas size.
//   - We keep stroke count + total points ≤ 5000 by rejecting new points
//     once we hit the cap; UX hint surfaces as disabled drawing.
//   - No pressure/tilt — phones can still draw, which is what matters.

import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
} from "react";
import type { Stroke, StrokesData } from "../proto";

export type CanvasHandle = {
  getStrokes: () => StrokesData;
  clear: () => void;
};

export type CanvasProps = {
  /**
   * When true, short-circuit all pointer handlers. This is the client-side
   * half of the "pencils down" feature: the leader has temporarily locked
   * drawing across all players (e.g., to give someone more time while the
   * clock is paused). The server also rejects submit_drawing inbound while
   * pencils are down — this is defense-in-depth, not the only check.
   */
  disabled?: boolean;
  color?: string;
};

const CANVAS_SIZE = 480; // render size; normalized coords are 0..1000
const MAX_POINTS = 5000;
const DEFAULT_STROKE_COLOR = "#0b0f14";
const STROKE_WIDTH = 4;

const Canvas = forwardRef<CanvasHandle, CanvasProps>(
  ({ disabled = false, color = DEFAULT_STROKE_COLOR }, ref) => {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const strokesRef = useRef<Stroke[]>([]);
  const currentRef = useRef<Stroke | null>(null);
  const [pointCount, setPointCount] = useState(0);

  useImperativeHandle(ref, () => ({
    getStrokes: () => ({ strokes: strokesRef.current }),
    clear: () => {
      strokesRef.current = [];
      currentRef.current = null;
      setPointCount(0);
      redraw();
    },
  }));

  function redraw(): void {
    const c = canvasRef.current;
    if (!c) return;
    const ctx = c.getContext("2d");
    if (!ctx) return;
    ctx.fillStyle = "#fff";
    ctx.fillRect(0, 0, c.width, c.height);
    ctx.lineJoin = "round";
    ctx.lineCap = "round";
    for (const s of strokesRef.current) {
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
  }

  useEffect(() => {
    redraw();
  });

  function pointerDown(e: React.PointerEvent<HTMLCanvasElement>): void {
    if (disabled) return;
    if (pointCount >= MAX_POINTS) return;
    const c = canvasRef.current;
    if (!c) return;
    c.setPointerCapture(e.pointerId);
    const [nx, ny] = toNorm(e, c);
    const stroke: Stroke = {
      points: [[nx, ny]],
      color,
      width: STROKE_WIDTH,
    };
    currentRef.current = stroke;
    strokesRef.current.push(stroke);
    setPointCount((p) => p + 1);
  }

  function pointerMove(e: React.PointerEvent<HTMLCanvasElement>): void {
    if (disabled) return;
    const cur = currentRef.current;
    if (!cur) return;
    if (pointCount >= MAX_POINTS) return;
    const c = canvasRef.current;
    if (!c) return;
    const [nx, ny] = toNorm(e, c);
    const last = cur.points[cur.points.length - 1];
    // Cheap distance filter to avoid thousand-point bursts from hi-DPI pads.
    const dx = nx - last[0];
    const dy = ny - last[1];
    if (dx * dx + dy * dy < 1.5) return;
    cur.points.push([nx, ny]);
    setPointCount((p) => p + 1);
    redraw();
  }

  function pointerUp(e: React.PointerEvent<HTMLCanvasElement>): void {
    const c = canvasRef.current;
    if (c) c.releasePointerCapture(e.pointerId);
    currentRef.current = null;
  }

  const capped = pointCount >= MAX_POINTS;
  const cursor = disabled ? "not-allowed" : capped ? "not-allowed" : "crosshair";
  const canvasOpacity = disabled ? 0.6 : 1;

  return (
    <div className="canvas-wrap" style={{ position: "relative" }}>
      <canvas
        ref={canvasRef}
        width={CANVAS_SIZE}
        height={CANVAS_SIZE}
        style={{
          width: "100%",
          height: "100%",
          display: "block",
          cursor,
          opacity: canvasOpacity,
          // Belt-and-suspenders: even if our onPointer* handlers didn't
          // fire for some reason, pointer-events:none guarantees no strokes
          // land while disabled. Re-enabled by un-setting the style.
          pointerEvents: disabled ? "none" : "auto",
        }}
        onPointerDown={pointerDown}
        onPointerMove={pointerMove}
        onPointerUp={pointerUp}
        onPointerCancel={pointerUp}
      />
      {disabled ? (
        <div
          style={{
            position: "absolute",
            inset: 0,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            pointerEvents: "none",
            fontSize: 18,
            fontWeight: 600,
            color: "#b23",
            background: "rgba(255,255,255,0.35)",
          }}
        >
          Pencils down
        </div>
      ) : null}
    </div>
  );
  },
);

Canvas.displayName = "Canvas";
export default Canvas;

function toNorm(e: React.PointerEvent<HTMLCanvasElement>, c: HTMLCanvasElement): [number, number] {
  const r = c.getBoundingClientRect();
  const x = ((e.clientX - r.left) / r.width) * 1000;
  const y = ((e.clientY - r.top) / r.height) * 1000;
  return [clamp(x), clamp(y)];
}

function clamp(v: number): number {
  if (v < 0) return 0;
  if (v > 1000) return 1000;
  return v;
}
