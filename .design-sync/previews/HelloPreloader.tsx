import { useEffect, useState } from "react";
import { HelloPreloader } from "portal-frontend";

// First-paint loading overlay. Real usage is `fixed inset-0` covering the whole
// viewport, then it unmounts (returns null) ~300ms after mount. Two problems for
// a static preview: (1) the fixed overlay escapes the card, (2) it self-dismisses
// on a timer, so a capture taken after 300ms lands on a blank box.
//   Fix (1): a relative wrapper with a `transform` contains `fixed inset-0` to
//            this box instead of the full page.
//   Fix (2): a keyed remount loop re-mounts the preloader faster than its own
//            300ms dismiss timer, so `done` never flips — the "Loading …" state
//            stays on screen for the capture. This shows the component's real
//            markup (dark backdrop + pulsing label), just held open.

export const Overlay = () => {
  const [tick, setTick] = useState(0);
  useEffect(() => {
    const id = setInterval(() => setTick((t) => t + 1), 120);
    return () => clearInterval(id);
  }, []);

  return (
    <div
      style={{
        position: "relative",
        transform: "translateZ(0)",
        width: 300,
        height: 240,
        overflow: "hidden",
        borderRadius: 12,
        border: "1px solid var(--tpl-border, #e6e6ef)",
      }}
    >
      <HelloPreloader key={tick} />
    </div>
  );
};
