// Countdown renders a deadline-driven seconds counter. Intentionally dumb:
// consumes an absolute epoch ms, re-renders every 250ms while >0, freezes at
// "0s" once elapsed. The source of truth is the server's deadline; the client
// never computes phase length on its own.
//
// Pause support: when `paused` is true, we ignore the ticking wall clock and
// render the server-provided `remainingMs` verbatim (prefixed with "⏸"). The
// store clears `remainingMs` back to null on resume, so re-hydration is
// handled by a re-render when pause_state echoes false.

import { useEffect, useState } from "react";

type Props = {
  deadlineMs: number | null;
  paused?: boolean;
  remainingMs?: number | null;
};

export default function Countdown({ deadlineMs, paused = false, remainingMs = null }: Props): JSX.Element | null {
  const [now, setNow] = useState(Date.now());

  useEffect(() => {
    // No ticking while paused — the rendered value is server-driven.
    if (paused) return;
    if (deadlineMs == null) return;
    const tick = window.setInterval(() => setNow(Date.now()), 250);
    return () => window.clearInterval(tick);
  }, [deadlineMs, paused]);

  if (paused) {
    const frozen = remainingMs != null ? Math.max(0, remainingMs) : 0;
    const seconds = Math.ceil(frozen / 1000);
    return (
      <span className="pill" style={{ fontVariantNumeric: "tabular-nums" }}>
        ⏸ {seconds}s
      </span>
    );
  }

  if (deadlineMs == null) return null;
  const ms = Math.max(0, deadlineMs - now);
  const seconds = Math.ceil(ms / 1000);
  const urgent = ms <= 5_000;

  return (
    <span className={`pill ${urgent ? "offline" : ""}`} style={{ fontVariantNumeric: "tabular-nums" }}>
      {seconds}s
    </span>
  );
}
