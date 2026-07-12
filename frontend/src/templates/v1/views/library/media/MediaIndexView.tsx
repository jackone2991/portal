"use client";

import "@vidstack/react/player/styles/default/theme.css";
import "@vidstack/react/player/styles/default/layouts/video.css";

import { useMemo, useState, type ReactNode } from "react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import type { Route } from "next";
import {
  useInfiniteQuery,
  useMutation,
  useQueryClient,
  type InfiniteData,
} from "@tanstack/react-query";
import { MediaPlayer, MediaProvider } from "@vidstack/react";
import {
  defaultLayoutIcons,
  DefaultVideoLayout,
} from "@vidstack/react/player/layouts/default";
import { Icon } from "../../../components/ui/Icon";
import { SelectField } from "../../../components/form/SelectField";
import { Modal, BtnSecondary } from "../../../components/popup/Modal";
import { ApiError } from "@/lib/api-client";
import { problemDisplayMessage } from "@/lib/problems";
import {
  assetOriginalURL,
  assetVariantURL,
  deleteAsset,
  listAssets,
  type AssetKind,
  type AssetKindFilter,
  type AssetStatus,
  type AssetStatusFilter,
  type ListAssetsPage,
  type MediaAsset,
} from "@/lib/media-assets";

const KIND_OPTIONS = [
  { value: "all", label: "All kinds" },
  { value: "video", label: "Video" },
  { value: "image", label: "Image" },
];

const STATUS_OPTIONS = [
  { value: "all", label: "All statuses" },
  { value: "ready", label: "Ready" },
  { value: "processing", label: "Processing" },
  { value: "failed", label: "Failed" },
];

const KIND_ICON: Record<AssetKind, string> = {
  video: "play-icon",
  image: "photos-icon",
  audio: "multimedia-icon",
};

const KIND_LABEL: Record<AssetKind, string> = {
  video: "Video",
  image: "Image",
  audio: "Audio",
};

/** Root query key for the whole asset-list cache (all filter combinations). */
const ASSETS_KEY = "assets" as const;

type Viewer = { asset: MediaAsset; mode: "player" | "lightbox" };

/**
 * Library · Media index — SPEC-01 P0.4 ([F006]). No Blade equivalent (the
 * Olympus reference predates the media pipeline); this is a fresh v1 view.
 * Rendered inside MasterBase.
 *
 * Grid of the caller's assets (thumb variants only — never originals, per the
 * P0.4 LCP acceptance criterion), with kind/status filters kept in the URL
 * (shareable filter state, D-32) and cursor pagination via TanStack's
 * `useInfiniteQuery`. Delete is an optimistic TanStack cache mutation — no
 * full refetch on success.
 */
