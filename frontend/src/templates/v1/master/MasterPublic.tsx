import type { ShellProps } from "@/templates/types";
import { SvgSprite } from "../components/footers/SvgSprite";

/**
 * Guest / marketing shell — port of `master/master-public.blade.php`.
 * Minimal chrome: just the page body + the shared svg sprite.
 */
export function MasterPublic({ children }: ShellProps) {
  return (
    <div data-template="v1" className="landing-page min-h-screen">
      {children}
      <SvgSprite />
    </div>
  );
}
