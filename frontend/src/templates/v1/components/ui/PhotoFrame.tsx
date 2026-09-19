"use client";

import { useState } from "react";
import { Icon } from "./Icon";

/**
 * A picture that keeps its footprint when it cannot load: a `photos-icon` tile
 * in place of the browser's broken-image glyph. Variant URLs can 404 (the
 * media module's public variant route has been RLS-blocked before) while the
 * Attachment behind them is still valid, so a card or a composer tile must
 * degrade to "a photo is here", never to "something is broken".
 */
export function PhotoFrame({
  src,
  alt,
  className,
  fallbackClassName,
  iconSize = 28,
  loading,
  style,
}: {
  src: string;
  alt: string;
  /** Applied to the `<img>` — sizing and fit. */
  className: string;
  /** Applied to the placeholder — the same footprint, since the image's own size is gone with it. */
  fallbackClassName: string;
  iconSize?: number;
  loading?: "lazy" | "eager";
  style?: React.CSSProperties;
}) {
  const [failed, setFailed] = useState(false);

  if (failed) {
    return (
      <span
        className={`grid w-full place-items-center ${fallbackClassName}`}
        style={{ background: "var(--tpl-surface-2)", color: "var(--tpl-muted)" }}
      >
        <Icon name="photos-icon" size={iconSize} />
      </span>
    );
  }

  return (
    /* eslint-disable-next-line @next/next/no-img-element -- object URL or API-proxied variant, not a static/optimizable asset */
    <img
      src={src}
      alt={alt}
      loading={loading}
      onError={() => setFailed(true)}
      className={className}
      style={{ background: "var(--tpl-surface-2)", ...style }}
    />
  );
}
