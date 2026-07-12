"use client";

import { useEffect, useMemo, useState, type ReactNode } from "react";
import {
  useInfiniteQuery,
  useMutation,
  useQueryClient,
  type InfiniteData,
} from "@tanstack/react-query";
import { Avatar } from "../../components/ui/Avatar";
import { Icon } from "../../components/ui/Icon";
import { Composer } from "../../components/composer/Composer";
import { EntryCard } from "../../components/journal/EntryCard";
import { Modal, BtnSecondary } from "../../components/popup/Modal";
import { ApiError } from "@/lib/api-client";
import { problemDisplayMessage } from "@/lib/problems";
import {
  createEntry,
  deleteEntry,
  listEntries,
  patchEntry,
  type CreateEntryInput,
  type JournalEntry,
  type ListEntriesPage,
  type PatchEntryInput,
} from "@/lib/journal";

/**
 * Home / life-stream — SPEC-05 P0.4. The centre column is the real journal
 * write path: the ported {@link Composer} posts to `POST /journal/entries` via
 * a TanStack mutation with optimistic insert (D-32), and the feed slot renders
 * an interim journal-only list (`useInfiniteQuery` over `GET /journal/entries`,
 * newest `occurred_at` first). SPEC-06 later swaps this list query for
 * `GET /stream` without moving the composer.
 *
 * The old hard-coded `INITIAL_FEED` fixtures, the fake local-state composer and
 * the social `PostCard` are gone — every rendered entry is a DB row. The
 * left/right rails (weather/calendar/pages, birthday/friends/activity) remain
 * static Olympus chrome; they are out of SPEC-05's scope.
 */

const API_BASE =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "https://api.portal.localhost";

/** Query key for the journal list cache (single infinite-query entry). */
const JOURNAL_KEY = ["journal", "entries"] as const;

