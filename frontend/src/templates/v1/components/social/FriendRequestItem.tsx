import { Avatar } from "../ui/Avatar";
import { ControlBlockButtons } from "../profile/ControlBlockButtons";

type Status = "online" | "away" | "work" | "offline" | "invisible";

/**
 * A friend-request row — React port of the Olympus `.friend-requests` list item
 * (Your Account - Friends Requests 1751-1780): Avatar (+presence), name, mutual
 * count, and accept / decline circular buttons (reusing ControlBlockButtons).
 */
export function FriendRequestItem({
  name,
  mutual,
  status,
  onAccept,
  onDecline,
}: {
  name: string;
  mutual?: number;
  status?: Status;
  onAccept?: () => void;
  onDecline?: () => void;
}) {
  return (
    <div
      className="flex items-center gap-3 rounded-xl px-4 py-3 shadow-sm"
      style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)" }}
    >
      <Avatar name={name} size={48} status={status} className="shrink-0" />
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>
          {name}
        </p>
        {mutual != null && (
          <p className="truncate text-xs" style={{ color: "var(--tpl-muted)" }}>
            {mutual} mutual friend{mutual === 1 ? "" : "s"}
          </p>
        )}
      </div>
      <ControlBlockButtons
        size={36}
        buttons={[
          { icon: "check-icon", label: "Accept", color: "var(--tpl-accent)", onClick: onAccept },
          { icon: "close-icon", label: "Decline", color: "var(--tpl-muted)", onClick: onDecline },
        ]}
      />
    </div>
  );
}
