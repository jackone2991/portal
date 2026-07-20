"use client";

import Link from "next/link";
import type { Route } from "next";
import { useQuery } from "@tanstack/react-query";
import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";
import { WidgetCard } from "./WidgetCard";
import { listPeople } from "@/lib/people";

/**
 * People rail — real contacts from the people module (SPEC-08), replacing the
 * Olympus "Friend Suggestions" fixtures. Self-fetching (D-32: TanStack owns
 * server state); shows a compact empty state so the rail slot stays visible when
 * the registry is empty. Keeps the export name so HomeView's rail is unchanged.
 */
export function FriendSuggestions() {
  const { data } = useQuery({
    queryKey: ["people", "rail"],
    queryFn: () => listPeople(),
    retry: false,
  });
  const people = (data?.people ?? []).slice(0, 5);

  return (
    <WidgetCard title="People">
      {people.length === 0 ? (
        <Link
          href={"/people" as Route}
          className="block rounded-md py-1 text-xs hover:underline"
          style={{ color: "var(--tpl-muted)" }}
        >
          No people yet — add someone to your registry.
        </Link>
      ) : (
        <ul className="space-y-3">
          {people.map((p) => (
            <li key={p.id} className="flex items-center gap-3">
              <Avatar name={p.display_name} size={40} />
              <div className="min-w-0">
                <p className="truncate text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>
                  {p.display_name}
                </p>
                <p className="truncate text-xs capitalize" style={{ color: "var(--tpl-muted)" }}>
                  {p.relationship ?? "Contact"}
                </p>
              </div>
              <Link
                href={`/people/${p.id}` as Route}
                className="ml-auto grid h-8 w-8 place-items-center rounded-md text-white transition hover:opacity-90"
                style={{ background: "var(--tpl-blue)" }}
                aria-label={`Open ${p.display_name}`}
              >
                <Icon name="happy-face-icon" size={16} />
              </Link>
            </li>
          ))}
        </ul>
      )}
    </WidgetCard>
  );
}