export function HomeView() {
  const displayName = useDisplayName();
  const queryClient = useQueryClient();

  // Composer draft (form state via local useState; the composer is controlled +
  // presentational so the mutation can clear/restore it).
  const [bodyMd, setBodyMd] = useState("");
  const [mood, setMood] = useState("");
  const [occurredAt, setOccurredAt] = useState(() => toDatetimeLocal(new Date()));
  const [composerError, setComposerError] = useState<string | null>(null);

  const [editing, setEditing] = useState<JournalEntry | null>(null);
  const [pendingDelete, setPendingDelete] = useState<JournalEntry | null>(null);
  const [rowError, setRowError] = useState<string | null>(null);

  const query = useInfiniteQuery({
    queryKey: JOURNAL_KEY,
    queryFn: ({ pageParam }: { pageParam: string | undefined }) =>
      listEntries({ cursor: pageParam }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage.next_cursor ?? undefined,
  });

  const entries = useMemo(
    () => query.data?.pages.flatMap((page) => page.items) ?? [],
    [query.data],
  );

  const createMutation = useMutation({
    mutationFn: (input: CreateEntryInput) => createEntry(input),
    onMutate: async (input) => {
      await queryClient.cancelQueries({ queryKey: JOURNAL_KEY });
      const previous = queryClient.getQueryData<InfiniteData<ListEntriesPage>>(JOURNAL_KEY);
      const tempId = makeTempId();
      const now = new Date().toISOString();
      const optimistic: JournalEntry = {
        id: tempId,
        body_md: input.body_md,
        mood: input.mood ?? null,
        asset_ids: [],
        occurred_at: input.occurred_at ?? now,
        created_at: now,
        updated_at: now,
      };
      queryClient.setQueryData<InfiniteData<ListEntriesPage>>(JOURNAL_KEY, (data) =>
        insertEntry(data, optimistic),
      );
      return { previous, tempId };
    },
    onError: (err, input, context) => {
      if (context?.previous) queryClient.setQueryData(JOURNAL_KEY, context.previous);
      // Restore the composer draft so nothing is lost (P0.4 acceptance).
      setBodyMd(input.body_md);
      setMood(input.mood ?? "");
      if (input.occurred_at) setOccurredAt(toDatetimeLocal(new Date(input.occurred_at)));
      setComposerError(errorMessage(err));
    },
    onSuccess: (saved, _input, context) => {
      // Swap the temp row for the server's canonical entry (id, timestamps) and
      // re-sort — no full refetch (P0.4 acceptance).
      queryClient.setQueryData<InfiniteData<ListEntriesPage>>(JOURNAL_KEY, (data) =>
        replaceEntry(data, context?.tempId ?? saved.id, () => saved),
      );
      setComposerError(null);
      setOccurredAt(toDatetimeLocal(new Date()));
    },
  });

  const patchMutation = useMutation({
    mutationFn: ({ id, fields }: { id: string; fields: PatchEntryInput }) =>
      patchEntry(id, fields),
    onMutate: async ({ id, fields }) => {
      await queryClient.cancelQueries({ queryKey: JOURNAL_KEY });
      const previous = queryClient.getQueryData<InfiniteData<ListEntriesPage>>(JOURNAL_KEY);
      const now = new Date().toISOString();
      queryClient.setQueryData<InfiniteData<ListEntriesPage>>(JOURNAL_KEY, (data) =>
        replaceEntry(data, id, (entry) => ({
          ...entry,
          ...(fields.body_md !== undefined ? { body_md: fields.body_md } : {}),
          ...(fields.mood !== undefined ? { mood: fields.mood || null } : {}),
          ...(fields.occurred_at !== undefined ? { occurred_at: fields.occurred_at } : {}),
          updated_at: now,
        })),
      );
      return { previous };
    },
    onError: (err, _vars, context) => {
      if (context?.previous) queryClient.setQueryData(JOURNAL_KEY, context.previous);
      setRowError(errorMessage(err));
    },
    onSuccess: (saved) => {
      queryClient.setQueryData<InfiniteData<ListEntriesPage>>(JOURNAL_KEY, (data) =>
        replaceEntry(data, saved.id, () => saved),
      );
      setEditing(null);
      setRowError(null);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (entry: JournalEntry) => deleteEntry(entry.id),
    onMutate: async (entry) => {
      await queryClient.cancelQueries({ queryKey: JOURNAL_KEY });
      const previous = queryClient.getQueryData<InfiniteData<ListEntriesPage>>(JOURNAL_KEY);
      queryClient.setQueryData<InfiniteData<ListEntriesPage>>(JOURNAL_KEY, (data) =>
        removeEntry(data, entry.id),
      );
      return { previous };
    },
    onError: (err, _entry, context) => {
      if (context?.previous) queryClient.setQueryData(JOURNAL_KEY, context.previous);
      setRowError(errorMessage(err));
    },
    onSuccess: () => {
      setPendingDelete(null);
      setRowError(null);
    },
  });

  function handleCreate() {
    const body = bodyMd.trim();
    if (!body || createMutation.isPending) return;
    setComposerError(null);
    const input: CreateEntryInput = {
      body_md: body,
      occurred_at: fromDatetimeLocal(occurredAt),
    };
    const trimmedMood = mood.trim();
    if (trimmedMood) input.mood = trimmedMood;
    // Optimistically clear the draft; onError restores it from the variables.
    setBodyMd("");
    setMood("");
    createMutation.mutate(input);
  }

  return (
    <div className="grid gap-5 lg:grid-cols-[260px_minmax(0,1fr)] 2xl:grid-cols-[260px_minmax(0,1fr)_300px]">
      <div className="hidden space-y-5 lg:block">
        <WeatherWidget />
        <CalendarWidget />
        <PagesWidget />
      </div>

      <div className="min-w-0 space-y-5">
        <Composer
          displayName={displayName}
          bodyMd={bodyMd}
          mood={mood}
          occurredAt={occurredAt}
          onBodyMdChange={setBodyMd}
          onMoodChange={setMood}
          onOccurredAtChange={setOccurredAt}
          onSubmit={handleCreate}
          submitting={createMutation.isPending}
          error={composerError}
        />

        {rowError && <ErrorBanner>{rowError}</ErrorBanner>}

        {query.isPending ? (
          <FeedSkeleton />
        ) : query.isError ? (
          <FeedError onRetry={() => query.refetch()} />
        ) : entries.length === 0 ? (
          <FeedEmpty />
        ) : (
          <>
            {entries.map((entry) => (
              <EntryCard
                key={entry.id}
                displayName={displayName}
                entry={entry}
                onRequestEdit={() => {
                  setRowError(null);
                  setEditing(entry);
                }}
                onRequestDelete={() => {
                  setRowError(null);
                  setPendingDelete(entry);
                }}
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
        <BirthdayCard />
        <FriendSuggestions />
        <ActivityFeed />
      </aside>

      {editing && (
        <EditEntryDialog
          entry={editing}
          saving={patchMutation.isPending}
          onCancel={() => setEditing(null)}
          onSave={(fields) => patchMutation.mutate({ id: editing.id, fields })}
        />
      )}

      {pendingDelete && (
        <Modal open onClose={() => setPendingDelete(null)} title="Delete entry?" width={420}>
          <div className="space-y-4 p-6">
            <p className="text-sm" style={{ color: "var(--tpl-text)" }}>
              This journal entry will be permanently deleted. This can&apos;t be undone.
            </p>
            <div className="flex justify-end gap-2">
              <BtnSecondary onClick={() => setPendingDelete(null)}>Cancel</BtnSecondary>
              <button
                type="button"
                onClick={() => deleteMutation.mutate(pendingDelete)}
                disabled={deleteMutation.isPending}
                className="rounded-md px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-50"
                style={{ background: "#ef4444" }}
              >
                {deleteMutation.isPending ? "Deleting…" : "Delete"}
              </button>
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
}

/* ── current user (display name) ──────────────────────────────────── */

/** Owner display name for composer/entry avatars. Kept as a light fetch (as it
 * was before) — it's chrome, not the P0.4 server-state surface. */
function useDisplayName(): string {
  const [displayName, setDisplayName] = useState("You");
  useEffect(() => {
    let alive = true;
    fetch(`${API_BASE}/api/v1/auth/me`, { credentials: "include" })
      .then((r) => (r.ok ? r.json() : null))
      .then((me: { display_name?: string } | null) => {
        if (alive && me?.display_name) setDisplayName(me.display_name);
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, []);
  return displayName;
}

/* ── edit dialog ──────────────────────────────────────────────────── */

function EditEntryDialog({
  entry,
  saving,
  onCancel,
  onSave,
}: {
  entry: JournalEntry;
  saving: boolean;
  onCancel: () => void;
  onSave: (fields: PatchEntryInput) => void;
}) {
  const [body, setBody] = useState(entry.body_md);
  const [mood, setMood] = useState(entry.mood ?? "");
  const [occurredAt, setOccurredAt] = useState(() =>
    toDatetimeLocal(new Date(entry.occurred_at)),
  );

  function save() {
    const trimmedBody = body.trim();
    if (!trimmedBody || saving) return;
    const fields: PatchEntryInput = {
      body_md: trimmedBody,
      mood: mood.trim(),
      occurred_at: fromDatetimeLocal(occurredAt),
    };
    onSave(fields);
  }

  return (
    <Modal open onClose={onCancel} title="Edit entry" width={520}>
      <div className="space-y-4 p-6">
        <label className="block">
          <span className="mb-1.5 block text-xs font-semibold uppercase tracking-wide" style={{ color: "var(--tpl-muted)" }}>
            Entry
          </span>
          <textarea
            value={body}
            onChange={(e) => setBody(e.target.value)}
            rows={4}
            className="w-full resize-y rounded-md border bg-transparent px-3 py-2 text-sm outline-none transition focus:border-[var(--tpl-accent)]"
            style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }}
          />
        </label>

        <div className="flex flex-wrap gap-4">
          <label className="min-w-0 flex-1 basis-40">
            <span className="mb-1.5 block text-xs font-semibold uppercase tracking-wide" style={{ color: "var(--tpl-muted)" }}>
              Mood
            </span>
            <input
              type="text"
              value={mood}
              onChange={(e) => setMood(e.target.value)}
              maxLength={80}
              placeholder="Optional"
              className="w-full rounded-md border bg-transparent px-3 py-2 text-sm outline-none transition focus:border-[var(--tpl-accent)]"
              style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }}
            />
          </label>
          <label className="min-w-0 flex-1 basis-52">
            <span className="mb-1.5 block text-xs font-semibold uppercase tracking-wide" style={{ color: "var(--tpl-muted)" }}>
              Happened at
            </span>
            <input
              type="datetime-local"
              value={occurredAt}
              onChange={(e) => setOccurredAt(e.target.value)}
              className="w-full rounded-md border bg-transparent px-3 py-2 text-sm outline-none transition focus:border-[var(--tpl-accent)]"
              style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }}
            />
          </label>
        </div>

        <div className="flex justify-end gap-2">
          <BtnSecondary onClick={onCancel}>Cancel</BtnSecondary>
          <button
            type="button"
            onClick={save}
            disabled={!body.trim() || saving}
            className="rounded-md px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-50"
            style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
          >
            {saving ? "Saving…" : "Save changes"}
          </button>
        </div>
      </div>
    </Modal>
  );
}

/* ── feed states ──────────────────────────────────────────────────── */

function FeedSkeleton() {
  return (
    <>
      {Array.from({ length: 3 }).map((_, i) => (
        <div
          key={i}
          className="h-32 animate-pulse rounded-xl"
          style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)" }}
        />
      ))}
    </>
  );
}

function FeedEmpty() {
  return (
    <div
      className="flex flex-col items-center gap-2 rounded-xl border border-dashed py-16 text-center"
      style={{ borderColor: "var(--tpl-border)" }}
    >
      <Icon name="status-icon" size={28} style={{ color: "var(--tpl-muted)" }} />
      <p className="text-sm" style={{ color: "var(--tpl-muted)" }}>
        No journal entries yet — write your first one above.
      </p>
    </div>
  );
}

function FeedError({ onRetry }: { onRetry: () => void }) {
  return (
    <div className="rounded-xl border py-12 text-center" style={{ borderColor: "var(--tpl-border)" }}>
      <p className="text-sm" style={{ color: "var(--tpl-muted)" }}>
        Couldn&apos;t load your journal.
      </p>
      <button
        type="button"
        onClick={onRetry}
        className="mt-3 rounded-md border px-3 py-1.5 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)]"
        style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
      >
        Retry
      </button>
    </div>
  );
}

function ErrorBanner({ children }: { children: ReactNode }) {
  return (
    <p
      className="rounded-lg border px-3 py-2 text-sm"
      style={{ borderColor: "rgba(239,68,68,.4)", background: "rgba(239,68,68,.08)", color: "#ef4444" }}
    >
      {children}
    </p>
  );
}

/* ── journal cache + date helpers ─────────────────────────────────── */

function errorMessage(err: unknown): string {
  return err instanceof ApiError
    ? problemDisplayMessage(err.body)
    : "Something went wrong. Please try again.";
}

/** `occurred_at DESC, id DESC` — the timeline order (SPEC-05 §5 P0.2). */
function compareEntries(a: JournalEntry, b: JournalEntry): number {
  if (a.occurred_at !== b.occurred_at) return a.occurred_at < b.occurred_at ? 1 : -1;
  return a.id < b.id ? 1 : -1;
}

function insertEntry(
  data: InfiniteData<ListEntriesPage> | undefined,
  entry: JournalEntry,
): InfiniteData<ListEntriesPage> {
  const first = data?.pages[0];
  if (!data || !first) {
    return { pageParams: [undefined], pages: [{ items: [entry], next_cursor: null }] };
  }
  const items = [entry, ...first.items].sort(compareEntries);
  return { ...data, pages: [{ ...first, items }, ...data.pages.slice(1)] };
}

function replaceEntry(
  data: InfiniteData<ListEntriesPage> | undefined,
  id: string,
  update: (entry: JournalEntry) => JournalEntry,
): InfiniteData<ListEntriesPage> | undefined {
  if (!data) return data;
  const sizes = data.pages.map((p) => p.items.length);
  const cursors = data.pages.map((p) => p.next_cursor ?? null);
  const all = data.pages
    .flatMap((p) => p.items)
    .map((e) => (e.id === id ? update(e) : e))
    .sort(compareEntries);
  let idx = 0;
  const pages = sizes.map((size, i) => {
    const slice = all.slice(idx, idx + size);
    idx += size;
    return { items: slice, next_cursor: cursors[i] };
  });
  return { ...data, pages };
}

function removeEntry(
  data: InfiniteData<ListEntriesPage> | undefined,
  id: string,
): InfiniteData<ListEntriesPage> | undefined {
  if (!data) return data;
  return {
    ...data,
    pages: data.pages.map((p) => ({ ...p, items: p.items.filter((e) => e.id !== id) })),
  };
}

function makeTempId(): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) return crypto.randomUUID();
  return `tmp-${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

/** `Date` → `datetime-local` input value (`YYYY-MM-DDTHH:mm`), local tz. */
function toDatetimeLocal(d: Date): string {
  if (Number.isNaN(d.getTime())) return toDatetimeLocal(new Date());
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(
    d.getHours(),
  )}:${pad(d.getMinutes())}`;
}

/** `datetime-local` value → ISO string (interpreted in local tz). */
function fromDatetimeLocal(value: string): string {
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? new Date().toISOString() : d.toISOString();
}

/* ── Left rail widgets ────────────────────────────────────────────── */

function WeatherWidget() {
  const week = [
    ["SUN", "weather-sunny-icon", 60],
    ["MON", "weather-sunny-icon", 58],
    ["TUE", "weather-cloudy-icon", 67],
    ["WED", "weather-rain-icon", 70],
    ["THU", "weather-rain-icon", 58],
    ["FRI", "weather-rain-icon", 68],
    ["SAT", "weather-partly-sunny-icon", 65],
  ] as const;
  return (
    <div
      className="overflow-hidden rounded-xl text-white shadow-sm"
      style={{ background: "linear-gradient(160deg, var(--tpl-weather-1), var(--tpl-weather-2))" }}
    >
      <div className="p-5">
        <div className="flex items-start justify-between">
          <div className="flex items-start gap-1">
            <span className="text-5xl font-light leading-none">64°</span>
            <span className="mt-1 text-xs leading-tight text-white/80">
              58°
              <br />
              76°
            </span>
          </div>
          <Icon name="weather-partly-sunny-icon" size={40} />
        </div>
        <p className="mt-3 text-lg font-semibold">Partly Sunny</p>
        <p className="text-xs text-white/80">Real Feel: 67°&nbsp;&nbsp;&nbsp;Chance of Rain: 49%</p>
      </div>

      <div className="grid grid-cols-7 gap-1 px-3 py-4" style={{ background: "rgba(255,255,255,0.08)" }}>
        {week.map(([d, ic, t]) => (
          <div key={d} className="flex flex-col items-center gap-1.5 text-[10px] text-white/85">
            <span className="font-semibold">{d}</span>
            <Icon name={ic} size={18} />
            <span>{t}°</span>
          </div>
        ))}
      </div>

      <div className="px-5 py-3 text-center text-xs">
        <p className="font-semibold">Saturday, March 26th</p>
        <p className="text-white/75">San Francisco, CA</p>
      </div>
    </div>
  );
}

function CalendarWidget() {
  const days = Array.from({ length: 31 }, (_, i) => i + 1);
  return (
    <WidgetCard>
      <div className="mb-3 flex items-center justify-between px-1" style={{ color: "var(--tpl-muted)" }}>
        <button type="button" aria-label="Previous month">
          <Icon name="popup-left-arrow" size={12} />
        </button>
        <span className="font-semibold" style={{ color: "var(--tpl-heading)" }}>
          May
        </span>
        <button type="button" aria-label="Next month">
          <Icon name="popup-right-arrow" size={12} />
        </button>
      </div>
      <div className="grid grid-cols-7 gap-y-2 text-center text-[11px]">
        {["MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"].map((d) => (
          <span key={d} className="font-semibold" style={{ color: "var(--tpl-muted)" }}>
            {d}
          </span>
        ))}
        {days.map((n) => (
          <span
            key={n}
            className="grid h-7 place-items-center rounded-full text-xs"
            style={
              n === 12
                ? { background: "var(--tpl-accent)", color: "#fff", margin: "0 auto", width: 28 }
                : { color: "var(--tpl-text)" }
            }
          >
            {n}
          </span>
        ))}
      </div>
    </WidgetCard>
  );
}

function PagesWidget() {
  const pages = [
    { name: "The Marina Bar", cat: "Restaurant · Bar" },
    { name: "Tapronus Rock", cat: "Rock Band" },
    { name: "Pixel Digital Design", cat: "Company" },
    { name: "Thompson's Custom Clothing Boutique", cat: "Clothing Store" },
  ];
  return (
    <WidgetCard title="Pages You May Like" more>
      <ul className="space-y-3">
        {pages.map((p) => (
          <li key={p.name} className="flex items-center gap-3">
            <Avatar name={p.name} size={38} />
            <div className="min-w-0">
              <p className="truncate text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>
                {p.name}
              </p>
              <p className="truncate text-xs" style={{ color: "var(--tpl-muted)" }}>
                {p.cat}
              </p>
            </div>
            <button
              type="button"
              className="ml-auto text-[var(--tpl-muted)] transition hover:text-[var(--tpl-accent)]"
              aria-label="Like page"
            >
              <Icon name="star-icon" size={18} />
            </button>
          </li>
        ))}
      </ul>
    </WidgetCard>
  );
}

/* ── Right rail widgets ───────────────────────────────────────────── */

function BirthdayCard() {
  return (
    <div
      className="relative overflow-hidden rounded-xl p-5 text-white shadow-sm"
      style={{ background: "linear-gradient(150deg, #8a63d2, #6d4bb8)" }}
    >
      {/* decorative blobs (≈ Olympus birthday widget) */}
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 opacity-20"
        style={{
          background:
            "radial-gradient(60% 60% at 85% 15%, rgba(255,255,255,0.5), transparent 60%)," +
            "radial-gradient(50% 50% at 20% 90%, rgba(255,255,255,0.25), transparent 60%)",
        }}
      />
      <div className="relative">
        <div className="flex items-start justify-between">
          <Icon name="cupcake-icon" size={22} />
          <button type="button" aria-label="Options">
            <Icon name="three-dots-icon" size={12} />
          </button>
        </div>

        <div className="mt-4">
          <Avatar name="Marina Valentine" size={44} className="ring-2 ring-white/40" />
        </div>

        <p className="mt-3 text-sm text-white/80">Today is</p>
        <h3 className="text-2xl font-bold leading-tight">Marina Valentine&apos;s Birthday!</h3>
        <p className="mt-3 text-sm leading-relaxed text-white/80">
          Leave her a message with your best wishes on her profile page!
        </p>
      </div>
    </div>
  );
}

function FriendSuggestions() {
  const people = [
    { name: "Francine Smith", meta: "8 Friends in Common" },
    { name: "Hugh Wilson", meta: "6 Friends in Common" },
    { name: "Karen Masters", meta: "6 Friends in Common" },
  ];
  return (
    <WidgetCard title="Friend Suggestions" more>
      <ul className="space-y-3">
        {people.map((p) => (
          <li key={p.name} className="flex items-center gap-3">
            <Avatar name={p.name} size={40} />
            <div className="min-w-0">
              <p className="truncate text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>
                {p.name}
              </p>
              <p className="truncate text-xs" style={{ color: "var(--tpl-muted)" }}>
                {p.meta}
              </p>
            </div>
            <button
              type="button"
              className="ml-auto grid h-8 w-8 place-items-center rounded-md text-white transition hover:opacity-90"
              style={{ background: "var(--tpl-blue)" }}
              aria-label={`Add ${p.name}`}
            >
              <Icon name="happy-face-icon" size={16} />
            </button>
          </li>
        ))}
      </ul>
    </WidgetCard>
  );
}

function ActivityFeed() {
  const items = [
    { who: "Marina Polson", what: <>commented on Jason Mark&apos;s <A>photo</A>.</>, time: "2 mins ago", icon: "comments-post-icon" },
    { who: "Jake Parker", what: <>liked Nicholas Grissom&apos;s <A>status update</A>.</>, time: "5 mins ago", icon: "like-post-icon" },
    { who: "Mary Jane Stark", what: <>added 20 new photos to her <A>gallery album</A>.</>, time: "12 mins ago", icon: "photos-icon" },
    { who: "Nicholas Grissom", what: <>updated his profile <A>photo</A>.</>, time: "1 hour ago", icon: "happy-face-icon" },
  ];
  return (
    <WidgetCard title="Activity Feed">
      <ul className="space-y-4">
        {items.map((it) => (
          <li key={it.who} className="flex items-start gap-3">
            <Avatar name={it.who} size={36} />
            <div className="min-w-0 flex-1">
              <p className="text-sm leading-snug" style={{ color: "var(--tpl-text)" }}>
                <b style={{ color: "var(--tpl-heading)" }}>{it.who}</b> {it.what}
              </p>
              <p className="mt-0.5 text-xs" style={{ color: "var(--tpl-muted)" }}>
                {it.time}
              </p>
            </div>
            <span className="mt-1 shrink-0" style={{ color: "var(--tpl-muted)" }}>
              <Icon name={it.icon} size={16} />
            </span>
          </li>
        ))}
      </ul>
    </WidgetCard>
  );
}

/* ── Shared pieces ────────────────────────────────────────────────── */

function Card({
  children,
  className = "",
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={`rounded-xl shadow-sm ${className}`}
      style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)" }}
    >
      {children}
    </div>
  );
}

function WidgetCard({
  title,
  more,
  children,
}: {
  title?: string;
  more?: boolean;
  children: ReactNode;
}) {
  return (
    <Card className="p-4">
      {title && (
        <div className="mb-3 flex items-center justify-between">
          <h3 className="text-sm font-bold" style={{ color: "var(--tpl-heading)" }}>
            {title}
          </h3>
          {more && (
            <span style={{ color: "var(--tpl-muted)" }}>
              <Icon name="three-dots-icon" size={12} />
            </span>
          )}
        </div>
      )}
      {children}
    </Card>
  );
}

function A({ children }: { children: ReactNode }) {
  return (
    <a href="#" className="font-medium hover:underline" style={{ color: "var(--tpl-accent)" }}>
      {children}
    </a>
  );
}
