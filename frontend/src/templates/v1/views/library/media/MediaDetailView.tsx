"use client";

import "@vidstack/react/player/styles/default/theme.css";
import "@vidstack/react/player/styles/default/layouts/video.css";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { MediaPlayer, MediaProvider, type MediaPlayerInstance } from "@vidstack/react";
import {
  defaultLayoutIcons,
  DefaultVideoLayout,
} from "@vidstack/react/player/layouts/default";
import { Icon } from "../../../components/ui/Icon";
import {
  baseURL,
} from "@/lib/api-client";
import {
  getAsset,
  getPlaybackProgress,
  type PlaybackProgress,
} from "@/lib/media-assets";

// ── Beacon helper ──────────────────────────────────────────────────
// pagehide/visibilitychange cannot reliably use fetch (browser may cancel it).
// Spec mandates navigator.sendBeacon with a JSON Blob, or a keepalive fetch.
function sendProgressBeacon(assetId: string, positionMs: number): void {
  const url = `${baseURL}/api/v1/assets/${assetId}/progress`;
  const body = JSON.stringify({ position_ms: positionMs });

  if (typeof navigator !== "undefined" && navigator.sendBeacon) {
    // sendBeacon defaults to text/plain — we need application/json for the handler.
    const blob = new Blob([body], { type: "application/json" });
    navigator.sendBeacon(url, blob);
  } else {
    // Fallback: keepalive fetch (Safari ≤ 15 sendBeacon + Blob quirk).
    fetch(url, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body,
      credentials: "include",
      keepalive: true,
    }).catch(() => {});
  }
}

// ── Regular (non-beacon) progress PUT ──────────────────────────────
function putProgressFetch(assetId: string, positionMs: number): void {
  const url = `${baseURL}/api/v1/assets/${assetId}/progress`;
  fetch(url, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ position_ms: positionMs }),
    credentials: "include",
  }).catch(() => {});
}

// ── Resume gating ──────────────────────────────────────────────────
// P0.4: Seek to saved position only if ≥ 30s AND < 95%. When progress_pct is
// null (duration_ms unknown), skip the 95% gate — any position ≥ 30s resumes.
// ≥ 95% = completed → start at 0 ("Start over" implied).
function shouldResume(progress: PlaybackProgress | null | undefined): boolean {
  if (!progress || progress.position_ms < 30_000) return false;
  if (progress.progress_pct == null) return true; // unknown duration — resume unconditionally
  return progress.progress_pct < 95;
}

const THROTTLE_MS = 10_000; // spec: ~10 s between beacons

export function MediaDetailView({ id }: { id: string }) {
  const playerRef = useRef<MediaPlayerInstance>(null);
  const lastSyncRef = useRef<number>(0);
  const lastTimeRef = useRef<number>(0);
  const [ready, setReady] = useState(false);
  const [showStartOver, setShowStartOver] = useState(false);
  const [didSeek, setDidSeek] = useState(false);

  const { data: asset, isLoading: assetLoading } = useQuery({
    queryKey: ["assets", id],
    queryFn: () => getAsset(id),
  });

  const { data: progress, isLoading: progressLoading } = useQuery({
    queryKey: ["assets", id, "progress"],
    queryFn: () => getPlaybackProgress(id).catch(() => null), // 404 is fine
  });

  // Resume gating: set start time once player is ready.
  useEffect(() => {
    if (!ready || !playerRef.current || didSeek) return;
    if (shouldResume(progress)) {
      playerRef.current.currentTime = progress!.position_ms / 1000;
      setShowStartOver(true);
    }
    // Mark that we've applied the seek (or skipped it) so we don't repeat.
    setDidSeek(true);
  }, [ready, progress, didSeek]);

  // ── Pagehide / visibilitychange beacon ──────────────────────────
  const sendFinalBeacon = useCallback(() => {
    const t = lastTimeRef.current;
    if (t > 0) {
      sendProgressBeacon(id, Math.floor(t * 1000));
    }
  }, [id]);

  useEffect(() => {
    const onVisChange = () => {
      if (document.visibilityState === "hidden") sendFinalBeacon();
    };
    const onPageHide = () => sendFinalBeacon();

    document.addEventListener("visibilitychange", onVisChange);
    window.addEventListener("pagehide", onPageHide);
    return () => {
      document.removeEventListener("visibilitychange", onVisChange);
      window.removeEventListener("pagehide", onPageHide);
    };
  }, [sendFinalBeacon]);

  if (assetLoading || progressLoading)
    return <div className="p-4 text-white">Loading…</div>;
  if (!asset)
    return <div className="p-4 text-white">Asset not found</div>;

  const handleTimeUpdate = () => {
    const time = playerRef.current?.currentTime || 0;
    lastTimeRef.current = time;
    const now = Date.now();
    if (now - lastSyncRef.current > THROTTLE_MS) {
      lastSyncRef.current = now;
      putProgressFetch(id, Math.floor(time * 1000));
    }
  };

  const handlePause = () => {
    lastSyncRef.current = Date.now();
    putProgressFetch(id, Math.floor(lastTimeRef.current * 1000));
  };

  const handleStartOver = () => {
    if (playerRef.current) {
      playerRef.current.currentTime = 0;
      setShowStartOver(false);
    }
  };

  return (
    <main className="min-h-screen bg-black relative">
      <header className="absolute top-0 left-0 right-0 z-10 flex items-center p-4 bg-gradient-to-b from-black/60 to-transparent">
        <Link
          href="/library/media"
          className="p-2 rounded-full hover:bg-white/10 text-white"
        >
          <Icon name="arrow-left" className="w-6 h-6" />
        </Link>
        <h1 className="ml-4 text-white font-medium truncate">
          {asset.title || asset.original_filename || "Untitled"}
        </h1>

        {showStartOver && (
          <button
            onClick={handleStartOver}
            className="ml-auto px-4 py-1.5 text-sm font-medium rounded-md bg-white/10 hover:bg-white/20 text-white backdrop-blur-sm transition-colors"
          >
            Start over
          </button>
        )}
      </header>

      <div className="w-full h-screen flex items-center justify-center">
        {asset.kind === "video" || asset.kind === "audio" ? (
          <MediaPlayer
            ref={playerRef}
            src={asset.hls_url || ""}
            crossOrigin
            onCanPlay={() => setReady(true)}
            onTimeUpdate={handleTimeUpdate}
            onPause={handlePause}
            onEnded={handlePause}
            className="w-full h-full max-h-screen"
          >
            <MediaProvider />
            <DefaultVideoLayout icons={defaultLayoutIcons} />
          </MediaPlayer>
        ) : (
          <img
            src={`${baseURL}/api/v1/assets/${id}/original`}
            alt={asset.title || ""}
            className="max-w-full max-h-screen object-contain"
          />
        )}
      </div>
    </main>
  );
}
