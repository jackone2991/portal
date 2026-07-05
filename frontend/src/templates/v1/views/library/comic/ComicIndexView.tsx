/**
 * Library · Comics index — port of `views/library/commic/index.blade.php`
 * (folder typo "commic" → "comic"). Rendered inside MasterBase.
 */
export function ComicIndexView() {
  return (
    <section>
      <header className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Comics</h1>
        <button
          type="button"
          className="rounded-md px-3 py-1.5 text-sm font-medium"
          style={{
            background: "var(--tpl-accent)",
            color: "var(--tpl-accent-contrast)",
          }}
        >
          {/* TODO: open <AddBook /> modal */}
          Add book
        </button>
      </header>

      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
        {/* TODO: map comic items from API */}
        {Array.from({ length: 5 }).map((_, i) => (
          <div
            key={i}
            className="aspect-[3/4] rounded-lg border"
            style={{
              borderColor: "var(--tpl-border)",
              background: "var(--tpl-surface)",
            }}
          />
        ))}
      </div>
    </section>
  );
}
