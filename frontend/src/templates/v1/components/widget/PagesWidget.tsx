"use client";

import Link from "next/link";
import type { Route } from "next";
import { useQuery } from "@tanstack/react-query";
import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";
import { WidgetCard } from "./WidgetCard";
import { listComics } from "@/lib/comic";

/**
 * Discover rail — real published comics from the comic module (SPEC-02),
 * replacing the Olympus "Pages You May Like" fixtures. Self-fetching (D-32); a
 * compact empty state keeps the slot visible when nothing is published yet.
 * Keeps the export name so HomeView's rail is unchanged.
 */
export function PagesWidget() {
  const { data } = useQuery({
    queryKey: ["comics", "discover"],
    queryFn: () => listComics(),
    retry: false,
  });
  const comics = (data?.comics ?? []).slice(0, 4);

  return (
    <WidgetCard title="Discover">
      {comics.length === 0 ? (
        <Link
          href={"/library/comic" as Route}
          className="block rounded-md py-1 text-xs hover:underline"
          style={{ color: "var(--tpl-muted)" }}
        >
          Nothing published yet — browse the library.
        </Link>
      ) : (
        <ul className="space-y-3">
          {comics.map((c) => (
            <li key={c.id} className="flex items-center gap-3">
              <Avatar name={c.title} size={38} />
              <div className="min-w-0">
                <p className="truncate text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>
                  {c.title}
                </p>
                <p className="truncate text-xs" style={{ color: "var(--tpl-muted)" }}>
                  {c.chapter_count ? `${c.chapter_count} chapter${c.chapter_count === 1 ? "" : "s"}` : "Comic"}
                </p>
              </div>
              <Link
                href={`/library/comic/${c.id}` as Route}
                className="ml-auto text-[var(--tpl-muted)] transition hover:text-[var(--tpl-accent)]"
                aria-label={`Open ${c.title}`}
              >
                <Icon name="star-icon" size={18} />
              </Link>
            </li>
          ))}
        </ul>
      )}
    </WidgetCard>
  );
}
