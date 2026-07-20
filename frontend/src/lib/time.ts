// Central time config for the whole app — ONE source of truth for how every date
// is displayed and reckoned. The display timezone + the server clock both come
// from the API (GET /api/v1/time), driven by APP_TIMEZONE in .env. Change that one
// env value and the entire UI's date/time shifts.
//
// RULE FOR FUTURE DATE CODE: never call raw `new Date()` / `toLocale*` / `getUTC*`
// for display. Go through this module — `useTimeConfig()` for the clock+tz, then
// `formatDate` / `formatDateTime` / `zonedYMD` / `relativeTime`. That keeps a
// single timezone switch across the app and every date consistent.

import { useQuery } from "@tanstack/react-query";
import { baseURL } from "./api-client";

export interface TimeConfig {
  /** The server's current instant (authoritative — not the browser clock). */
  now: Date;
  /** IANA display timezone, e.g. "UTC" or "Asia/Ho_Chi_Minh" (APP_TIMEZONE). */
  timezone: string;
}

const FALLBACK_TZ = "UTC";
const LOCALE = "en-GB"; // day-month order; the app UI is English-only

/**
 * Fetches the app clock + display timezone from the API. Falls back to the client
 * clock + UTC only if the API is unreachable or misconfigured.
 */
export async function getTimeConfig(): Promise<TimeConfig> {
  try {
    const res = await fetch(`${baseURL}/api/v1/time`, { cache: "no-store", credentials: "include" });
    if (res.ok) {
      const j = (await res.json()) as { now?: string; timezone?: string };
      const now = j.now ? new Date(j.now) : new Date();
      const timezone = isValidTz(j.timezone) ? (j.timezone as string) : FALLBACK_TZ;
      if (!Number.isNaN(now.getTime())) return { now, timezone };
    }
  } catch {
    // offline / blocked — fall through to the client clock
  }
  return { now: new Date(), timezone: FALLBACK_TZ };
}

/** React hook: the app's TimeConfig (TanStack-cached, one fetch shared app-wide). */
export function useTimeConfig() {
  return useQuery({
    queryKey: ["time-config"],
    queryFn: getTimeConfig,
    staleTime: 5 * 60 * 1000,
    retry: false,
  });
}

function isValidTz(tz: string | undefined): boolean {
  if (!tz) return false;
  try {
    new Intl.DateTimeFormat(LOCALE, { timeZone: tz });
    return true;
  } catch {
    return false;
  }
}

/** Year / 0-based month / day-of-month of `date`, reckoned IN the given timezone. */
export function zonedYMD(date: Date, tz: string): { year: number; month: number; day: number } {
  const parts = new Intl.DateTimeFormat("en-US", {
    timeZone: tz,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(date);
  const val = (t: Intl.DateTimeFormatPartTypes) => Number(parts.find((p) => p.type === t)?.value);
  return { year: val("year"), month: val("month") - 1, day: val("day") };
}

/** Compact date in the configured tz, e.g. "12 Jul". */
export function formatDate(iso: string, tz: string): string {
  return new Intl.DateTimeFormat(LOCALE, { timeZone: tz, day: "numeric", month: "short" }).format(new Date(iso));
}

/** Date + time in the configured tz, e.g. "12 Jul 2026, 18:40". */
export function formatDateTime(iso: string, tz: string): string {
  return new Intl.DateTimeFormat(LOCALE, {
    timeZone: tz,
    day: "numeric",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(iso));
}

/** A `YYYY-MM-DDTHH:mm` value for a `<input type="datetime-local">`, in the given tz. */
export function toDatetimeLocalInTz(date: Date, tz: string): string {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: tz,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).formatToParts(date);
  const p = (t: Intl.DateTimeFormatPartTypes) => parts.find((x) => x.type === t)?.value ?? "00";
  const hour = p("hour") === "24" ? "00" : p("hour"); // en-CA can emit "24" at midnight
  return `${p("year")}-${p("month")}-${p("day")}T${hour}:${p("minute")}`;
}

/**
 * Inverse of `toDatetimeLocalInTz`: interpret a `YYYY-MM-DDTHH:mm` wall-clock as a
 * time IN `tz` and return the UTC ISO instant. (Ignores the rare DST-fold
 * ambiguity — fine for a "when did this happen" field.)
 */
export function fromDatetimeLocalInTz(local: string, tz: string): string {
  const asUTC = new Date(`${local}:00Z`); // parse the wall-clock digits as if UTC
  if (Number.isNaN(asUTC.getTime())) return new Date(local).toISOString();
  const parts = new Intl.DateTimeFormat("en-US", {
    timeZone: tz,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).formatToParts(asUTC);
  const g = (t: Intl.DateTimeFormatPartTypes) => Number(parts.find((p) => p.type === t)?.value);
  const hh = g("hour") === 24 ? 0 : g("hour"); // en-US can emit "24" at midnight
  const shownAsUTC = Date.UTC(g("year"), g("month") - 1, g("day"), hh, g("minute"));
  return new Date(asUTC.getTime() - (shownAsUTC - asUTC.getTime())).toISOString();
}

/** "5m ago" relative to a reference instant in ms (pass the server clock). */
export function relativeTime(iso: string, nowMs: number): string {
  const secs = Math.max(1, Math.floor((nowMs - new Date(iso).getTime()) / 1000));
  if (secs < 60) return `${secs}s ago`;
  const mins = Math.floor(secs / 60);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return `${Math.floor(hrs / 24)}d ago`;
}
