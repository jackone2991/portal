"use client";

import "@vidstack/react/player/styles/default/theme.css";
import "@vidstack/react/player/styles/default/layouts/video.css";

import { useEffect, useRef, useState } from "react";
import { MediaPlayer, MediaProvider } from "@vidstack/react";
import { defaultLayoutIcons, DefaultVideoLayout } from "@vidstack/react/player/layouts/default";
import { Icon } from "../../components/ui/Icon";
import { baseURL as API } from "@/lib/api-client";

type Phase = "idle" | "uploading" | "processing" | "ready" | "error";

type AssetJSON = {
  id: string;
  status: string;
  mime_type: string;
  size_bytes: number;
  duration_ms: number | null;
  width: number | null;
  height: number | null;
  hls_url?: string;
  error?: string;
  created_at: string;
};

/**
 * Upload → transcode → playback studio (v1 media slice). The browser uploads the
 * original through the API (POST /assets → PUT /assets/{id}/source → complete),
 * polls until the worker's HLS output is ready, then plays it with Vidstack.
 */
export function UploadStudio() {
  const [phase, setPhase] = useState<Phase>("idle");
  const [progress, setProgress] = useState(0);
  const [current, setCurrent] = useState<AssetJSON | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [library, setLibrary] = useState<AssetJSON[]>([]);
  const fileRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    void refreshLibrary();
  }, []);

  async function refreshLibrary() {
    try {
      const r = await fetch(`${API}/api/v1/assets`, { credentials: "include" });
      if (r.ok) setLibrary(((await r.json()).assets as AssetJSON[]) ?? []);
    } catch {
      /* ignore */
    }
  }

  async function onFile(file: File) {
    setError(null);
    setCurrent(null);
    setPhase("uploading");
    setProgress(0);
    try {
      // 1. create the asset + session
      const cr = await fetch(`${API}/api/v1/assets`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          filename: file.name,
          content_type: file.type || "video/mp4",
          size_bytes: file.size,
        }),
      });
      if (!cr.ok) throw new Error("Could not start upload.");
      const { asset } = (await cr.json()) as { asset: AssetJSON };

      // 2. upload the original (proxied through the API, with progress)
      await putWithProgress(
        `${API}/api/v1/assets/${asset.id}/source`,
        file,
        setProgress,
      );

      // 3. confirm → enqueue transcode
      const co = await fetch(`${API}/api/v1/assets/${asset.id}/complete`, {
        method: "POST",
        credentials: "include",
      });
      if (!co.ok) throw new Error("Could not finalise upload.");

      // 4. poll until the worker finishes
      setPhase("processing");
      const done = await poll(asset.id);
      if (done.status !== "ready") throw new Error(done.error || "Transcode failed.");
      setCurrent(done);
      setPhase("ready");
      void refreshLibrary();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Upload failed.");
      setPhase("error");
    }
  }

  function reset() {
    setPhase("idle");
    setCurrent(null);
    setError(null);
    setProgress(0);
    if (fileRef.current) fileRef.current.value = "";
  }

  return (
    <div className="mx-auto max-w-3xl space-y-5">
      <Card>
        <h1 className="text-lg font-bold" style={{ color: "var(--tpl-heading)" }}>
          Upload a video
        </h1>
        <p className="mt-1 text-sm" style={{ color: "var(--tpl-muted)" }}>
          Pick an MP4. It uploads, transcodes to HLS on the worker, then plays back here.
        </p>

        {phase === "idle" || phase === "error" ? (
          <div className="mt-4">
            <button
              type="button"
              onClick={() => fileRef.current?.click()}
              className="flex w-full flex-col items-center gap-2 rounded-xl border-2 border-dashed py-10 transition hover:bg-[var(--tpl-surface-2)]"
              style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
            >
              <Icon name="camera-icon" size={28} />
              <span className="text-sm font-medium">Choose a video file</span>
            </button>
            <input
              ref={fileRef}
              type="file"
              accept="video/*"
              className="hidden"
              onChange={(e) => {
                const f = e.target.files?.[0];
                if (f) void onFile(f);
              }}
            />
            {error && (
              <p className="mt-3 rounded-lg border px-3 py-2 text-sm" style={{ borderColor: "rgba(239,68,68,.4)", background: "rgba(239,68,68,.08)", color: "#ef4444" }}>
                {error}
              </p>
            )}
          </div>
        ) : phase === "uploading" ? (
          <Progress label={`Uploading… ${progress}%`} value={progress} />
        ) : phase === "processing" ? (
          <Progress label="Transcoding to HLS…" indeterminate />
        ) : (
          current && (
            <div className="mt-4 space-y-3">
              <Player src={current.hls_url!} />
              <div className="flex items-center justify-between text-xs" style={{ color: "var(--tpl-muted)" }}>
                <span>
                  {current.width}×{current.height} · {Math.round((current.duration_ms ?? 0) / 1000)}s ·{" "}
                  {(current.size_bytes / 1024 / 1024).toFixed(1)} MB
                </span>
                <button type="button" onClick={reset} className="font-semibold" style={{ color: "var(--tpl-accent)" }}>
                  Upload another
                </button>
              </div>
            </div>
          )
        )}
      </Card>

      {library.length > 0 && (
        <Card>
          <h2 className="mb-3 text-sm font-bold" style={{ color: "var(--tpl-heading)" }}>
            Your uploads
          </h2>
          <ul className="divide-y" style={{ borderColor: "var(--tpl-border)" }}>
            {library.map((a) => (
              <li key={a.id} className="flex items-center gap-3 py-2.5">
                <span className="grid h-8 w-8 place-items-center rounded-md" style={{ background: "var(--tpl-surface-2)", color: "var(--tpl-muted)" }}>
                  <Icon name="play-icon" size={14} />
                </span>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm" style={{ color: "var(--tpl-heading)" }}>
                    {a.id.slice(0, 8)} · {a.mime_type}
                  </p>
                  <p className="text-xs" style={{ color: "var(--tpl-muted)" }}>
                    {a.status}
                    {a.width ? ` · ${a.width}×${a.height}` : ""}
                  </p>
                </div>
                {a.status === "ready" && a.hls_url && (
                  <button
                    type="button"
                    onClick={() => {
                      setCurrent(a);
                      setPhase("ready");
                      window.scrollTo({ top: 0, behavior: "smooth" });
                    }}
                    className="rounded-md px-3 py-1 text-xs font-semibold text-white"
                    style={{ background: "var(--tpl-accent)" }}
                  >
                    Play
                  </button>
                )}
              </li>
            ))}
          </ul>
        </Card>
      )}
    </div>
  );
}

