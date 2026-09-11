"use client";

import { useState } from "react";
import Link from "next/link";
import type { Route } from "next";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  CIRCLE_LABEL,
  createPerson,
  formatBirthday,
  listPeople,
  listSuggestions,
  updatePerson,
  upcomingBirthdays,
  type Birthday,
  type Person,
  type PersonCircle,
} from "@/lib/people";
import {
  acceptConnection,
  listConnections,
  removeConnection,
  requestConnection,
  type Connection,
} from "@/lib/social";
import { problemDisplayMessage } from "@/lib/problems";
import { ApiError } from "@/lib/api-client";
import { Avatar } from "../../components/ui/Avatar";

/**
 * People — the registry, and the management page behind each section of the
 * right rail. The rail's "Settings" links land here with `?circle=…`, so a tab
 * is a real address you can link to and reload, not component state.
 *
 * `suggestions` is a tab, not a circle: it lists accounts on this Portal that
 * are not in the registry yet, and adding one files it into a circle and records
 * the account id, which is what stops it being suggested again.
 */
type Tab = PersonCircle | "all" | "suggestions" | "requests";

const TABS: { key: Tab; label: string }[] = [
  { key: "all", label: "Everyone" },
  { key: "close_friend", label: CIRCLE_LABEL.close_friend },
  { key: "family", label: CIRCLE_LABEL.family },
  { key: "other", label: CIRCLE_LABEL.other },
  { key: "suggestions", label: "Có thể bạn biết" },
  { key: "requests", label: "Lời mời" },
];

const CIRCLE_OPTIONS: PersonCircle[] = ["close_friend", "family", "other"];

