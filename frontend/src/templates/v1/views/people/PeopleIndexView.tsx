"use client";

import { useState } from "react";
import Link from "next/link";
import type { Route } from "next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type Birthday, createPerson, formatBirthday, listPeople, upcomingBirthdays } from "@/lib/people";
import { problemDisplayMessage } from "@/lib/problems";
import { ApiError } from "@/lib/api-client";

export function PeopleIndexView() {
  const qc = useQueryClient();
  const { data, isLoading } = useQuery({ queryKey: ["people"], queryFn: () => listPeople() });
  const { data: upcoming = [] } = useQuery({ queryKey: ["people", "upcoming"], queryFn: () => upcomingBirthdays(30) });

  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [relationship, setRelationship] = useState("");
  const [month, setMonth] = useState("");
  const [day, setDay] = useState("");
  const [year, setYear] = useState("");
  const [err, setErr] = useState<string | null>(null);

  const create = useMutation({
    mutationFn: () => {
      let birthday: Birthday | null = null;
      if (month && day) birthday = { month: Number(month), day: Number(day), year: year ? Number(year) : null, calendar: "solar" };
      return createPerson({ display_name: name, relationship: relationship || null, birthday });
    },
    onSuccess: () => {
      setName(""); setRelationship(""); setMonth(""); setDay(""); setYear(""); setOpen(false); setErr(null);
      qc.invalidateQueries({ queryKey: ["people"] });
    },
    onError: (e) => setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : "Could not save"),
  });

  const people = data?.people ?? [];

  return (
    <main className="mx-auto max-w-3xl p-6 text-white">
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold">People</h1>
        <button className="rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium hover:bg-blue-500" onClick={() => setOpen((o) => !o)}>+ Add person</button>
      </div>

      {upcoming.length > 0 && (
        <div className="mb-6 rounded-lg border border-gray-800 bg-gray-900 p-4">
          <h2 className="mb-2 text-sm font-semibold uppercase text-gray-500">Upcoming birthdays</h2>
          <ul className="space-y-1 text-sm">
            {upcoming.slice(0, 5).map((u) => (
              <li key={u.person_id} className="flex justify-between">
                <span>{u.display_name}{u.age_turning != null ? ` (turns ${u.age_turning})` : ""}</span>
                <span className="text-gray-400">{u.days_until === 0 ? "today 🎂" : `in ${u.days_until} day${u.days_until === 1 ? "" : "s"}`}</span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {open && (
        <form className="mb-6 grid grid-cols-3 gap-2 rounded-lg border border-gray-800 bg-gray-900 p-4" onSubmit={(e) => { e.preventDefault(); if (name.trim()) create.mutate(); }}>
          <input className="col-span-3 rounded-md border border-gray-700 bg-gray-800 px-3 py-2" placeholder="Name (e.g. Mẹ)" value={name} onChange={(e) => setName(e.target.value)} />
          <input className="col-span-3 rounded-md border border-gray-700 bg-gray-800 px-3 py-2" placeholder="Relationship (e.g. mẹ, bạn đại học)" value={relationship} onChange={(e) => setRelationship(e.target.value)} />
          <input className="rounded-md border border-gray-700 bg-gray-800 px-3 py-2" placeholder="Day" inputMode="numeric" value={day} onChange={(e) => setDay(e.target.value)} />
          <input className="rounded-md border border-gray-700 bg-gray-800 px-3 py-2" placeholder="Month" inputMode="numeric" value={month} onChange={(e) => setMonth(e.target.value)} />
          <input className="rounded-md border border-gray-700 bg-gray-800 px-3 py-2" placeholder="Year (optional)" inputMode="numeric" value={year} onChange={(e) => setYear(e.target.value)} />
          <button type="submit" disabled={create.isPending || !name.trim()} className="col-span-3 rounded-md bg-blue-600 py-2 text-sm font-medium disabled:opacity-50">Add</button>
          {err && <p className="col-span-3 text-sm text-red-400">{err}</p>}
        </form>
      )}

      {isLoading ? (
        <p className="text-gray-400">Loading…</p>
      ) : people.length === 0 ? (
        <p className="text-gray-500">No people yet — add family and friends to track birthdays.</p>
      ) : (
        <ul className="divide-y divide-gray-800 rounded-lg border border-gray-800 bg-gray-900">
          {people.map((p) => (
            <li key={p.id}>
              <Link href={`/people/${p.id}` as Route} className="flex items-center justify-between px-4 py-3 hover:bg-gray-800">
                <div>
                  <div className="font-medium">{p.display_name}</div>
                  {p.relationship && <div className="text-xs text-gray-500">{p.relationship}</div>}
                </div>
                {p.birthday && <span className="text-sm text-gray-400">🎂 {formatBirthday(p.birthday)}</span>}
              </Link>
            </li>
          ))}
        </ul>
      )}
    </main>
  );
}