export function MediaIndexView() {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const queryClient = useQueryClient();

  const kind = (searchParams.get("kind") as AssetKindFilter | null) ?? "all";
  const status = (searchParams.get("status") as AssetStatusFilter | null) ?? "all";

  const [viewer, setViewer] = useState<Viewer | null>(null);
  const [pendingDelete, setPendingDelete] = useState<MediaAsset | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  function setFilter(next: { kind?: AssetKindFilter; status?: AssetStatusFilter }) {
    const merged = { kind, status, ...next };
    const params = new URLSearchParams(searchParams.toString());
    if (merged.kind === "all") params.delete("kind");
    else params.set("kind", merged.kind);
    if (merged.status === "all") params.delete("status");
    else params.set("status", merged.status);
    const qs = params.toString();
    router.replace((qs ? `${pathname}?${qs}` : pathname) as Route);
  }

  const query = useInfiniteQuery({
    queryKey: [ASSETS_KEY, kind, status],
    queryFn: ({ pageParam }: { pageParam: string | undefined }) =>
      listAssets({ kind, status, cursor: pageParam }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage.next_cursor ?? undefined,
  });

  const assets = useMemo(
    () => query.data?.pages.flatMap((page) => page.assets) ?? [],
    [query.data],
  );

  const deleteMutation = useMutation({
    mutationFn: (asset: MediaAsset) => deleteAsset(asset.id),
    onMutate: async (asset) => {
      await queryClient.cancelQueries({ queryKey: [ASSETS_KEY] });
      const previous = queryClient.getQueriesData<InfiniteData<ListAssetsPage>>({
        queryKey: [ASSETS_KEY],
      });
      queryClient.setQueriesData<InfiniteData<ListAssetsPage>>(
        { queryKey: [ASSETS_KEY] },
        (data) =>
          data && {
            ...data,
            pages: data.pages.map((page) => ({
              ...page,
              assets: page.assets.filter((a) => a.id !== asset.id),
            })),
          },
      );
      return { previous };
    },
    onError: (err, _asset, context) => {
      context?.previous.forEach(([key, data]) => queryClient.setQueryData(key, data));
      setDeleteError(
        err instanceof ApiError
          ? problemDisplayMessage(err.body)
          : "Could not delete this asset.",
      );
    },
    onSuccess: (_data, asset) => {
      setPendingDelete(null);
      setViewer((v) => (v && v.asset.id === asset.id ? null : v));
    },
  });

  function openDelete(asset: MediaAsset) {
    setDeleteError(null);
    setPendingDelete(asset);
  }

  return (
    <section>
      <header className="mb-6 flex flex-wrap items-center justify-between gap-4">
        <h1 className="text-2xl font-semibold" style={{ color: "var(--tpl-heading)" }}>
          Media
        </h1>
        <div className="flex gap-4">
          <div className="w-36">
            <SelectField
              label="Kind"
              value={kind}
              onChange={(v) => setFilter({ kind: v as AssetKindFilter })}
              options={KIND_OPTIONS}
            />
          </div>
          <div className="w-36">
            <SelectField
              label="Status"
              value={status}
              onChange={(v) => setFilter({ status: v as AssetStatusFilter })}
              options={STATUS_OPTIONS}
            />
          </div>
        </div>
      </header>

      {deleteError && <ErrorBanner>{deleteError}</ErrorBanner>}

      {query.isPending ? (
        <SkeletonGrid />
      ) : query.isError ? (
        <QueryErrorState onRetry={() => query.refetch()} />
      ) : assets.length === 0 ? (
        <EmptyState />
      ) : (
        <>
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
            {assets.map((asset) => (
              <AssetCard
                key={asset.id}
                asset={asset}
                onOpen={() =>
                  setViewer({ asset, mode: asset.kind === "image" ? "lightbox" : "player" })
                }
                onDelete={() => openDelete(asset)}
              />
            ))}
          </div>

          {query.hasNextPage && (
            <div className="mt-6 flex justify-center">
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

      {viewer && (
        <ViewerModal
          asset={viewer.asset}
          mode={viewer.mode}
          onClose={() => setViewer(null)}
          onDelete={() => openDelete(viewer.asset)}
        />
      )}

      {pendingDelete && (
        <Modal open onClose={() => setPendingDelete(null)} title="Delete asset?" width={420}>
          <div className="space-y-4 p-6">
            <p className="text-sm" style={{ color: "var(--tpl-text)" }}>
              {assetLabel(pendingDelete)} will be permanently deleted. This can&apos;t be undone.
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
    </section>
  );
}

/* ── pieces ──────────────────────────────────────────────────────── */

function assetLabel(asset: MediaAsset): string {
  return asset.title || asset.original_filename || `Asset ${asset.id.slice(0, 8)}`;
}

function AssetCard({
  asset,
  onOpen,
  onDelete,
}: {
  asset: MediaAsset;
  onOpen: () => void;
  onDelete: () => void;
}) {
  const isFailed = asset.status === "failed";
  const isReady = asset.status === "ready";
  const label = assetLabel(asset);

  return (
    <div
      className="flex flex-col overflow-hidden rounded-lg border"
      style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
    >
      <button
        type="button"
        onClick={onOpen}
        disabled={isFailed}
        aria-label={isFailed ? undefined : `Open ${label}`}
        className="group relative aspect-video w-full overflow-hidden disabled:cursor-default"
        style={{ background: "var(--tpl-surface-2)" }}
      >
        {isReady ? (
          <img
            src={assetVariantURL(asset.id, "thumb")}
            alt={label}
            loading="lazy"
            className="h-full w-full object-cover transition group-hover:scale-[1.03]"
          />
        ) : (
          <div
            className="flex h-full w-full items-center justify-center"
            style={{ color: "var(--tpl-muted)" }}
          >
            <Icon name={isFailed ? "close-icon" : KIND_ICON[asset.kind]} size={22} />
          </div>
        )}
        <span className="absolute left-2 top-2">
          <Pill>{KIND_LABEL[asset.kind]}</Pill>
        </span>
        <span className="absolute right-2 top-2">
          <StatusPill status={asset.status} />
        </span>
      </button>

      <div className="flex flex-1 flex-col gap-1 p-3">
        <p
          className="truncate text-sm font-semibold"
          style={{ color: "var(--tpl-heading)" }}
          title={label}
        >
          {label}
        </p>
        <p className="text-xs" style={{ color: "var(--tpl-muted)" }}>
          {new Date(asset.created_at).toLocaleDateString()}
        </p>
        {isFailed && asset.error && (
          <p className="mt-1 line-clamp-2 text-xs" style={{ color: "#ef4444" }} title={asset.error}>
            {asset.error}
          </p>
        )}

        <div className="mt-2 flex items-center gap-3">
          {!isFailed && (
            <a
              href={assetOriginalURL(asset.id)}
              download
              className="text-xs font-semibold transition hover:opacity-80"
              style={{ color: "var(--tpl-accent)" }}
            >
              Download original
            </a>
          )}
          <button
            type="button"
            onClick={onDelete}
            aria-label={`Delete ${label}`}
            className="ml-auto transition hover:text-[#ef4444]"
            style={{ color: "var(--tpl-muted)" }}
          >
            <Icon name="little-delete" size={16} />
          </button>
        </div>
      </div>
    </div>
  );
}

function Pill({ children, color }: { children: ReactNode; color?: string }) {
  return (
    <span
      className="rounded-full px-2 py-0.5 text-[11px] font-semibold"
      style={{
        background: "rgba(0,0,0,.55)",
        color: color ?? "#fff",
      }}
    >
      {children}
    </span>
  );
}

const STATUS_STYLE: Record<AssetStatus, { label: string; color: string }> = {
  ready: { label: "Ready", color: "#22c55e" },
  processing: { label: "Processing", color: "#38a9ff" },
  uploading: { label: "Uploading", color: "#38a9ff" },
  failed: { label: "Failed", color: "#ef4444" },
};

function StatusPill({ status }: { status: AssetStatus }) {
  const s = STATUS_STYLE[status];
  return <Pill color={s.color}>{s.label}</Pill>;
}

function ViewerModal({
  asset,
  mode,
  onClose,
  onDelete,
}: {
  asset: MediaAsset;
  mode: "player" | "lightbox";
  onClose: () => void;
  onDelete: () => void;
}) {
  return (
    <Modal open onClose={onClose} title={assetLabel(asset)} width={mode === "player" ? 880 : 720}>
      <div className="space-y-3 p-6">
        {mode === "player" ? (
          asset.hls_url ? (
            <MediaPlayer
              className="w-full overflow-hidden rounded-xl"
              title={assetLabel(asset)}
              src={{ src: asset.hls_url, type: "application/vnd.apple.mpegurl" }}
              playsInline
            >
              <MediaProvider />
              <DefaultVideoLayout icons={defaultLayoutIcons} />
            </MediaPlayer>
          ) : (
            <p className="text-sm" style={{ color: "var(--tpl-muted)" }}>
              This video isn&apos;t ready to play yet.
            </p>
          )
        ) : (
          // eslint-disable-next-line @next/next/no-img-element -- dynamic, API-proxied variant, not a static/optimizable asset
          <img
            src={assetVariantURL(asset.id, "medium")}
            alt={assetLabel(asset)}
            className="max-h-[70vh] w-full rounded-lg object-contain"
            style={{ background: "var(--tpl-surface-2)" }}
          />
        )}

        <div className="flex items-center justify-between">
          <a
            href={assetOriginalURL(asset.id)}
            download
            className="text-sm font-semibold transition hover:opacity-80"
            style={{ color: "var(--tpl-accent)" }}
          >
            Download original
          </a>
          <button
            type="button"
            onClick={onDelete}
            className="flex items-center gap-1.5 text-sm font-semibold transition hover:opacity-80"
            style={{ color: "#ef4444" }}
          >
            <Icon name="little-delete" size={14} />
            Delete
          </button>
        </div>
      </div>
    </Modal>
  );
}

function SkeletonGrid() {
  return (
    <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
      {Array.from({ length: 10 }).map((_, i) => (
        <div
          key={i}
          className="aspect-[4/5] animate-pulse rounded-lg border"
          style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
        />
      ))}
    </div>
  );
}

function EmptyState() {
  return (
    <div
      className="flex flex-col items-center gap-3 rounded-xl border border-dashed py-16 text-center"
      style={{ borderColor: "var(--tpl-border)" }}
    >
      <Icon name="photos-icon" size={28} style={{ color: "var(--tpl-muted)" }} />
      <p className="text-sm" style={{ color: "var(--tpl-muted)" }}>
        No media yet.
      </p>
      <Link
        href="/upload"
        className="rounded-md px-3 py-1.5 text-sm font-semibold"
        style={{ background: "var(--tpl-accent)", color: "var(--tpl-accent-contrast)" }}
      >
        Upload something
      </Link>
    </div>
  );
}

function QueryErrorState({ onRetry }: { onRetry: () => void }) {
  return (
    <div className="rounded-xl border py-12 text-center" style={{ borderColor: "var(--tpl-border)" }}>
      <p className="text-sm" style={{ color: "var(--tpl-muted)" }}>
        Couldn&apos;t load your media.
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
      className="mb-4 rounded-lg border px-3 py-2 text-sm"
      style={{ borderColor: "rgba(239,68,68,.4)", background: "rgba(239,68,68,.08)", color: "#ef4444" }}
    >
      {children}
    </p>
  );
}