export function PeopleIndexView() {
  const qc = useQueryClient();
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const tab = (searchParams.get("circle") as Tab | null) ?? "all";
  const circle: PersonCircle | undefined =
    tab === "all" || tab === "suggestions" || tab === "requests" ? undefined : tab;

  const people = useQuery({
    queryKey: ["people", "list", circle ?? "all"],
    queryFn: () => listPeople(undefined, circle),
    enabled: tab !== "suggestions" && tab !== "requests",
  });
  const incoming = useQuery({
    queryKey: ["connections", "incoming"],
    queryFn: () => listConnections("incoming"),
    enabled: tab === "requests",
  });
  const outgoing = useQuery({
    queryKey: ["connections", "outgoing"],
    queryFn: () => listConnections("outgoing"),
    enabled: tab === "requests",
  });
  const suggestions = useQuery({
    queryKey: ["people", "suggestions"],
    queryFn: listSuggestions,
    enabled: tab === "suggestions",
  });
  const { data: upcoming = [] } = useQuery({
    queryKey: ["people", "upcoming"],
    queryFn: () => upcomingBirthdays(30),
  });

  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [relationship, setRelationship] = useState("");
  const [month, setMonth] = useState("");
  const [day, setDay] = useState("");
  const [year, setYear] = useState("");
  const [err, setErr] = useState<string | null>(null);

  /** Every mutation here changes what the rail shows, so both go stale. */
  function refresh() {
    qc.invalidateQueries({ queryKey: ["people"] });
    qc.invalidateQueries({ queryKey: ["connections"] });
  }

  const create = useMutation({
    mutationFn: () => {
      let birthday: Birthday | null = null;
      if (month && day) {
        birthday = {
          month: Number(month),
          day: Number(day),
          year: year ? Number(year) : null,
          calendar: "solar",
        };
      }
      return createPerson({
        display_name: name,
        relationship: relationship || null,
        birthday,
        // Adding from inside a circle's page files them there — that is what
        // you came to this page to do.
        circle: circle ?? "other",
      });
    },
    onSuccess: () => {
      setName("");
      setRelationship("");
      setMonth("");
      setDay("");
      setYear("");
      setOpen(false);
      setErr(null);
      refresh();
    },
    onError: (e) => setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : "Could not save"),
  });

  const move = useMutation({
    mutationFn: ({ id, to }: { id: string; to: PersonCircle }) => updatePerson(id, { circle: to }),
    onSuccess: refresh,
    onError: (e) =>
      setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : "Could not move this person"),
  });

  // Connections. Each of these changes the suggestion list too — someone you
  // have asked is no longer someone you "may know" — hence the shared refresh.
  const connect = useMutation({
    mutationFn: (userId: string) => requestConnection(userId),
    onSuccess: refresh,
    onError: (e) =>
      setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : "Could not send the request"),
  });
  const accept = useMutation({
    mutationFn: (id: string) => acceptConnection(id),
    onSuccess: refresh,
    onError: (e) =>
      setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : "Could not accept"),
  });
  const drop = useMutation({
    mutationFn: (id: string) => removeConnection(id),
    onSuccess: refresh,
    onError: (e) =>
      setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : "Could not update the request"),
  });

  const add = useMutation({
    mutationFn: ({ userId, displayName }: { userId: string; displayName: string }) =>
      createPerson({ display_name: displayName, linked_user_id: userId, circle: "other" }),
    onSuccess: refresh,
    onError: (e) =>
      setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : "Could not add this person"),
  });

  function selectTab(next: Tab) {
    setErr(null);
    const params = new URLSearchParams(searchParams.toString());
    if (next === "all") params.delete("circle");
    else params.set("circle", next);
    const qs = params.toString();
    router.replace((qs ? `${pathname}?${qs}` : pathname) as Route);
  }

  const rows = people.data?.people ?? [];

  return (
    <section>
      <header className="mb-6 flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-2xl font-semibold" style={{ color: "var(--tpl-heading)" }}>
          People
        </h1>
        {tab !== "suggestions" && (
          <button
            type="button"
            onClick={() => setOpen((o) => !o)}
            className="rounded-md px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90"
            style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
          >
            {open ? "Close" : "+ Add person"}
          </button>
        )}
      </header>

      <div
        className="mb-5 flex flex-wrap gap-1 border-b"
        style={{ borderColor: "var(--tpl-border)" }}
      >
        {TABS.map((t) => {
          const on = t.key === tab;
          return (
            <button
              key={t.key}
              type="button"
              onClick={() => selectTab(t.key)}
              className="-mb-px px-4 py-2.5 text-sm font-semibold transition"
              style={
                on
                  ? { color: "var(--tpl-accent)", boxShadow: "inset 0 -2px 0 var(--tpl-accent)" }
                  : { color: "var(--tpl-muted)" }
              }
            >
              {t.label}
            </button>
          );
        })}
      </div>

      {err && (
        <p
          role="alert"
          className="mb-4 rounded-lg border px-3 py-2 text-sm"
          style={{
            borderColor: "rgba(239,68,68,.4)",
            background: "rgba(239,68,68,.08)",
            color: "#ef4444",
          }}
        >
          {err}
        </p>
      )}

      {tab === "all" && upcoming.length > 0 && (
        <Card className="mb-6 p-4">
          <h2
            className="mb-2 text-xs font-bold uppercase tracking-wide"
            style={{ color: "var(--tpl-muted)" }}
          >
            Upcoming birthdays
          </h2>
          <ul className="space-y-1 text-sm">
            {upcoming.slice(0, 5).map((u) => (
              <li key={u.person_id} className="flex justify-between">
                <span style={{ color: "var(--tpl-text)" }}>
                  {u.display_name}
                  {u.age_turning != null ? ` (turns ${u.age_turning})` : ""}
                </span>
                <span style={{ color: "var(--tpl-muted)" }}>
                  {u.days_until === 0
                    ? "today 🎂"
                    : `in ${u.days_until} day${u.days_until === 1 ? "" : "s"}`}
                </span>
              </li>
            ))}
          </ul>
        </Card>
      )}

      {open && tab !== "suggestions" && (
        <Card className="mb-6 p-4">
          <form
            className="grid grid-cols-3 gap-2"
            onSubmit={(e) => {
              e.preventDefault();
              if (name.trim()) create.mutate();
            }}
          >
            <Input
              className="col-span-3"
              placeholder="Name (e.g. Mẹ)"
              value={name}
              onChange={setName}
            />
            <Input
              className="col-span-3"
              placeholder="Relationship (e.g. mẹ, bạn đại học)"
              value={relationship}
              onChange={setRelationship}
            />
            <Input placeholder="Day" value={day} onChange={setDay} numeric />
            <Input placeholder="Month" value={month} onChange={setMonth} numeric />
            <Input placeholder="Year (optional)" value={year} onChange={setYear} numeric />
            <button
              type="submit"
              disabled={create.isPending || !name.trim()}
              className="col-span-3 rounded-md py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-50"
              style={{
                background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))",
              }}
            >
              {create.isPending ? "Adding…" : `Add to ${CIRCLE_LABEL[circle ?? "other"]}`}
            </button>
          </form>
        </Card>
      )}

      {tab === "requests" ? (
        <RequestLists
          incoming={incoming.data ?? []}
          outgoing={outgoing.data ?? []}
          loading={incoming.isPending || outgoing.isPending}
          onAccept={(id) => accept.mutate(id)}
          onRemove={(id) => drop.mutate(id)}
          busy={accept.isPending || drop.isPending}
        />
      ) : tab === "suggestions" ? (
        <SuggestionList
          loading={suggestions.isPending}
          items={suggestions.data ?? []}
          onConnect={(userId) => connect.mutate(userId)}
          onAdd={(userId, displayName) => add.mutate({ userId, displayName })}
          busy={connect.isPending || add.isPending}
        />
      ) : people.isPending ? (
        <p style={{ color: "var(--tpl-muted)" }}>Loading…</p>
      ) : rows.length === 0 ? (
        <p style={{ color: "var(--tpl-muted)" }}>
          {circle
            ? `No one in ${CIRCLE_LABEL[circle]} yet — move someone here, or add a person.`
            : "No people yet — add family and friends to track birthdays."}
        </p>
      ) : (
        <Card>
          <ul className="divide-y" style={{ borderColor: "var(--tpl-border)" }}>
            {rows.map((p) => (
              <PersonListItem
                key={p.id}
                person={p}
                onMove={(to) => move.mutate({ id: p.id, to })}
                busy={move.isPending}
              />
            ))}
          </ul>
        </Card>
      )}
    </section>
  );
}

