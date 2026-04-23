import type { PointerEventHandler, ReactNode } from "react";

export type TouchActionButtonProps = {
  label: string;
  active?: boolean;
  onPressStart?: PointerEventHandler<HTMLButtonElement>;
  onPressEnd?: PointerEventHandler<HTMLButtonElement>;
};

export function TouchActionButton({
  label,
  active = false,
  onPressStart,
  onPressEnd,
}: TouchActionButtonProps): JSX.Element {
  return (
    <button
      type="button"
      className={`touch-action-button${active ? " active" : ""}`}
      onPointerDown={onPressStart}
      onPointerUp={onPressEnd}
      onPointerCancel={onPressEnd}
      style={{ touchAction: "none" }}
    >
      {label}
    </button>
  );
}

export function TouchPlayfield({ children }: { children: ReactNode }): JSX.Element {
  return (
    <div className="touch-playfield" style={{ touchAction: "none", WebkitUserSelect: "none", userSelect: "none" }}>
      {children}
    </div>
  );
}

// Shared seam for the Minigame Madness follow-up work. The current playable
// games do not use these controls yet, but the touch-safe event surface is now
// centralized so future arcade-style minigames can opt in without rebuilding
// iOS-specific pointer handling from scratch.
