"use client";

// Keyboard-shortcut help (SPEC-02 reader redesign, R5). Toggled with "?" or the
// help button; lists the reader's controls.

const ROWS: [string, string][] = [
  ["← / →", "Lật trang (theo hướng đọc)"],
  ["Space", "Trang sau"],
  ["↑ / ↓", "Cuộn (chế độ cuộn dọc)"],
  ["+ / − / 0", "Phóng to / thu nhỏ / đặt lại"],
  ["Chạm‑đúp · cuộn chuột", "Phóng to tại điểm"],
  ["[ / ]", "Chương trước / sau"],
  ["F", "Ẩn/hiện thanh điều khiển"],
  ["S", "Cài đặt"],
  ["C", "Danh sách chương"],
  ["?", "Trợ giúp này"],
];

export function ReaderHelp({ open, onClose }: { open: boolean; onClose: () => void }) {
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-40 flex items-center justify-center bg-black/70 p-4" onClick={onClose} role="dialog" aria-modal="true" aria-label="Phím tắt">
      <div className="w-full max-w-sm rounded-2xl border border-white/10 bg-neutral-900 p-5 text-white" onClick={(e) => e.stopPropagation()}>
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-xs font-bold uppercase tracking-wider text-gray-400">Phím tắt</h2>
          <button type="button" onClick={onClose} className="rounded-md bg-blue-600 px-3 py-1 text-sm font-medium hover:bg-blue-500">Đóng</button>
        </div>
        <dl className="space-y-2">
          {ROWS.map(([k, v]) => (
            <div key={k} className="flex items-center justify-between gap-4">
              <dt className="shrink-0 rounded-md border border-white/15 bg-white/5 px-2 py-1 font-mono text-xs">{k}</dt>
              <dd className="text-right text-sm text-gray-300">{v}</dd>
            </div>
          ))}
        </dl>
      </div>
    </div>
  );
}
