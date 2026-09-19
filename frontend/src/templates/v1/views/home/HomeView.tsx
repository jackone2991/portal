"use client";

import { useEffect, useMemo, useState } from "react";
import {
  useInfiniteQuery,
  useMutation,
  useQueryClient,
  type InfiniteData,
} from "@tanstack/react-query";
import { Composer, type ComposerDraft } from "../../components/composer/Composer";
import { StreamItemCard } from "../../components/stream/StreamItemCard";
import { widgetComponent } from "../../components/widget/registry";
import { useLayout, type WidgetSlot } from "@/lib/layout";
import { ApiError, baseURL } from "@/lib/api-client";
import { problemDisplayMessage } from "@/lib/problems";
import { createEntry, deleteEntry, patchEntry, type CreateEntryInput } from "@/lib/journal";
import { getStream, type StreamItem, type StreamPage } from "@/lib/stream";

/**
 * Home — the newsfeed / life-stream (SPEC-06 P0.3/P0.4). The centre column is
 * the merged timeline (`GET /stream`): journal entries + system events (media/
 * bank/comic/people), newest first. The SPEC-05 composer sits on top; a new post
 * is inserted optimistically and survives refetch (the projection is written in
 * the entry's transaction — P0.1a). Edit and delete on a post go straight to the
 * journal entry behind the card's `ref_id`, also optimistically. The rail
 * carries real, failure-isolated facet widgets. Zero fixtures on this route.
 */
const STREAM_KEY = ["stream"] as const;

