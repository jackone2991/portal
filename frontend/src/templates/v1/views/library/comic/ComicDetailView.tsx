"use client";

import { useState } from "react";
import Link from "next/link";
import type { Route } from "next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api-client";
import { createChapter, getComic, publishComic, unpublishComic, variantURL } from "@/lib/comic";
import { problemDisplayMessage } from "@/lib/problems";
import { ApiError } from "@/lib/api-client";

export function ComicDetailView({ id }: { id: string }) {
  const qc = useQueryClient();
  const { data: comic, isLoading } = useQuery({ queryKey: ["comic", id], queryFn: () => getComic(id) });
  const { data: me } = useQuery({ queryKey: ["me"], queryFn: () => api<{ id?: string }>("/api/v1/auth/me").catch((): { id?: string } => ({})) });
  const [chapterTitle, setChapterTitle] = useState("");
  const [err, setErr] = useState<string | null>(null);

  const invalidate = () => qc.invalidateQueries({ queryKey: ["comic", id] });
  const addChapter = useMutation({
    mutationFn: () => createChapter(id, { title: chapterTitle, sort_order: ((comic?.chapters.length ?? 0) + 1) * 10 }),
    onSuccess: () => { setChapterTitle(""); invalidate(); },
  });
  const publish = useMutation({
    mutationFn: () => (comic?.status === "published" ? unpublishComic(id) : publishComic(id)),
    onSuccess: () => { setErr(null); invalidate(); },
    onError: (e) => setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : "Could not change status"),
  });

  if (isLoading) return <div className="p-6 text-gray-400">Loading…</div>;
  if (!comic) return <div className="p-6 text-white">Comic not found</div>;

  const isOwner = me?.id && me.id === comic.owner_id;
  const continueHref =
    comic.progress
      ? (`/library/comic/${id}/read/${comic.progress.chapter_id}` as Route)
      : comic.chapters[0]
        ? (`/library/comic/${id}/read/${comic.chapters[0].id}` as Route)
        : null;

  return (
    <div className="text-white">
      <div className="flex gap-6">
        <div className="w-40 shrink-0">
          <div className="aspect-[3/4] overflow-hidden rounded-lg border border-gray-800 bg-gray-900">
            {comic.cover_asset_id ? (
              <img src={variantURL(comic.cover_asset_id, "medium")} alt={comic.title} className="h-full w-full object-cover" />
            ) : (
              <div className="flex h-full w-full items-center justify-center text-gray-600">No cover</div>
            )}
          </div>
        </div>
        <div className="flex-1">
          <div className="mb-1 flex items-center gap-2">
            <h1 className="text-2xl font-bold">{comic.title}</h1>
            <span className={`rounded px-1.5 py-0.5 text-[10px] uppercase ${comic.status === "published" ? "bg-green-700" : "bg-gray-700"}`}>{comic.status}</span>
          </div>
          {comic.description && <p className="mb-3 text-sm text-gray-400">{comic.description}</p>}
          <div className="flex gap-2">
            {continueHref && (
              <Link href={continueHref} className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium hover:bg-blue-500">
                {comic.progress ? "Continue reading" : "Start reading"}
              </Link>
            )}
            {isOwner && (
              <button onClick={() => publish.mutate()} disabled={publish.isPending} className="rounded-md bg-gray-700 px-4 py-2 text-sm font-medium hover:bg-gray-600 disabled:opacity-50">
                {comic.status === "published" ? "Unpublish" : "Publish"}
              </button>
            )}
          </div>
          {err && <p className="mt-2 text-sm text-red-400">{err}</p>}
        </div>
      </div>

      <h2 className="mb-2 mt-8 text-sm font-semibold uppercase text-gray-500">Chapters</h2>
      <ul className="divide-y divide-gray-800 rounded-lg border border-gray-800 bg-gray-900">
        {comic.chapters.map((ch, i) => (
          <li key={ch.id}>
            <Link href={`/library/comic/${id}/read/${ch.id}` as Route} className="flex justify-between px-4 py-3 text-sm hover:bg-gray-800">
              <span>{i + 1}. {ch.title}</span>
              <span className="text-gray-500">{new Date(ch.created_at).toLocaleDateString()}</span>
            </Link>
          </li>
        ))}
        {comic.chapters.length === 0 && <li className="px-4 py-4 text-center text-gray-500">No chapters yet.</li>}
      </ul>

      {isOwner && (
        <form className="mt-3 flex gap-2" onSubmit={(e) => { e.preventDefault(); if (chapterTitle.trim()) addChapter.mutate(); }}>
          <input className="flex-1 rounded-md border border-gray-700 bg-gray-800 px-3 py-2 text-sm" placeholder="New chapter title" value={chapterTitle} onChange={(e) => setChapterTitle(e.target.value)} />
          <button type="submit" disabled={addChapter.isPending || !chapterTitle.trim()} className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium disabled:opacity-50">Add chapter</button>
        </form>
      )}
    </div>
  );
}
