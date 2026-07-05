import Link from "next/link";
import { AuthForm } from "./AuthForm";

/**
 * Olympus "Landing Page" two-column auth layout (welcome + form card) —
 * port of template-main/social/social/Landing Page.html. Shared by /login and
 * /register; only the active tab and the left-column copy differ.
 */
export function AuthLanding({ defaultTab }: { defaultTab: "login" | "register" }) {
  const registering = defaultTab === "register";

  return (
    <div className="relative flex min-h-screen items-center overflow-hidden">
      {/* ambient background (≈ Olympus .content-bg-wrap) */}
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0"
        style={{
          background:
            "radial-gradient(55% 55% at 18% 22%, rgba(255,94,58,0.14), transparent 70%)," +
            "radial-gradient(45% 45% at 90% 85%, rgba(56,169,255,0.10), transparent 70%)",
        }}
      />

      <div className="relative mx-auto grid w-full max-w-6xl items-center gap-10 px-6 py-16 md:grid-cols-2">
        {/* Left — welcome */}
        <div className="landing-content">
          <Link
            href="/"
            className="inline-flex items-center gap-2 text-sm"
            style={{ color: "var(--tpl-muted)" }}
          >
            <span
              className="grid h-8 w-8 place-items-center rounded-lg text-sm font-bold"
              style={{ background: "var(--tpl-accent)", color: "var(--tpl-accent-contrast)" }}
            >
              S
            </span>
            Sky Feeling · Social Network
          </Link>

          <h1 className="mt-6 text-4xl font-bold leading-[1.1] tracking-tight sm:text-5xl">
            {registering
              ? "Join the biggest social network"
              : "Welcome to the biggest social network"}
          </h1>
          <p className="mt-5 max-w-md text-lg" style={{ color: "var(--tpl-muted)" }}>
            Share your thoughts, write posts, stream your favourite music, earn badges and much
            more.
          </p>

          <Link
            href={registering ? "/login" : "/register"}
            className="mt-8 inline-block rounded-lg border px-5 py-2.5 text-sm font-medium transition hover:bg-white/5"
            style={{ borderColor: "var(--tpl-border)" }}
          >
            {registering ? "I already have an account" : "Register now"}
          </Link>
        </div>

        {/* Right — form card */}
        <div className="flex justify-center md:justify-end">
          <AuthForm defaultTab={defaultTab} />
        </div>
      </div>
    </div>
  );
}