export function HomeView() {
  const displayName = useDisplayName();
  const qc = useQueryClient();

  const [bodyMd, setBodyMd] = useState("");
  const [composerError, setComposerError] = useState<string | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const query = useInfiniteQuery({
    queryKey: STREAM_KEY,
    queryFn: ({ pageParam }: { pageParam: string | undefined }) => getStream(pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor ?? undefined,
  });

  const items = useMemo(() => query.data?.pages.flatMap((p) => p.items) ?? [], [query.data]);

  const create = useMutation({
    mutationFn: (input: CreateEntryInput) => createEntry(input),
    onMutate: async (input) => {
      await qc.cancelQueries({ queryKey: STREAM_KEY });
      const previous = qc.getQueryData<InfiniteData<StreamPage>>(STREAM_KEY);
      const tempId = makeTempId();
      const optimistic: StreamItem = {
        id: tempId,
        ref_id: tempId,
        source_module: "journal",
        event_type: "journal:entry_created",
        occurred_at: input.occurred_at ?? new Date().toISOString(),
        body_md: input.body_md ?? "",
        mood: null,
        asset_ids: input.asset_ids ?? [],
        location: input.location ?? null,
      };
      qc.setQueryData<InfiniteData<StreamPage>>(STREAM_KEY, (data) => prepend(data, optimistic));
      return { previous };
    },
    onError: (err, input, ctx) => {
      if (ctx?.previous) qc.setQueryData(STREAM_KEY, ctx.previous);
      // The text comes back into the box; the composer keeps its Attachments
      // and Location itself until `handleCreate` reports success.
      setBodyMd(input.body_md ?? "");
      setComposerError(err instanceof ApiError ? problemDisplayMessage(err.body) : "Could not post");
    },
    onSuccess: () => {
      setComposerError(null);
      qc.invalidateQueries({ queryKey: STREAM_KEY }); // pull the canonical projection row
    },
  });

  const update = useMutation({
    mutationFn: ({ refId, bodyMd: body }: { refId: string; bodyMd: string }) =>
      patchEntry(refId, { body_md: body }),
    onMutate: async ({ refId, bodyMd: body }) => {
      await qc.cancelQueries({ queryKey: STREAM_KEY });
      const previous = qc.getQueryData<InfiniteData<StreamPage>>(STREAM_KEY);
      qc.setQueryData<InfiniteData<StreamPage>>(STREAM_KEY, (data) =>
        mapItems(data, (it) => (it.ref_id === refId ? { ...it, body_md: body } : it)),
      );
      return { previous };
    },
    onError: (err, _vars, ctx) => {
      if (ctx?.previous) qc.setQueryData(STREAM_KEY, ctx.previous);
      setActionError(
        err instanceof ApiError ? problemDisplayMessage(err.body) : "Could not save the post",
      );
    },
    onSuccess: () => {
      setActionError(null);
      setEditingId(null);
      qc.invalidateQueries({ queryKey: STREAM_KEY });
    },
  });

  const remove = useMutation({
    mutationFn: (refId: string) => deleteEntry(refId),
    onMutate: async (refId) => {
      await qc.cancelQueries({ queryKey: STREAM_KEY });
      const previous = qc.getQueryData<InfiniteData<StreamPage>>(STREAM_KEY);
      qc.setQueryData<InfiniteData<StreamPage>>(STREAM_KEY, (data) =>
        filterItems(data, (it) => it.ref_id !== refId),
      );
      return { previous };
    },
    onError: (err, _refId, ctx) => {
      if (ctx?.previous) qc.setQueryData(STREAM_KEY, ctx.previous);
      setActionError(
        err instanceof ApiError ? problemDisplayMessage(err.body) : "Could not delete the post",
      );
    },
    onSuccess: () => {
      setActionError(null);
      qc.invalidateQueries({ queryKey: STREAM_KEY });
    },
  });

  // The composer decides what is postable; this only reports whether the
  // server took it (the mutation's onError has already shown why not).
  async function handleCreate(draft: ComposerDraft): Promise<boolean> {
    if (create.isPending) return false;
    setComposerError(null);
    setBodyMd("");
    try {
      await create.mutateAsync({
        body_md: draft.bodyMd.trim(),
        asset_ids: draft.assetIds,
        location: draft.location,
      });
      return true;
    } catch {
      return false;
    }
  }

  return (
    <div className="grid gap-5 lg:grid-cols-[260px_minmax(0,1fr)] 2xl:grid-cols-[260px_minmax(0,1fr)_300px]">
      <div className="hidden space-y-5 lg:block">
        <WidgetRail slot="left" />
      </div>

      <div className="min-w-0 space-y-5">
        <Composer
          displayName={displayName}
          bodyMd={bodyMd}
          onBodyMdChange={setBodyMd}
          onSubmit={handleCreate}
          submitting={create.isPending}
          error={composerError}
        />

        {actionError && (
          <p
            role="alert"
            className="rounded-lg border px-3 py-2 text-sm"
            style={{
              borderColor: "rgba(239,68,68,.4)",
              background: "rgba(239,68,68,.08)",
              color: "#ef4444",
            }}
          >
            {actionError}
          </p>
        )}

        {query.isPending ? (
          <p style={{ color: "var(--tpl-muted)" }}>Loading your stream…</p>
        ) : query.isError ? (
          <p style={{ color: "var(--tpl-muted)" }}>
            Couldn&apos;t load your stream.{" "}
            <button type="button" onClick={() => query.refetch()} className="underline">
              Retry
            </button>
          </p>
        ) : items.length === 0 ? (
          <p style={{ color: "var(--tpl-muted)" }}>
            Your life-stream is empty — write your first note above.
          </p>
        ) : (
          <>
            {items.map((it) => (
              <StreamItemCard
                key={it.id}
                item={it}
                displayName={displayName}
                editing={editingId === it.id}
                saving={update.isPending}
                onStartEdit={(target) => {
                  setActionError(null);
                  setEditingId(target.id);
                }}
                onCancelEdit={() => setEditingId(null)}
                onSave={(target, body) => update.mutate({ refId: target.ref_id, bodyMd: body })}
                onDelete={(target) => remove.mutate(target.ref_id)}
              />
            ))}
            {query.hasNextPage && (
              <div className="flex justify-center">
                <button
                  type="button"
                  onClick={() => query.fetchNextPage()}
                  disabled={query.isFetchingNextPage}
                  className="rounded-md border px-4 py-2 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)] disabled:opacity-50"
                  style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
                >
                  {query.isFetchingNextPage ? "Loading…" : "Load more"}
                </button>
              </div>
            )}
          </>
        )}
      </div>

      <aside className="hidden space-y-5 2xl:block">
        <WidgetRail slot="right" />
      </aside>
    </div>
  );
}

/**
 * One dashboard rail, composed from `GET /layout` — which rows appear, in which
 * order, is data now (migration 0035) rather than JSX.
 *
 * Renders nothing at all until the layout resolves. A rail is decoration around
 * the feed, so an empty column for one beat is invisible; guessing a default set
 * and then reshuffling it would not be.
 *
 * A key the bundle does not know is skipped rather than thrown on: during a
 * rolling deploy an old bundle can meet a newly seeded widget, and one missing
 * card beats a crashed dashboard.
 */
function WidgetRail({ slot }: { slot: WidgetSlot }) {
  const { data } = useLayout();
  const widgets = (data?.widgets ?? []).filter((w) => w.slot === slot);

  return (
    <>
      {widgets.map((w) => {
        const Widget = widgetComponent(w.key);
        return Widget ? <Widget key={w.key} /> : null;
      })}
    </>
  );
}

/* ── helpers ───────────────────────────────────────────────────────── */

function useDisplayName(): string {
  const [name, setName] = useState("You");
  useEffect(() => {
    let alive = true;
    fetch(`${baseURL}/api/v1/auth/me`, { credentials: "include" })
      .then((r) => (r.ok ? r.json() : null))
      .then((me: { display_name?: string } | null) => {
        if (alive && me?.display_name) setName(me.display_name);
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, []);
  return name;
}

function prepend(data: InfiniteData<StreamPage> | undefined, item: StreamItem): InfiniteData<StreamPage> {
  const first = data?.pages[0];
  if (!data || !first) {
    return { pages: [{ items: [item] }], pageParams: [undefined] };
  }
  return { ...data, pages: [{ ...first, items: [item, ...first.items] }, ...data.pages.slice(1)] };
}

function mapItems(
  data: InfiniteData<StreamPage> | undefined,
  fn: (item: StreamItem) => StreamItem,
): InfiniteData<StreamPage> | undefined {
  if (!data) return data;
  return { ...data, pages: data.pages.map((p) => ({ ...p, items: p.items.map(fn) })) };
}

function filterItems(
  data: InfiniteData<StreamPage> | undefined,
  keep: (item: StreamItem) => boolean,
): InfiniteData<StreamPage> | undefined {
  if (!data) return data;
  return { ...data, pages: data.pages.map((p) => ({ ...p, items: p.items.filter(keep) })) };
}

let tempSeq = 0;
function makeTempId(): string {
  tempSeq += 1;
  return `temp-${tempSeq}`;
}
