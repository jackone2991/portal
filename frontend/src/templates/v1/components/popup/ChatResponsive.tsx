"use client";

import { useState, type FormEvent } from "react";
import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";

type Msg = { id: string; from: "them" | "me"; text: string; time: string };

const SEED: Msg[] = [
  { id: "m1", from: "them", text: "Hey! Are we still on for the show this weekend?", time: "Yesterday at 8:10pm" },
  { id: "m2", from: "me", text: "Absolutely — got the tickets this morning 🎸", time: "Yesterday at 8:12pm" },
  { id: "m3", from: "them", text: "Amazing. Iron Maid is going to be incredible live.", time: "Yesterday at 8:13pm" },
  { id: "m4", from: "me", text: "Can’t wait. I’ll pick you up at 7.", time: "Yesterday at 8:15pm" },
];

/**
 * Responsive chat panel — port of Olympus `.popup-chat-responsive`
 * (`Fav Page - Settings And Create Popup.html`). A floating chat window with a
 * presence header, a scrolling message list (incoming / outgoing bubbles), and
 * a composer. Controlled via `open` / `onClose`; docks bottom-right.
 */
export function ChatResponsive({
  open = false,
  onClose = () => {},
  contact = "Marina Valentine",
  online = true,
}: {
  open?: boolean;
  onClose?: () => void;
  contact?: string;
  online?: boolean;
}) {
  const [messages, setMessages] = useState<Msg[]>(SEED);
  const [draft, setDraft] = useState("");

  function send(e: FormEvent) {
    e.preventDefault();
    const t = draft.trim();
    if (!t) return;
    setMessages((prev) => [...prev, { id: `me-${prev.length}`, from: "me", text: t, time: "Just now" }]);
    setDraft("");
  }

  if (!open) return null;

  return (
    <div
      className="fixed bottom-0 right-4 z-[55] flex h-[26rem] w-80 flex-col overflow-hidden rounded-t-xl shadow-2xl"
      style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)" }}
      role="dialog"
      aria-label={`Chat with ${contact}`}
    >
      {/* header */}
      <div className="flex items-center gap-2.5 px-4 py-3" style={{ background: "var(--tpl-header)" }}>
        <Avatar name={contact} size={32} status={online ? "online" : "offline"} />
        <div className="min-w-0 flex-1 leading-tight">
          <p className="truncate text-sm font-semibold text-white">{contact}</p>
          <p className="text-[11px] text-white/60">{online ? "Online" : "Offline"}</p>
        </div>
        <button type="button" className="text-white/60 transition hover:text-white" aria-label="Options">
          <Icon name="three-dots-icon" size={16} />
        </button>
        <button type="button" onClick={onClose} className="text-white/60 transition hover:text-white" aria-label="Close chat">
          <Icon name="little-delete" size={14} />
        </button>
      </div>

      {/* messages */}
      <div className="flex-1 space-y-3 overflow-y-auto px-3 py-4" style={{ background: "var(--tpl-surface-2)" }}>
        {messages.map((m) => (
          <div key={m.id} className={`flex items-end gap-2 ${m.from === "me" ? "flex-row-reverse" : ""}`}>
            <Avatar name={m.from === "me" ? "You" : contact} size={26} />
            <div className={`max-w-[70%] ${m.from === "me" ? "text-right" : ""}`}>
              <div
                className="inline-block rounded-2xl px-3 py-2 text-left text-sm"
                style={
                  m.from === "me"
                    ? { background: "var(--tpl-accent)", color: "#fff" }
                    : { background: "var(--tpl-surface)", color: "var(--tpl-text)", border: "1px solid var(--tpl-border)" }
                }
              >
                {m.text}
              </div>
              <div className="mt-0.5 text-[10px]" style={{ color: "var(--tpl-muted)" }}>
                {m.time}
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* composer */}
      <form onSubmit={send} className="flex items-center gap-2 border-t px-3 py-2.5" style={{ borderColor: "var(--tpl-border)" }}>
        <button type="button" className="text-[var(--tpl-muted)] transition hover:text-[var(--tpl-accent)]" aria-label="Attach">
          <Icon name="computer-icon" size={18} />
        </button>
        <button type="button" className="text-[var(--tpl-muted)] transition hover:text-[var(--tpl-accent)]" aria-label="Emoji">
          <Icon name="happy-sticker-icon" size={18} />
        </button>
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          placeholder="Press enter to post..."
          className="min-w-0 flex-1 bg-transparent text-sm outline-none placeholder:text-[var(--tpl-muted)]"
          style={{ color: "var(--tpl-text)" }}
        />
      </form>
    </div>
  );
}