/* ── pieces ──────────────────────────────────────────────────────── */

function PersonListItem({
  person,
  onMove,
  busy,
}: {
  person: Person;
  onMove: (to: PersonCircle) => void;
  busy: boolean;
}) {
  return (
    <li className="flex items-center gap-3 px-4 py-3">
      <Avatar name={person.display_name} size={38} />
      <Link href={`/people/${person.id}` as Route} className="min-w-0 flex-1">
        <span className="block truncate font-medium" style={{ color: "var(--tpl-heading)" }}>
          {person.display_name}
        </span>
        {person.relationship && (
          <span className="block truncate text-xs" style={{ color: "var(--tpl-muted)" }}>
            {person.relationship}
          </span>
        )}
      </Link>
      {person.birthday && (
        <span className="hidden text-sm sm:inline" style={{ color: "var(--tpl-muted)" }}>
          🎂 {formatBirthday(person.birthday)}
        </span>
      )}
      {/* The whole point of the manage page: move someone between sections. */}
      <select
        value={person.circle}
        disabled={busy}
        onChange={(e) => onMove(e.target.value as PersonCircle)}
        aria-label={`Circle for ${person.display_name}`}
        className="rounded-md border bg-transparent px-2 py-1.5 text-xs outline-none disabled:opacity-50"
        style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }}
      >
        {CIRCLE_OPTIONS.map((c) => (
          <option key={c} value={c}>
            {CIRCLE_LABEL[c]}
          </option>
        ))}
      </select>
    </li>
  );
}

function SuggestionList({
  loading,
  items,
  onConnect,
  onAdd,
  busy,
}: {
  loading: boolean;
  items: { user_id: string; display_name: string }[];
  onConnect: (userId: string) => void;
  onAdd: (userId: string, displayName: string) => void;
  busy: boolean;
}) {
  if (loading) return <p style={{ color: "var(--tpl-muted)" }}>Looking for people…</p>;
  if (items.length === 0) {
    return (
      <p style={{ color: "var(--tpl-muted)" }}>
        No other accounts on this Portal yet — or you have added everyone already.
      </p>
    );
  }
  return (
    <Card>
      <ul className="divide-y" style={{ borderColor: "var(--tpl-border)" }}>
        {items.map((s) => (
          <li key={s.user_id} className="flex items-center gap-3 px-4 py-3">
            <Avatar name={s.display_name} size={38} />
            <span className="min-w-0 flex-1 truncate font-medium" style={{ color: "var(--tpl-heading)" }}>
              {s.display_name}
            </span>
            {/* Connecting is the social act — mutual, and it notifies them.
                Adding to the registry is the private one, kept beside it. */}
            <button
              type="button"
              disabled={busy}
              onClick={() => onConnect(s.user_id)}
              className="rounded-md px-3 py-1.5 text-xs font-semibold text-white transition hover:opacity-90 disabled:opacity-50"
              style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
            >
              Kết bạn
            </button>
            <button
              type="button"
              disabled={busy}
              onClick={() => onAdd(s.user_id, s.display_name)}
              className="rounded-md border px-3 py-1.5 text-xs font-semibold transition hover:bg-[var(--tpl-surface-2)] disabled:opacity-50"
              style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
            >
              Thêm vào sổ
            </button>
          </li>
        ))}
      </ul>
    </Card>
  );
}

