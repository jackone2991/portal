"use client";

import { useState, type FormEvent, type ReactNode } from "react";

type Tab = "login" | "register";

const API_BASE =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "https://api.portal.localhost";

/**
 * Login / register card — React + Tailwind port of the Olympus "Landing Page"
 * form. Portal owns credentials (ADR-06 local auth): both actions POST straight
 * to the API and, on success, the browser carries the `portal_access` cookie to
 * the app. No IdP redirect, no SSO round-trip.
 */
export function AuthForm({ defaultTab = "login" }: { defaultTab?: Tab }) {
  const [tab, setTab] = useState<Tab>(defaultTab);
  const [notice, setNotice] = useState<string | null>(null);
  const [prefillEmail, setPrefillEmail] = useState("");

  // After a successful registration we do NOT log the user in — we bounce them
  // to the login tab to sign in with the account they just created.
  function handleRegistered(email: string) {
    setPrefillEmail(email);
    setNotice("Account created. Please sign in with your new account.");
    setTab("login");
  }

  function switchTo(next: Tab) {
    setNotice(null);
    setTab(next);
  }

  return (
    <div
      className="w-full max-w-md rounded-2xl border p-6 shadow-2xl backdrop-blur sm:p-8"
      style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
    >
      {/* Nav tabs */}
      <div
        className="mb-6 grid grid-cols-2 overflow-hidden rounded-xl border"
        style={{ borderColor: "var(--tpl-border)" }}
      >
        <TabButton active={tab === "login"} onClick={() => switchTo("login")}>
          <LogInIcon /> Sign in
        </TabButton>
        <TabButton active={tab === "register"} onClick={() => switchTo("register")}>
          <UserPlusIcon /> Register
        </TabButton>
      </div>

      {tab === "login" ? (
        <LoginForm
          onSwitch={() => switchTo("register")}
          initialEmail={prefillEmail}
          notice={notice}
        />
      ) : (
        <RegisterForm onSwitch={() => switchTo("login")} onRegistered={handleRegistered} />
      )}
    </div>
  );
}

/* ── forms ───────────────────────────────────────────────────────── */

function nextTarget(): string {
  if (typeof window === "undefined") return "/";
  const n = new URLSearchParams(window.location.search).get("next");
  // only allow same-origin relative paths
  return n && n.startsWith("/") ? n : "/";
}

async function postAuth(path: string, payload: Record<string, unknown>) {
  const res = await fetch(`${API_BASE}/api/v1/auth/${path}`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (res.ok) return null;
  let msg = "Something went wrong. Please try again.";
  try {
    const body = (await res.json()) as { message?: string };
    if (body.message) msg = body.message;
  } catch {
    /* non-JSON error body */
  }
  return msg;
}

function LoginForm({
  onSwitch,
  initialEmail = "",
  notice = null,
}: {
  onSwitch: () => void;
  initialEmail?: string;
  notice?: string | null;
}) {
  const [email, setEmail] = useState(initialEmail);
  const [password, setPassword] = useState("");
  const [remember, setRemember] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    const err = await postAuth("login", { email, password, remember });
    if (err) {
      setError(err);
      setLoading(false);
      return;
    }
    // Mark the just-issued token as fresh so SessionKeeper doesn't immediately
    // re-refresh (and needlessly rotate) right after login.
    try {
      localStorage.setItem("portal_refresh_at", String(Date.now()));
    } catch {
      /* storage disabled */
    }
    window.location.assign(nextTarget());
  }

  return (
    <form className="space-y-4" onSubmit={onSubmit}>
      <h2 className="text-lg font-semibold">Login to your account</h2>
      {notice && <SuccessBanner>{notice}</SuccessBanner>}
      {error && <ErrorBanner>{error}</ErrorBanner>}
      <Field
        label="Your Email"
        name="email"
        type="email"
        autoComplete="email"
        placeholder="you@example.com"
        value={email}
        onChange={setEmail}
        required
      />
      <Field
        label="Your Password"
        name="password"
        type="password"
        autoComplete="current-password"
        placeholder="••••••••"
        value={password}
        onChange={setPassword}
        required
      />

      <div className="flex items-center justify-between text-sm">
        <label
          className="flex cursor-pointer items-center gap-2"
          style={{ color: "var(--tpl-muted)" }}
        >
          <input
            type="checkbox"
            className="accent-[var(--tpl-accent)]"
            checked={remember}
            onChange={(e) => setRemember(e.target.checked)}
          />{" "}
          Remember me
        </label>
        <span style={{ color: "var(--tpl-muted)" }}>Forgot password? (coming soon)</span>
      </div>

      <PrimaryButton loading={loading}>Login</PrimaryButton>

      <p className="text-center text-sm" style={{ color: "var(--tpl-muted)" }}>
        Don&apos;t have an account?{" "}
        <button
          type="button"
          onClick={onSwitch}
          className="font-medium hover:underline"
          style={{ color: "var(--tpl-accent)" }}
        >
          Register now
        </button>
      </p>
    </form>
  );
}

