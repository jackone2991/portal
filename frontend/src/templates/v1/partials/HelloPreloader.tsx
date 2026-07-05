"use client";

import { useEffect, useState } from "react";

/**
 * Loading overlay — port of `partials/hellopreloader.blade.php`.
 * Shown on first paint, removed once the client mounts.
 */
export function HelloPreloader() {
  const [done, setDone] = useState(false);

  useEffect(() => {
    const t = setTimeout(() => setDone(true), 300);
    return () => clearTimeout(t);
  }, []);

  if (done) return null;

  return (
    <div
      id="hellopreloader"
      className="fixed inset-0 z-[100] grid place-items-center"
      style={{ background: "var(--background, #0b0b10)" }}
    >
      <div className="animate-pulse text-sm" style={{ color: "var(--tpl-muted)" }}>
        Loading …
      </div>
    </div>
  );
}
