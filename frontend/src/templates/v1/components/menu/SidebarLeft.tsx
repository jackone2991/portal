import Link from "next/link";

/**
 * Left fixed sidebar — port of `components/menu/sidebarLeft.blade.php`.
 * Primary navigation; hidden below `lg`.
 */
export function SidebarLeft() {
  return (
    <aside
      className="fixed left-0 hidden border-r p-4 lg:block"
      style={{
        top: "var(--tpl-header-h)",
        width: "var(--tpl-sidebar-w)",
        height: "calc(100vh - var(--tpl-header-h))",
        borderColor: "var(--tpl-border)",
      }}
    >
      <nav className="space-y-2 text-sm">
        <Link href="/" className="block opacity-80 hover:opacity-100">
          Newsfeed
        </Link>
        <Link
          href="/library/comic"
          className="block opacity-80 hover:opacity-100"
        >
          Library · Comics
        </Link>
        <Link
          href="/library/novel/1"
          className="block opacity-80 hover:opacity-100"
        >
          Library · Novels
        </Link>
      </nav>
    </aside>
  );
}
