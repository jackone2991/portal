"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import { Icon } from "../../../components/ui/Icon";
import { ApiError } from "@/lib/api-client";
import { problemDisplayMessage } from "@/lib/problems";
import {
  createTrack,
  enrichImport,
  getImport,
  importInFlight,
  importZip,
  lookupImport,
  metaFromFilename,
  uploadAudioAsset,
  type MusicImport,
} from "@/lib/music";

/**
 * Library · Nhạc · nhập nhiều bài — the two bulk paths, in one dialog.
 *
 * They exist separately because they fail differently, not to offer a choice for
 * its own sake:
 *
 *   FILES  — the browser uploads each one and creates a track per file. Every
 *            step is a request the user is watching, so a failure is attributable
 *            to a filename and the rest still import. Right for a handful.
 *   ZIP    — one upload, then a worker unpacks it. Survives closing the tab and
 *            costs two requests instead of two hundred. Right for a library.
 *
 * The cut-over is about round trips, so the dialog picks by count rather than
 * asking: dropping 300 files would mean 600 sequential requests held open by a
 * tab that must stay focused.
 */

/** Above this many files, the dialog steers to the zip path. */
const BULK_HINT_THRESHOLD = 25;

/** How often to re-poll a running import. */
const POLL_MS = 1500;

type FileState = {
  file: File;
  status: "waiting" | "uploading" | "done" | "failed";
  pct: number;
  error?: string;
  title?: string;
};

export function BulkImportModal({
  onClose,
  onImported,
}: {
  onClose: () => void;
  /** Fired whenever tracks were created, so the list behind can refetch. */
  onImported: () => void;
}) {
  const [mode, setMode] = useState<"files" | "zip">("files");
  const [err, setErr] = useState<string | null>(null);

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-label="Nhập nhiều bài hát"
    >
      <div
        className="flex max-h-[85vh] w-full max-w-2xl flex-col rounded-xl p-6 shadow-lg"
        style={{ background: "var(--tpl-surface)" }}
        onClick={(e) => e.stopPropagation()}
      >
        <h2 className="mb-1 text-lg font-semibold" style={{ color: "var(--tpl-heading)" }}>
          Nhập nhiều bài hát
        </h2>
        <p className="mb-4 text-xs" style={{ color: "var(--tpl-muted)" }}>
          Tên bài lấy từ tên tệp (“01 - Nghệ sĩ - Tên bài.mp3”). Riêng cách tải
          .zip còn đọc được thẻ nhạc trong tệp nên lấy đúng cả nghệ sĩ và album.
          Sửa lại sau ở từng bài.
        </p>

        <div className="mb-4 flex flex-wrap items-center gap-2">
          <TabBtn active={mode === "files"} onClick={() => setMode("files")}>
            Chọn nhiều tệp
          </TabBtn>
          <TabBtn active={mode === "zip"} onClick={() => setMode("zip")}>
            Tải tệp .zip
          </TabBtn>
        </div>

        {err && <Banner onDismiss={() => setErr(null)}>{err}</Banner>}

        <div className="min-h-0 flex-1 overflow-y-auto">
          {mode === "files" ? (
            <FilesPanel onError={setErr} onImported={onImported} onSwitchToZip={() => setMode("zip")} />
          ) : (
            <ZipPanel onError={setErr} onImported={onImported} />
          )}
        </div>

        <div className="mt-5 flex justify-end">
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg border px-4 py-2 text-sm font-medium transition hover:bg-[var(--tpl-surface-2)]"
            style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
          >
            Đóng
          </button>
        </div>
      </div>
    </div>
  );
}

/* ── multi-file ───────────────────────────────────────────────────── */

