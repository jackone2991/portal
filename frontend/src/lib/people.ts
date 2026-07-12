// Data layer for the people registry (SPEC-08). TanStack owns server state (D-32).

import { api } from "./api-client";

export interface Birthday {
  month: number;
  day: number;
  year: number | null;
  calendar?: "solar" | "lunar";
}

export interface Person {
  id: string;
  display_name: string;
  relationship: string | null;
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
}

export async function listPeople(cursor?: string): Promise<PeoplePage> {
  const q = cursor ? `?cursor=${cursor}` : "";
  const r = await api<PeoplePage>(`/api/v1/people${q}`);
  return { people: r.people ?? [], next_cursor: r.next_cursor };
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
