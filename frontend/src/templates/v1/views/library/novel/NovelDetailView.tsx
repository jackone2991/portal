/**
 * Library · Novel detail — port of `views/library/novel/detail.blade.php`.
 * Rendered inside MasterBase. `id` comes from the `[id]` route segment.
 */
export function NovelDetailView({ id }: { id: string }) {
  return (
    <section>
      <h1 className="text-2xl font-semibold">Novel #{id}</h1>
      <div className="mt-6 grid gap-6 md:grid-cols-[12rem_1fr]">
        <div
          className="aspect-[3/4] rounded-lg border"
          style={{
            borderColor: "var(--tpl-border)",
            background: "var(--tpl-surface)",
          }}
        />
        <div>
          <p style={{ color: "var(--tpl-muted)" }}>
            {/* TODO: synopsis, metadata, chapter list */}
            Synopsis and chapter list go here.
          </p>
        </div>
      </div>
    </section>
  );
}
