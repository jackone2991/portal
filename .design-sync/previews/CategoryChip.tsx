import { CategoryChip } from "portal-frontend";

// The ledger's category glyph (migration 0042). `icon` is an EMOJI, not a sprite
// id — the Olympus sprite is a social-network set with no money/food/transport
// glyphs. Both icon and colour are nullable and the chip falls back to a letter
// on a neutral tone, so "no icon" is an ordinary state, not a defect.

const frame = (label: string, children: React.ReactNode) => (
  <div data-template="v1" className="rounded-xl p-4" style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)" }}>
    <p className="mb-3 text-xs font-bold uppercase tracking-wide" style={{ color: "var(--tpl-muted)" }}>{label}</p>
    <div className="flex flex-wrap items-center gap-4">{children}</div>
  </div>
);

const cell = (node: React.ReactNode, caption: string) => (
  <div className="flex w-20 flex-col items-center gap-1.5">
    {node}
    <span className="text-center text-[11px] leading-tight" style={{ color: "var(--tpl-muted)" }}>{caption}</span>
  </div>
);

export const WithEmoji = () =>
  frame("Có emoji + màu", (
    <>
      {cell(<CategoryChip icon="🍜" color="#f97316" name="Ăn uống" />, "Ăn uống")}
      {cell(<CategoryChip icon="🏍️" color="#0ea5e9" name="Đi lại" />, "Đi lại")}
      {cell(<CategoryChip icon="🏠" color="#8b5cf6" name="Nhà cửa" />, "Nhà cửa")}
      {cell(<CategoryChip icon="💊" color="#ef4444" name="Sức khoẻ" />, "Sức khoẻ")}
      {cell(<CategoryChip icon="💰" color="#22c55e" name="Lương" />, "Lương")}
    </>
  ));

export const LetterFallback = () =>
  frame("Không icon → chip chữ cái", (
    <>
      {cell(<CategoryChip name="Giáo dục" color="#6366f1" />, "có màu")}
      {cell(<CategoryChip name="Quà tặng" />, "không màu")}
      {cell(<CategoryChip name="Chưa phân loại" />, "mặc định")}
    </>
  ));

export const Sizes = () =>
  frame("Kích thước", (
    <>
      {cell(<CategoryChip icon="☕" color="#a16207" name="Cà phê" size={28} />, "28")}
      {cell(<CategoryChip icon="☕" color="#a16207" name="Cà phê" size={40} />, "40 (mặc định)")}
      {cell(<CategoryChip icon="☕" color="#a16207" name="Cà phê" size={56} />, "56")}
    </>
  ));
