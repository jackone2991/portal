import type { TemplateManifest } from "./types";
import { v1 } from "./v1";

/**
 * Registry of all frontend template versions.
 *
 * This is the SINGLE switch point for the whole UI. To ship a redesign:
 *   1. Create `templates/v2/` implementing `TemplateManifest`.
 *   2. Add `v2` to REGISTRY below.
 *   3. Set `NEXT_PUBLIC_TEMPLATE_VERSION=v2` (and swap the theme import in
 *      `app/layout.tsx`).
 * URLs and `app/` route files never change — they resolve through this registry.
 */
const REGISTRY = { v1 } satisfies Record<string, TemplateManifest>;

export type TemplateVersion = keyof typeof REGISTRY;

/** Active version. Override with NEXT_PUBLIC_TEMPLATE_VERSION at build time. */
export const ACTIVE_VERSION: TemplateVersion =
  (process.env.NEXT_PUBLIC_TEMPLATE_VERSION as TemplateVersion) ?? "v1";

/**
 * Resolve the active template manifest. `app/` route files call this so they
 * stay version-agnostic.
 */
export function activeTemplate(): TemplateManifest {
  const t = REGISTRY[ACTIVE_VERSION];
  if (!t) {
    throw new Error(
      `Unknown template version "${ACTIVE_VERSION}". Registered: ${Object.keys(
        REGISTRY,
      ).join(", ")}`,
    );
  }
  return t;
}

/** All registered version ids (for dev tooling / version pickers). */
export const templateVersions = Object.keys(REGISTRY) as TemplateVersion[];
