import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";

/**
 * Birthday rail card — extracted from the inline `BirthdayCard` in
 * `views/home/HomeView.tsx` (Olympus birthday widget). A purple gradient card
 * with decorative blobs, a ringed avatar and a "Today is X's Birthday!"
 * headline. `name` is optional and defaults to the Olympus sample person.
 */
export function BirthdayCard({
  name = "Marina Valentine",
  message = "Leave her a message with your best wishes on her profile page!",
}: {
  name?: string;
  message?: string;
} = {}) {
  return (
    <div
      className="relative overflow-hidden rounded-xl p-5 text-white shadow-sm"
      style={{ background: "linear-gradient(150deg, #8a63d2, #6d4bb8)" }}
    >
      {/* decorative blobs (≈ Olympus birthday widget) */}
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 opacity-20"
        style={{
          background:
            "radial-gradient(60% 60% at 85% 15%, rgba(255,255,255,0.5), transparent 60%)," +
            "radial-gradient(50% 50% at 20% 90%, rgba(255,255,255,0.25), transparent 60%)",
        }}
      />
      <div className="relative">
        <div className="flex items-start justify-between">
          <Icon name="cupcake-icon" size={22} />
          <button type="button" aria-label="Options">
            <Icon name="three-dots-icon" size={18} />
          </button>
        </div>

        <div className="mt-4">
          <Avatar name={name} size={44} className="ring-2 ring-white/40" />
        </div>

        <p className="mt-3 text-sm text-white/80">Today is</p>
        <h3 className="text-2xl font-bold leading-tight">{name}&apos;s Birthday!</h3>
        <p className="mt-3 text-sm leading-relaxed text-white/80">{message}</p>
      </div>
    </div>
  );
}
