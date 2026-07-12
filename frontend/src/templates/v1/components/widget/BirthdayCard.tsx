"use client";

import { useQuery } from "@tanstack/react-query";
import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";
import { upcomingBirthdays } from "@/lib/people";

/**
 * Birthday rail card (SPEC-08 P0.5) — wired to /people/upcoming-birthdays. Shows
 * the nearest upcoming birthday with real state; renders nothing when there are
 * none (the rail slot degrades to empty, SPEC-06 pattern). Olympus purple design.
 */
export function BirthdayCard() {
  const { data = [], isLoading } = useQuery({ queryKey: ["people", "upcoming"], queryFn: () => upcomingBirthdays(14) });
  const next = data[0];
  if (isLoading || !next) return null;

  const when = next.days_until === 0 ? "Today is" : next.days_until === 1 ? "Tomorrow is" : `In ${next.days_until} days —`;
  const age = next.age_turning != null ? ` (turning ${next.age_turning})` : "";

  return (
    <div className="relative overflow-hidden rounded-xl p-5 text-white shadow-sm" style={{ background: "linear-gradient(150deg, #8a63d2, #6d4bb8)" }}>
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
          <Avatar name={next.display_name} size={44} className="ring-2 ring-white/40" />
        </div>
        <p className="mt-3 text-sm text-white/80">{when}</p>
        <h3 className="text-2xl font-bold leading-tight">{next.display_name}&apos;s Birthday!{age}</h3>
        <p className="mt-3 text-sm leading-relaxed text-white/80">
          {next.days_until <= 1 ? "Reach out and make their day." : "A heads-up so you don't forget."}
        </p>
      </div>
    </div>
  );
}