function FilesPanel({
  onError,
  onImported,
  onSwitchToZip,
}: {
  onError: (m: string | null) => void;
  onImported: () => void;
  onSwitchToZip: () => void;
}) {
  const [files, setFiles] = useState<FileState[]>([]);
  const [running, setRunning] = useState(false);
  const inputRef = useRef<HTMLInputElement | null>(null);

  const done = files.filter((f) => f.status === "done").length;
  const failed = files.filter((f) => f.status === "failed").length;

  function pick(list: FileList | null) {
    if (!list) return;
    onError(null);
    setFiles(
      Array.from(list)
        .filter((f) => f.type.startsWith("audio/") || /\.(mp3|m4a|aac|flac|ogg|oga|opus|wav|wma)$/i.test(f.name))
        .map((file) => ({ file, status: "waiting" as const, pct: 0 })),
    );
  }

  /**
   * Sequential, not parallel. Each file is an upload plus a create, and firing
   * fifty of those at once buys nothing on one connection while making the
   * per-file progress meaningless and the failures hard to attribute.
   */
  async function run() {
    setRunning(true);
    onError(null);
    let created = 0;

    for (let i = 0; i < files.length; i++) {
      const current = files[i];
      if (!current || current.status === "done") continue;

      const patch = (changes: Partial<FileState>) =>
        setFiles((prev) => prev.map((f, j) => (j === i ? { ...f, ...changes } : f)));

      patch({ status: "uploading", pct: 0, error: undefined });
      try {
        const assetId = await uploadAudioAsset(current.file, (pct) => patch({ pct }));
        // Same filename rules the zip importer falls back to, so a file does not
        // get one name here and a different one through the archive. Embedded
        // tags are still zip-only — the browser has no ffprobe.
        const { title, artist } = metaFromFilename(current.file.name);
        await createTrack({ title, artist: artist ?? null, audio_asset_id: assetId });
        patch({ status: "done", pct: 100, title });
        created++;
      } catch (e) {
        // One bad file must not abandon the rest — the whole point of a bulk
        // action is that it keeps going and tells you what it skipped.
        patch({ status: "failed", error: message(e, "Không nhập được tệp này.") });
      }
    }

    setRunning(false);
    if (created > 0) onImported();
  }

  return (
    <>
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <button
          type="button"
          onClick={() => inputRef.current?.click()}
          disabled={running}
          className="rounded-lg border px-3 py-2 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)] disabled:opacity-50"
          style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-accent)" }}
        >
          Chọn tệp…
        </button>
        <input
          ref={inputRef}
          type="file"
          accept="audio/*,.mp3,.m4a,.aac,.flac,.ogg,.oga,.opus,.wav,.wma"
          multiple
          className="hidden"
          onChange={(e) => {
            pick(e.target.files);
            e.target.value = ""; // allow re-picking the same selection
          }}
        />
        {files.length > 0 && (
          <button
            type="button"
            onClick={run}
            disabled={running}
            className="rounded-lg px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-50"
            style={{ background: "var(--tpl-accent)" }}
          >
            {running ? `Đang nhập… (${done}/${files.length})` : `Nhập ${files.length} bài`}
          </button>
        )}
      </div>

      {files.length > BULK_HINT_THRESHOLD && (
        <p
          className="mb-3 rounded-lg border px-3 py-2 text-xs"
          style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
        >
          {files.length} tệp là khá nhiều cho cách này — mỗi tệp là hai lượt gọi và
          phải giữ tab mở tới khi xong.{" "}
          <button
            type="button"
            onClick={onSwitchToZip}
            className="font-semibold underline"
            style={{ color: "var(--tpl-accent)" }}
          >
            Nén thành .zip
          </button>{" "}
          rồi tải một lần sẽ nhanh và an toàn hơn.
        </p>
      )}

      {files.length === 0 ? (
        <Empty>Chưa chọn tệp nào. Có thể chọn nhiều tệp cùng lúc.</Empty>
      ) : (
        <ul className="space-y-1.5">
          {files.map((f) => (
            <li
              key={f.file.name + f.file.size}
              className="flex items-center gap-3 rounded-lg border px-3 py-2 text-sm"
              style={{ borderColor: "var(--tpl-border)" }}
            >
              <span className="min-w-0 flex-1 truncate" style={{ color: "var(--tpl-text)" }}>
                {f.file.name}
              </span>
              <FileStatus state={f} />
            </li>
          ))}
        </ul>
      )}

      {(done > 0 || failed > 0) && !running && (
        <p className="mt-3 text-xs" style={{ color: "var(--tpl-muted)" }}>
          Xong: {done} thành công{failed > 0 ? `, ${failed} lỗi` : ""}.
        </p>
      )}
    </>
  );
}

