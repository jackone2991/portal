"use client";

import { useState, type ReactNode } from "react";

type Tab = "login" | "register";

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080";
const OIDC_LOGIN = `${API_BASE}/api/v1/auth/login`;
// Hint to pre-select the Google source in Authentik. The backend forwards `idp`
// to Authentik's authorize request; requires a Google source configured there.
const OIDC_GOOGLE = `${API_BASE}/api/v1/auth/login?idp=google`;

/**
 * Login / register card — React + Tailwind port of the Olympus "Landing Page"
 * `registration-login-form` (template-main/social/social/Landing Page.html).
 *
 * Portal auth is OIDC via Authentik — there is no local password. Both the
 * "Login" and "Register" actions redirect to `/api/v1/auth/login`; the fields
 * reproduce the template's visual design. The bordered SSO button is the
 * explicit, honest entry point.
 */
export function AuthForm({ defaultTab = "login" }: { defaultTab?: Tab }) {
  const [tab, setTab] = useState<Tab>(defaultTab);

  function goOidc() {
    window.location.href = OIDC_LOGIN;
  }

  function goGoogle() {
    window.location.href = OIDC_GOOGLE;
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
        <TabButton active={tab === "login"} onClick={() => setTab("login")}>
          <LogInIcon /> Sign in
        </TabButton>
        <TabButton active={tab === "register"} onClick={() => setTab("register")}>
          <UserPlusIcon /> Register
        </TabButton>
      </div>

      {tab === "login" ? (
        <form
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            goOidc();
          }}
        >
          <h2 className="text-lg font-semibold">Login to your account</h2>
          <Field label="Your Email" name="email" type="email" placeholder="you@example.com" />
          <Field label="Your Password" name="password" type="password" placeholder="••••••••" />

          <div className="flex items-center justify-between text-sm">
            <label
              className="flex cursor-pointer items-center gap-2"
              style={{ color: "var(--tpl-muted)" }}
            >
              <input type="checkbox" className="accent-[var(--tpl-accent)]" /> Remember me
            </label>
            <a href="#" className="hover:underline" style={{ color: "var(--tpl-accent)" }}>
              Forgot password?
            </a>
          </div>

          <PrimaryButton>Login</PrimaryButton>

          <SocialAuth onGoogle={goGoogle} onSso={goOidc} />

          <p className="text-center text-sm" style={{ color: "var(--tpl-muted)" }}>
            Don&apos;t have an account?{" "}
            <button
              type="button"
              onClick={() => setTab("register")}
              className="font-medium hover:underline"
              style={{ color: "var(--tpl-accent)" }}
            >
              Register now
            </button>
          </p>
        </form>
      ) : (
        <form
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            goOidc();
          }}
        >
          <h2 className="text-lg font-semibold">Create your account</h2>
          <div className="grid grid-cols-2 gap-3">
            <Field label="First name" name="first_name" />
            <Field label="Last name" name="last_name" />
          </div>
          <Field label="Your Email" name="email" type="email" />
          <Field label="Your Password" name="password" type="password" />

          <label
            className="flex cursor-pointer items-start gap-2 text-sm"
            style={{ color: "var(--tpl-muted)" }}
          >
            <input type="checkbox" className="mt-0.5 accent-[var(--tpl-accent)]" />
            <span>
              I accept the{" "}
              <a href="#" className="hover:underline" style={{ color: "var(--tpl-accent)" }}>
                Terms &amp; Conditions
              </a>{" "}
              of the website
            </span>
          </label>

          <PrimaryButton>Complete registration</PrimaryButton>

          <SocialAuth onGoogle={goGoogle} onSso={goOidc} />

          <p className="text-center text-xs" style={{ color: "var(--tpl-muted)" }}>
            Registration is completed through our secure single sign-on provider.
          </p>
        </form>
      )}
    </div>
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
}: {
  label: string;
  name: string;
  type?: string;
  placeholder?: string;
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
        autoComplete="off"
        className="w-full rounded-lg border bg-transparent px-3.5 py-2.5 text-sm outline-none transition focus:border-[var(--tpl-accent)]"
        style={{ borderColor: "var(--tpl-border)" }}
      />
    </label>
  );
}

function PrimaryButton({ children }: { children: ReactNode }) {
  return (
    <button
      type="submit"
      className="w-full rounded-lg px-4 py-2.5 text-sm font-semibold transition hover:opacity-90"
      style={{ background: "var(--tpl-accent)", color: "var(--tpl-accent-contrast)" }}
    >
      {children}
    </button>
  );
}

function Divider({ children }: { children: ReactNode }) {
  return (
    <div className="flex items-center gap-3 text-xs" style={{ color: "var(--tpl-muted)" }}>
      <span className="h-px flex-1" style={{ background: "var(--tpl-border)" }} />
      {children}
      <span className="h-px flex-1" style={{ background: "var(--tpl-border)" }} />
    </div>
  );
}

function SsoButton({ onClick }: { onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex w-full items-center justify-center gap-2 rounded-lg border px-4 py-2.5 text-sm font-medium transition hover:bg-white/5"
      style={{ borderColor: "var(--tpl-border)" }}
    >
      <ShieldIcon /> Continue with Single Sign-On
    </button>
  );
}

function SocialAuth({ onGoogle, onSso }: { onGoogle: () => void; onSso: () => void }) {
  return (
    <>
      <Divider>or continue with</Divider>
      <GoogleButton onClick={onGoogle} />
      <SsoButton onClick={onSso} />
    </>
  );
}

function GoogleButton({ onClick }: { onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex w-full items-center justify-center gap-3 rounded-lg border bg-white px-4 py-2.5 text-sm font-medium text-[#3c4043] transition hover:shadow"
      style={{ borderColor: "rgba(0,0,0,0.12)" }}
    >
      <GoogleIcon /> Continue with Google
    </button>
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

function ShieldIcon() {
  return (
    <svg {...iconProps()}>
      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
      <path d="m9 12 2 2 4-4" />
    </svg>
  );
}

function GoogleIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 48 48" aria-hidden>
      <path
        fill="#EA4335"
        d="M24 9.5c3.54 0 6.71 1.22 9.21 3.6l6.85-6.85C35.9 2.38 30.47 0 24 0 14.62 0 6.51 5.38 2.56 13.22l7.98 6.19C12.43 13.72 17.74 9.5 24 9.5z"
      />
      <path
        fill="#4285F4"
        d="M46.98 24.55c0-1.57-.15-3.09-.38-4.55H24v9.02h12.94c-.58 2.96-2.26 5.48-4.78 7.18l7.73 6c4.51-4.18 7.09-10.36 7.09-17.65z"
      />
      <path
        fill="#FBBC05"
        d="M10.53 28.59c-.48-1.45-.76-2.99-.76-4.59s.27-3.14.76-4.59l-7.98-6.19C.92 16.46 0 20.12 0 24c0 3.88.92 7.54 2.56 10.78l7.97-6.19z"
      />
      <path
        fill="#34A853"
        d="M24 48c6.48 0 11.93-2.13 15.89-5.81l-7.73-6c-2.15 1.45-4.92 2.3-8.16 2.3-6.26 0-11.57-4.22-13.47-9.91l-7.98 6.19C6.51 42.62 14.62 48 24 48z"
      />
    </svg>
  );
}
