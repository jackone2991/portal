// Data layer for the people registry (SPEC-08). TanStack owns server state (D-32).

import { api } from "./api-client";

export interface Birthday {
  month: number;
  day: number;
  year: number | null;
  calendar?: "solar" | "lunar";
}

/**
 * Which fixed section of the people rail someone appears in (migration 0035).
 * Beside `relationship`, not instead of it: relationship is the free text you
 * wrote, circle is the closed set the UI groups by.
 */
export type PersonCircle = "close_friend" | "family" | "other";

export const CIRCLE_LABEL: Record<PersonCircle, string> = {
  close_friend: "Close Friends",
  family: "My Family",
  other: "Uncategorized",
};

export interface Person {
  id: string;
  display_name: string;
  relationship: string | null;
  circle: PersonCircle;
  /** The portal account this entry stands for, when it stands for one. */
  linked_user_id: string | null;
  birthday: Birthday | null;
  contact: Record<string, unknown>;
  note_md: string | null;
  avatar_asset_id: string | null;
  created_at: string;
  updated_at: string;
}

export interface UpcomingBirthday {
  person_id: string;
  display_name: string;
  next_occurrence: string; // YYYY-MM-DD
  days_until: number;
  age_turning?: number;
}

export interface PeoplePage {
  people: Person[];
  next_cursor?: string | null;
}

export interface PersonWrite {
  display_name?: string;
  relationship?: string | null;
  birthday?: Birthday | null;
  note_md?: string | null;
  circle?: PersonCircle;
  /** Create only — records that this person is the given portal account. */
  linked_user_id?: string;
}

/** One "people you may know" entry: an account not yet in the registry. */
export interface PersonSuggestion {
  user_id: string;
  display_name: string;
}

export async function listPeople(cursor?: string, circle?: PersonCircle): Promise<PeoplePage> {
  const p = new URLSearchParams();
  if (cursor) p.set("cursor", cursor);
  if (circle) p.set("circle", circle);
  const qs = p.toString();
  const r = await api<PeoplePage>(`/api/v1/people${qs ? `?${qs}` : ""}`);
  return { people: r.people ?? [], next_cursor: r.next_cursor };
}

/**
 * `GET /api/v1/people/suggestions` — other accounts on this instance that are
 * not in the registry yet. Empty is the normal answer on a single-user install.
 */
export async function listSuggestions(): Promise<PersonSuggestion[]> {
  const r = await api<{ suggestions?: PersonSuggestion[] }>("/api/v1/people/suggestions");
  return r.suggestions ?? [];
}
export async function getPerson(id: string): Promise<Person> {
  return api<Person>(`/api/v1/people/${id}`);
}
export async function createPerson(body: PersonWrite): Promise<Person> {
  return api<Person>("/api/v1/people", { method: "POST", body: JSON.stringify(body) });
}
export async function updatePerson(id: string, body: PersonWrite): Promise<Person> {
  return api<Person>(`/api/v1/people/${id}`, { method: "PATCH", body: JSON.stringify(body) });
}
export async function deletePerson(id: string): Promise<void> {
  await api<void>(`/api/v1/people/${id}`, { method: "DELETE" });
}
export async function upcomingBirthdays(days = 14): Promise<UpcomingBirthday[]> {
  const r = await api<{ upcoming: UpcomingBirthday[] }>(`/api/v1/people/upcoming-birthdays?days=${days}`);
  return r.upcoming ?? [];
}

/** "15/03" or "15/03/1990" for display. */
export function formatBirthday(b: Birthday | null): string {
  if (!b) return "";
  const dd = String(b.day).padStart(2, "0");
  const mm = String(b.month).padStart(2, "0");
  return b.year ? `${dd}/${mm}/${b.year}` : `${dd}/${mm}`;
}