function FileStatus({ state }: { state: FileState }) {
  switch (state.status) {
    case "waiting":
      return <Muted>chờ</Muted>;
    case "uploading":
      return <Muted>{state.pct}%</Muted>;
    case "done":
      return (
        <span className="shrink-0 text-xs font-semibold" style={{ color: "#16a34a" }}>
          đã nhập
        </span>
      );
    case "failed":
      return (
        <span
          className="shrink-0 max-w-[45%] truncate text-xs font-semibold"
          style={{ color: "#ef4444" }}
          title={state.error}
        >
          {state.error}
        </span>
      );
  }
}

/* ── zip ──────────────────────────────────────────────────────────── */

function ZipPanel({
  onError,
  onImported,
}: {
  onError: (m: string | null) => void;
  onImported: () => void;
}) {
  const [pct, setPct] = useState<number | null>(null);
  const [job, setJob] = useState<MusicImport | null>(null);
  // Enrichment is fire-and-forget: the tracks already exist and are playable, so
  // this only reports that the second pass was asked for. Its results arrive by
  // covers appearing on the tracks, not through this dialog.
  const [enriching, setEnriching] = useState<"idle" | "queued" | "working">("idle");
  const [looking, setLooking] = useState<"idle" | "queued" | "working">("idle");
  const [lookupNote, setLookupNote] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);

  // Poll while the worker is unpacking. Cleared on unmount so closing the dialog
  // mid-import stops the requests — the job keeps running server-side either way,
  // which is the reason the zip path exists.
  useEffect(() => {
    if (!job || !importInFlight(job)) return;
    let alive = true;
    const timer = setInterval(async () => {
      try {
        const next = await getImport(job.id);
        if (!alive) return;
        setJob(next);
        if (next.succeeded > 0 && !importInFlight(next)) onImported();
      } catch {
        // A blip should not kill the poll loop; the next tick retries.
      }
    }, POLL_MS);
    return () => {
      alive = false;
      clearInterval(timer);
    };
  }, [job, onImported]);

  async function enrich(id: string) {
    setEnriching("working");
    try {
      await enrichImport(id);
      setEnriching("queued");
    } catch (e) {
      setEnriching("idle");
      onError(message(e, "Không gửi được yêu cầu lấy ảnh bìa."));
    }
  }

  async function lookup(id: string) {
    setLooking("working");
    setLookupNote(null);
    try {
      const { queued, skipped } = await lookupImport(id);
      setLooking("queued");
      setLookupNote(
        skipped > 0
          ? `Đã xếp hàng ${queued} bài, bỏ qua ${skipped} bài vượt giới hạn mỗi lượt.`
          : `Đã xếp hàng ${queued} bài.`,
      );
    } catch (e) {
      setLooking("idle");
      onError(message(e, "Không gửi được yêu cầu tra cứu."));
    }
  }

  async function upload(file: File) {
    onError(null);
    setJob(null);
    setEnriching("idle");
    setLooking("idle");
    setLookupNote(null);
    setPct(0);
    try {
      setJob(await importZip(file, setPct));
    } catch (e) {
      onError(message(e, "Không tải được tệp zip."));
    } finally {
      setPct(null);
    }
  }

  return (
    <>
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <button
          type="button"
          onClick={() => inputRef.current?.click()}
          disabled={pct !== null}
          className="rounded-lg px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-50"
          style={{ background: "var(--tpl-accent)" }}
        >
          {pct !== null ? `Đang tải lên… ${pct}%` : "Chọn tệp .zip"}
        </button>
        <input
          ref={inputRef}
          type="file"
          accept=".zip,application/zip"
          className="hidden"
          onChange={(e) => {
            const f = e.target.files?.[0];
            e.target.value = "";
            if (f) void upload(f);
          }}
        />
      </div>

      {!job && pct === null && (
        <Empty>
          Nén cả thư mục nhạc thành một tệp .zip rồi tải lên. Tối đa 4 GB, 2000
          bài. Ảnh bìa và tệp lạ trong zip được bỏ qua, không tính là lỗi.
        </Empty>
      )}

      {job && (
        <ImportProgress
          job={job}
          enriching={enriching}
          onEnrich={() => void enrich(job.id)}
          looking={looking}
          lookupNote={lookupNote}
          onLookup={() => void lookup(job.id)}
        />
      )}
    </>
  );
}

