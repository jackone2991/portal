"use client";

import { useState } from "react";
import Link from "next/link";
import type { Route } from "next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type Comic, createComic, listComics, listMyComics, variantURL } from "@/lib/comic";

// Library · Comics (SPEC-02 P0.5) — published grid + a "My comics" tab (incl.
// drafts) with a create action. Replaces the placeholder.
export function ComicIndexView() {
  const qc = useQueryClient();
  const [tab, setTab] = useState<"all" | "mine">("all");
  const [title, setTitle] = useState("");
  const [creating, setCreating] = useState(false);

  const all = useQuery({ queryKey: ["comics", "published"], queryFn: () => listComics() });
  const mine = useQuery({ queryKey: ["comics", "mine"], queryFn: () => listMyComics(), enabled: tab === "mine" });

  const create = useMutation({
    mutationFn: () => createComic({ title }),
    onSuccess: () => {
      setTitle("");
      setCreating(false);
      qc.invalidateQueries({ queryKey: ["comics"] });
      setTab("mine");
    },
  });

  const comics = tab === "all" ? all.data?.comics ?? [] : mine.data?.comics ?? [];

  return (
    <section className="text-white">
      <header className="mb-6 flex items-center justify-between">
        <div className="flex gap-2">
          <button className={`rounded-md px-3 py-1.5 text-sm ${tab === "all" ? "bg-blue-600" : "bg-gray-800"}`} onClick={() => setTab("all")}>Library</button>
          <button className={`rounded-md px-3 py-1.5 text-sm ${tab === "mine" ? "bg-blue-600" : "bg-gray-800"}`} onClick={() => setTab("mine")}>My comics</button>
        </div>
        <button className="rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium hover:bg-blue-500" onClick={() => setCreating((c) => !c)}>+ New comic</button>
      </header>

      {creating && (
        <form
          className="mb-6 flex gap-2 rounded-lg border border-gray-800 bg-gray-900 p-3"
          onSubmit={(e) => { e.preventDefault(); if (title.trim()) create.mutate(); }}
        >
          <input className="flex-1 rounded-md border border-gray-700 bg-gray-800 px-3 py-2" placeholder="Comic title" value={title} onChange={(e) => setTitle(e.target.value)} />
          <button type="submit" disabled={create.isPending || !title.trim()} className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium disabled:opacity-50">Create</button>
        </form>
      )}

      {comics.length === 0 ? (
        <p className="text-gray-500">{tab === "all" ? "No published comics yet." : "You haven't created any comics yet."}</p>
      ) : (
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
          {comics.map((c) => (
            <ComicCard key={c.id} comic={c} showStatus={tab === "mine"} />
          ))}
        </div>
      )}
    </section>
  );
}

function ComicCard({ comic, showStatus }: { comic: Comic; showStatus: boolean }) {
  return (
    <Link href={`/library/comic/${comic.id}` as Route} className="group block">
      <div className="relative aspect-[3/4] overflow-hidden rounded-lg border border-gray-800 bg-gray-900">
        {comic.cover_asset_id ? (
          <img src={variantURL(comic.cover_asset_id, "thumb")} alt={comic.title} className="h-full w-full object-cover" />
        ) : (
          <div className="flex h-full w-full items-center justify-center text-gray-600">No cover</div>
        )}
        {showStatus && (
          <span className={`absolute left-2 top-2 rounded px-1.5 py-0.5 text-[10px] uppercase ${comic.status === "published" ? "bg-green-700" : "bg-gray-700"}`}>{comic.status}</span>
        )}
      </div>
      <div className="mt-1.5 truncate text-sm font-medium text-white group-hover:text-blue-400">{comic.title}</div>
      <div className="text-xs text-gray-500">{comic.chapter_count ?? 0} chapters</div>
    </Link>
  );
}
