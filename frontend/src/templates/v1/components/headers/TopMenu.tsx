import Link from "next/link";

/**
 * Header navigation menu — port of `components/headers/menu.blade.php`.
 * Rendered inside SidebarCenter (the fixed top bar).
 */
export function TopMenu() {
  return (
    <nav className="mx-auto flex h-[var(--tpl-header-h)] max-w-6xl items-center gap-6 px-4">
      <Link href="/" className="font-semibold">
        Sky Feeling
      </Link>
      <Link href="/library/comic" className="opacity-80 hover:opacity-100">
        Comics
      </Link>
      <Link href="/library/novel/1" className="opacity-80 hover:opacity-100">
        Novels
      </Link>
      <span className="ml-auto text-sm" style={{ color: "var(--tpl-muted)" }}>
        {/* TODO: search · notifications · messages · profile dropdown */}
        search · notifications · profile
      </span>
    </nav>
  );
}