/* ── pieces ──────────────────────────────────────────────────────── */

function Player({ src }: { src: string }) {
  return (
    <MediaPlayer
      className="w-full overflow-hidden rounded-xl"
      title="Video"
      src={{ src, type: "application/vnd.apple.mpegurl" }}
      playsInline
    >
      <MediaProvider />
      <DefaultVideoLayout icons={defaultLayoutIcons} />
    </MediaPlayer>
  );
}

function Progress({ label, value, indeterminate }: { label: string; value?: number; indeterminate?: boolean }) {
  return (
    <div className="mt-6">
      <p className="mb-2 text-sm font-medium" style={{ color: "var(--tpl-text)" }}>
        {label}
      </p>
      <div className="h-2 overflow-hidden rounded-full" style={{ background: "var(--tpl-surface-2)" }}>
        <div
          className={`h-full rounded-full ${indeterminate ? "animate-pulse" : ""}`}
          style={{
            width: indeterminate ? "100%" : `${value ?? 0}%`,
            background: "linear-gradient(90deg, var(--tpl-accent), var(--tpl-accent-2))",
            transition: "width .2s",
          }}
        />
      </div>
    </div>
  );
}

function Card({ children }: { children: React.ReactNode }) {
  return (
    <div className="rounded-xl p-5 shadow-sm" style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)" }}>
      {children}
    </div>
  );
}

/* ── upload/poll helpers ─────────────────────────────────────────── */

function putWithProgress(url: string, file: File, onProgress: (pct: number) => void): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PUT", url);
    xhr.withCredentials = true; // send the session cookie cross-subdomain
    xhr.setRequestHeader("Content-Type", file.type || "video/mp4");
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) onProgress(Math.round((e.loaded / e.total) * 100));
    };
    xhr.onload = () =>
      xhr.status >= 200 && xhr.status < 300 ? resolve() : reject(new Error(`Upload failed (${xhr.status}).`));
    xhr.onerror = () => reject(new Error("Upload network error."));
    xhr.send(file);
  });
}

async function poll(id: string): Promise<AssetJSON> {
  for (let i = 0; i < 120; i++) {
    const r = await fetch(`${API}/api/v1/assets/${id}`, { credentials: "include" });
    if (r.ok) {
      const a = (await r.json()) as AssetJSON;
      if (a.status === "ready" || a.status === "failed") return a;
    }
    await new Promise((res) => setTimeout(res, 1500));
  }
  return { status: "failed", error: "Timed out waiting for transcode." } as AssetJSON;
}