function RegisterForm({
  onSwitch,
  onRegistered,
}: {
  onSwitch: () => void;
  onRegistered: (email: string) => void;
}) {
  const [displayName, setDisplayName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [accepted, setAccepted] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    if (!accepted) {
      setError("Please accept the Terms & Conditions to continue.");
      return;
    }
    if (password.length < 8) {
      setError("Password must be at least 8 characters.");
      return;
    }
    setLoading(true);
    const err = await postAuth("register", {
      email,
      password,
      display_name: displayName,
    });
    if (err) {
      setError(err);
      setLoading(false);
      return;
    }
    // Success: no session was created — hand the email back to the parent, which
    // switches to the login tab so the user signs in with the new account.
    onRegistered(email);
  }

  return (
    <form className="space-y-4" onSubmit={onSubmit}>
      <h2 className="text-lg font-semibold">Create your account</h2>
      {error && <ErrorBanner>{error}</ErrorBanner>}
      <Field
        label="Display name"
        name="display_name"
        autoComplete="name"
        placeholder="Your name"
        value={displayName}
        onChange={setDisplayName}
      />
      <Field
        label="Your Email"
        name="email"
        type="email"
        autoComplete="email"
        placeholder="you@example.com"
        value={email}
        onChange={setEmail}
        required
      />
      <Field
        label="Your Password"
        name="password"
        type="password"
        autoComplete="new-password"
        placeholder="At least 8 characters"
        value={password}
        onChange={setPassword}
        required
      />

      <label
        className="flex cursor-pointer items-start gap-2 text-sm"
        style={{ color: "var(--tpl-muted)" }}
      >
        <input
          type="checkbox"
          className="mt-0.5 accent-[var(--tpl-accent)]"
          checked={accepted}
          onChange={(e) => setAccepted(e.target.checked)}
        />
        <span>
          I accept the{" "}
          <a href="#" className="hover:underline" style={{ color: "var(--tpl-accent)" }}>
            Terms &amp; Conditions
          </a>{" "}
          of the website
        </span>
      </label>

      <PrimaryButton loading={loading}>Complete registration</PrimaryButton>

      <p className="text-center text-sm" style={{ color: "var(--tpl-muted)" }}>
        Already have an account?{" "}
        <button
          type="button"
          onClick={onSwitch}
          className="font-medium hover:underline"
          style={{ color: "var(--tpl-accent)" }}
        >
          Sign in
        </button>
      </p>
    </form>
  );
}

/* ── pieces ──────────────────────────────────────────────────────── */

function TabButton({
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
      className="flex items-center justify-center gap-2 px-4 py-2.5 text-sm font-medium transition"
      style={
        active
          ? { background: "var(--tpl-accent)", color: "var(--tpl-accent-contrast)" }
          : { color: "var(--tpl-muted)" }
      }
    >
      {children}
    </button>
  );
}

function Field({
  label,
  name,
  type = "text",
  placeholder,
  value,
  onChange,
  autoComplete,
  required,
}: {
  label: string;
  name: string;
  type?: string;
  placeholder?: string;
  value: string;
  onChange: (v: string) => void;
  autoComplete?: string;
  required?: boolean;
}) {
  return (
    <label className="block">
      <span
        className="mb-1.5 block text-xs font-medium uppercase tracking-wide"
        style={{ color: "var(--tpl-muted)" }}
      >
        {label}
      </span>
      <input
        name={name}
        type={type}
        placeholder={placeholder}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        autoComplete={autoComplete}
        required={required}
        className="w-full rounded-lg border bg-transparent px-3.5 py-2.5 text-sm outline-none transition focus:border-[var(--tpl-accent)]"
        style={{ borderColor: "var(--tpl-border)" }}
      />
    </label>
  );
}

function PrimaryButton({ children, loading }: { children: ReactNode; loading?: boolean }) {
  return (
    <button
      type="submit"
      disabled={loading}
      className="w-full rounded-lg px-4 py-2.5 text-sm font-semibold transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
      style={{ background: "var(--tpl-accent)", color: "var(--tpl-accent-contrast)" }}
    >
      {loading ? "Please wait…" : children}
    </button>
  );
}

function ErrorBanner({ children }: { children: ReactNode }) {
  return (
    <div
      role="alert"
      className="rounded-lg border px-3.5 py-2.5 text-sm"
      style={{
        borderColor: "rgba(239,68,68,0.4)",
        background: "rgba(239,68,68,0.08)",
        color: "#ef4444",
      }}
    >
      {children}
    </div>
  );
}

function SuccessBanner({ children }: { children: ReactNode }) {
  return (
    <div
      role="status"
      className="rounded-lg border px-3.5 py-2.5 text-sm"
      style={{
        borderColor: "rgba(34,197,94,0.4)",
        background: "rgba(34,197,94,0.08)",
        color: "#22c55e",
      }}
    >
      {children}
    </div>
  );
}

/* ── icons (inline, no external lib) ─────────────────────────────── */

function iconProps() {
  return {
    width: 16,
    height: 16,
    viewBox: "0 0 24 24",
    fill: "none",
    stroke: "currentColor",
    strokeWidth: 2,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
    "aria-hidden": true,
  };
}

function LogInIcon() {
  return (
    <svg {...iconProps()}>
      <path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4" />
      <polyline points="10 17 15 12 10 7" />
      <line x1="15" y1="12" x2="3" y2="12" />
    </svg>
  );
}

function UserPlusIcon() {
  return (
    <svg {...iconProps()}>
      <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" />
      <circle cx="9" cy="7" r="4" />
      <line x1="19" y1="8" x2="19" y2="14" />
      <line x1="22" y1="11" x2="16" y2="11" />
    </svg>
  );
}