function ImportProgress({
  job,
  enriching,
  onEnrich,
  looking,
  lookupNote,
  onLookup,
}: {
  job: MusicImport;
  enriching: "idle" | "queued" | "working";
  onEnrich: () => void;
  looking: "idle" | "queued" | "working";
  lookupNote: string | null;
  onLookup: () => void;
}) {
  const pct = job.total > 0 ? Math.round(((job.succeeded + job.failed) / job.total) * 100) : 0;
  const finished = !importInFlight(job) && job.succeeded > 0;

  return (
    <div>
      <div
        className="mb-3 rounded-lg border px-3 py-2.5"
        style={{ borderColor: "var(--tpl-border)" }}
      >
        <div className="flex items-center justify-between text-sm">
          <span style={{ color: "var(--tpl-heading)" }}>{statusLabel(job)}</span>
          {job.total > 0 && (
            <span className="tabular-nums text-xs" style={{ color: "var(--tpl-muted)" }}>
              {job.succeeded + job.failed}/{job.total}
            </span>
          )}
        </div>
        {job.total > 0 && (
          <div
            className="mt-2 h-1.5 overflow-hidden rounded-full"
            style={{ background: "var(--tpl-surface-2)" }}
          >
            <div
              className="h-full rounded-full transition-[width]"
              style={{ width: `${pct}%`, background: "var(--tpl-accent)" }}
            />
          </div>
        )}
        {job.error && (
          <p className="mt-2 text-xs" style={{ color: "#ef4444" }}>
            {job.error}
          </p>
        )}
        {!importInFlight(job) && !job.error && (
          <p className="mt-2 text-xs" style={{ color: "var(--tpl-muted)" }}>
            {job.succeeded} bài đã nhập{job.failed > 0 ? `, ${job.failed} tệp lỗi` : ""}. Có thể
            đóng cửa sổ này — nhạc đã nằm trong thư viện.
          </p>
        )}

        {/* The second pass is offered, not automatic: it is slower than the
            import that just finished and the tracks are already usable without
            it, so it should be the user's call rather than a wait they did not
            ask for. */}
        {finished && (
          <div className="mt-3">
            {enriching === "queued" ? (
              <p className="text-xs" style={{ color: "var(--tpl-muted)" }}>
                Đã gửi yêu cầu — ảnh bìa và thông tin còn thiếu sẽ hiện dần trong
                thư viện, không cần mở cửa sổ này.
              </p>
            ) : (
              <>
                <button
                  type="button"
                  onClick={onEnrich}
                  disabled={enriching === "working"}
                  className="rounded-lg border px-3 py-1.5 text-xs font-semibold transition hover:bg-[var(--tpl-surface-2)] disabled:opacity-50"
                  style={{ borderColor: "var(--tpl-accent)", color: "var(--tpl-accent)" }}
                >
                  {enriching === "working" ? "Đang gửi…" : "Lấy ảnh bìa & thông tin còn thiếu"}
                </button>
                <p className="mt-1.5 text-xs" style={{ color: "var(--tpl-muted)" }}>
                  Đọc ảnh bìa nhúng trong từng tệp nhạc và điền nghệ sĩ / album
                  còn trống. Chạy nền, chậm hơn bước nhập nên tách riêng; những gì
                  bạn đã tự sửa sẽ không bị ghi đè.
                </p>
              </>
            )}
          </div>
        )}

        {/* The catalogue lookup is a separate offer from the local enrichment,
            not a bigger version of it: it leaves the machine. Presenting them as
            one button would hide that a third party is being told what is in the
            library. */}
        {finished && (
          <div className="mt-3 border-t pt-3" style={{ borderColor: "var(--tpl-border)" }}>
            {looking === "queued" ? (
              <p className="text-xs" style={{ color: "var(--tpl-muted)" }}>
                {lookupNote} Kết quả hiện dần — tra cứu bị giới hạn 1 yêu cầu/giây
                nên vài trăm bài sẽ mất ít phút.
              </p>
            ) : (
              <>
                <button
                  type="button"
                  onClick={onLookup}
                  disabled={looking === "working"}
                  className="rounded-lg border px-3 py-1.5 text-xs font-semibold transition hover:bg-[var(--tpl-surface-2)] disabled:opacity-50"
                  style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
                >
                  {looking === "working" ? "Đang gửi…" : "Tra cứu MusicBrainz (năm, thể loại, bìa album)"}
                </button>
                <p className="mt-1.5 text-xs" style={{ color: "var(--tpl-muted)" }}>
                  ⚠️ Bước này <b>gửi tên bài và nghệ sĩ ra dịch vụ ngoài</b>
                  (MusicBrainz + Cover Art Archive) để lấy năm phát hành, thể loại
                  và ảnh bìa chất lượng cao — những thứ không có trong tệp nhạc.
                  Chỉ khớp khi đủ chắc chắn; không chắc thì để trống chứ không đoán.
                </p>
              </>
            )}
          </div>
        )}
      </div>

      {job.report.length > 0 && (
        <ul className="space-y-1">
          {job.report.map((entry) => (
            <li
              key={entry.name}
              className="flex items-center gap-3 rounded-lg border px-3 py-1.5 text-xs"
              style={{ borderColor: "var(--tpl-border)" }}
            >
              <span className="min-w-0 flex-1 truncate" style={{ color: "var(--tpl-text)" }}>
                {entry.title || entry.name}
              </span>
              {entry.ok ? (
                <span className="shrink-0 font-semibold" style={{ color: "#16a34a" }}>
                  ✓
                </span>
              ) : (
                <span
                  className="shrink-0 max-w-[50%] truncate font-semibold"
                  style={{ color: "#ef4444" }}
                  title={entry.error}
                >
                  {entry.error}
                </span>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function statusLabel(job: MusicImport): string {
  switch (job.status) {
    case "pending":
    case "uploaded":
      return "Đang chờ xử lý…";
    case "processing":
      return "Đang giải nén và nhập…";
    case "done":
      return "Đã nhập xong";
    case "failed":
      return "Nhập thất bại";
  }
}

/* ── pieces ───────────────────────────────────────────────────────── */

function TabBtn({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="rounded-lg px-3 py-1.5 text-sm font-semibold transition"
      style={{
        background: active ? "var(--tpl-accent)" : "transparent",
        color: active ? "#fff" : "var(--tpl-muted)",
        border: `1px solid ${active ? "var(--tpl-accent)" : "var(--tpl-border)"}`,
      }}
    >
      {children}
    </button>
  );
}

function Muted({ children }: { children: ReactNode }) {
  return (
    <span className="shrink-0 text-xs tabular-nums" style={{ color: "var(--tpl-muted)" }}>
      {children}
    </span>
  );
}

function Empty({ children }: { children: ReactNode }) {
  return (
    <p
      className="rounded-xl border border-dashed px-4 py-8 text-center text-xs"
      style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
    >
      {children}
    </p>
  );
}

function Banner({ children, onDismiss }: { children: ReactNode; onDismiss?: () => void }) {
  return (
    <p
      className="mb-3 flex items-center justify-between gap-3 rounded-lg border px-3 py-2 text-sm"
      style={{
        borderColor: "rgba(239,68,68,.4)",
        background: "rgba(239,68,68,.08)",
        color: "#ef4444",
      }}
    >
      <span>{children}</span>
      {onDismiss && (
        <button type="button" onClick={onDismiss} aria-label="Đóng">
          <Icon name="close-icon" size={10} />
        </button>
      )}
    </p>
  );
}

function message(e: unknown, fallback: string): string {
  if (e instanceof ApiError) return problemDisplayMessage(e.body);
  return e instanceof Error && e.message ? e.message : fallback;
}