/**
 * The two halves of "Lời mời": what is waiting on you, and what you are waiting
 * on. Separate lists because the only sensible button differs — you accept
 * theirs, you withdraw yours.
 */
function RequestLists({
  incoming,
  outgoing,
  loading,
  onAccept,
  onRemove,
  busy,
}: {
  incoming: Connection[];
  outgoing: Connection[];
  loading: boolean;
  onAccept: (id: string) => void;
  onRemove: (id: string) => void;
  busy: boolean;
}) {
  if (loading) return <p style={{ color: "var(--tpl-muted)" }}>Loading requests…</p>;
  if (incoming.length === 0 && outgoing.length === 0) {
    return <p style={{ color: "var(--tpl-muted)" }}>No requests waiting either way.</p>;
  }
  return (
    <div className="space-y-6">
      {incoming.length > 0 && (
        <RequestGroup title="Waiting on you">
          {incoming.map((c) => (
            <RequestRow key={c.id} connection={c} busy={busy}>
              <PrimaryBtn onClick={() => onAccept(c.id)} disabled={busy}>
                Chấp nhận
              </PrimaryBtn>
              <GhostBtn onClick={() => onRemove(c.id)} disabled={busy}>
                Từ chối
              </GhostBtn>
            </RequestRow>
          ))}
        </RequestGroup>
      )}

      {outgoing.length > 0 && (
        <RequestGroup title="You asked">
          {outgoing.map((c) => (
            <RequestRow key={c.id} connection={c} busy={busy}>
              <span className="text-xs" style={{ color: "var(--tpl-muted)" }}>
                Đang chờ
              </span>
              <GhostBtn onClick={() => onRemove(c.id)} disabled={busy}>
                Thu hồi
              </GhostBtn>
            </RequestRow>
          ))}
        </RequestGroup>
      )}
    </div>
  );
}

function RequestGroup({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <h2
        className="mb-2 text-xs font-bold uppercase tracking-wide"
        style={{ color: "var(--tpl-muted)" }}
      >
        {title}
      </h2>
      <Card>
        <ul className="divide-y" style={{ borderColor: "var(--tpl-border)" }}>
          {children}
        </ul>
      </Card>
    </div>
  );
}

function RequestRow({
  connection,
  children,
}: {
  connection: Connection;
  busy: boolean;
  children: React.ReactNode;
}) {
  return (
    <li className="flex items-center gap-3 px-4 py-3">
      <Avatar name={connection.display_name ?? "?"} size={38} />
      <span className="min-w-0 flex-1 truncate font-medium" style={{ color: "var(--tpl-heading)" }}>
        {connection.display_name}
      </span>
      {children}
    </li>
  );
}

function PrimaryBtn({
  children,
  onClick,
  disabled,
}: {
  children: React.ReactNode;
  onClick: () => void;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className="rounded-md px-3 py-1.5 text-xs font-semibold text-white transition hover:opacity-90 disabled:opacity-50"
      style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
    >
      {children}
    </button>
  );
}

function GhostBtn({
  children,
  onClick,
  disabled,
}: {
  children: React.ReactNode;
  onClick: () => void;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className="rounded-md border px-3 py-1.5 text-xs font-semibold transition hover:bg-[var(--tpl-surface-2)] disabled:opacity-50"
      style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
    >
      {children}
    </button>
  );
}

function Card({ children, className = "" }: { children: React.ReactNode; className?: string }) {
  return (
    <div
      className={`rounded-xl shadow-sm ${className}`}
      style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)" }}
    >
      {children}
    </div>
  );
}

function Input({
  value,
  onChange,
  placeholder,
  className = "",
  numeric,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder: string;
  className?: string;
  numeric?: boolean;
}) {
  return (
    <input
      value={value}
      onChange={(e) => onChange(e.target.value)}
      placeholder={placeholder}
      inputMode={numeric ? "numeric" : undefined}
      className={`rounded-md border bg-transparent px-3 py-2 text-sm outline-none focus:border-[var(--tpl-accent)] ${className}`}
      style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }}
    />
  );
}
