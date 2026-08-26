"use client";

import { useEffect, useMemo, useState } from "react";
import { useInfiniteQuery, useMutation, useQueryClient, type InfiniteData } from "@tanstack/react-query";
import { Composer } from "../../components/composer/Composer";
import { StreamItemCard } from "../../components/stream/StreamItemCard";
import { BirthdayCard } from "../../components/widget/BirthdayCard";
import { FinanceWidget } from "../../components/widget/FinanceWidget";
import { ContinueWidget } from "../../components/widget/ContinueWidget";
import { MusicWidget } from "../../components/widget/MusicWidget";
import { WeatherWidget } from "../../components/widget/WeatherWidget";
import { CalendarWidget } from "../../components/widget/CalendarWidget";
import { PagesWidget } from "../../components/widget/PagesWidget";
import { FriendSuggestions } from "../../components/widget/FriendSuggestions";
import { ActivityFeed } from "../../components/widget/ActivityFeed";
import { ApiError, baseURL } from "@/lib/api-client";
import { problemDisplayMessage } from "@/lib/problems";
import { createEntry, type CreateEntryInput } from "@/lib/journal";
import { getStream, type StreamItem, type StreamPage } from "@/lib/stream";

/**
 * Home — the life-stream (SPEC-06 P0.3/P0.4). The centre column is the merged
 * timeline (`GET /stream`): journal entries + system events (media/bank/comic/
 * people), newest first. The SPEC-05 composer sits on top; a new post is inserted
 * optimistically and survives refetch (the projection is written in the entry's
 * transaction — P0.1a). The rail carries real, failure-isolated facet widgets.
 * Zero fixtures on this route.
 */
const STREAM_KEY = ["stream"] as const;

export function HomeView() {
  const displayName = useDisplayName();
  const qc = useQueryClient();

  const [bodyMd, setBodyMd] = useState("");
  const [composerError, setComposerError] = useState<string | null>(null);

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
      const optimistic: StreamItem = {
        id: makeTempId(),
        source_module: "journal",
        event_type: "journal:entry_created",
        occurred_at: input.occurred_at ?? new Date().toISOString(),
        body_md: input.body_md,
        mood: null,
      };
      qc.setQueryData<InfiniteData<StreamPage>>(STREAM_KEY, (data) => prepend(data, optimistic));
      return { previous };
    },
    onError: (err, input, ctx) => {
      if (ctx?.previous) qc.setQueryData(STREAM_KEY, ctx.previous);
      setBodyMd(input.body_md);
      setComposerError(err instanceof ApiError ? problemDisplayMessage(err.body) : "Could not post");
    },
    onSuccess: () => {
      setComposerError(null);
      qc.invalidateQueries({ queryKey: STREAM_KEY }); // pull the canonical projection row
    },
  });

  function handleCreate() {
    const body = bodyMd.trim();
    if (!body || create.isPending) return;
    setComposerError(null);
    setBodyMd("");
    create.mutate({ body_md: body });
  }

  return (
    <div className="grid gap-5 lg:grid-cols-[260px_minmax(0,1fr)] 2xl:grid-cols-[260px_minmax(0,1fr)_300px]">
      <div className="hidden space-y-5 lg:block">
        <FinanceWidget />
        <ContinueWidget />
        <MusicWidget />
        <WeatherWidget />
        <CalendarWidget />
        <PagesWidget />
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

        {query.isPending ? (
          <p style={{ color: "var(--tpl-muted)" }}>Loading your stream…</p>
        ) : query.isError ? (
          <p style={{ color: "var(--tpl-muted)" }}>
            Couldn&apos;t load your stream.{" "}
            <button type="button" onClick={() => query.refetch()} className="underline">Retry</button>
          </p>
        ) : items.length === 0 ? (
          <p style={{ color: "var(--tpl-muted)" }}>Your life-stream is empty — write your first note above.</p>
        ) : (
          <>
            {items.map((it) => (
              <StreamItemCard key={it.id} item={it} displayName={displayName} />
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
        <BirthdayCard />
        <FriendSuggestions />
        <ActivityFeed />
      </aside>
    </div>
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

let tempSeq = 0;
function makeTempId(): string {
  tempSeq += 1;
  return `temp-${tempSeq}`;
}
