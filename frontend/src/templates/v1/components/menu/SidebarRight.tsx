/**
 * Right fixed sidebar — port of `components/menu/sidebarRight.blade.php`.
 * Secondary rail (friends / chat / suggestions); hidden below `xl`.
 */
export function SidebarRight() {
  return (
    <aside
      className="fixed right-0 hidden border-l p-4 xl:block"
      style={{
        top: "var(--tpl-header-h)",
        width: "var(--tpl-sidebar-w)",
        height: "calc(100vh - var(--tpl-header-h))",
        borderColor: "var(--tpl-border)",
      }}
    >
      <p className="text-sm" style={{ color: "var(--tpl-muted)" }}>
        {/* TODO: contacts / chat list / who-to-follow */}
        Contacts
      </p>
    </aside>
  );
}
