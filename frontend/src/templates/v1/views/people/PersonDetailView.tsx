"use client";

import { useRouter } from "next/navigation";
import Link from "next/link";
import type { Route } from "next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { deletePerson, formatBirthday, getPerson } from "@/lib/people";

export function PersonDetailView({ id }: { id: string }) {
  const qc = useQueryClient();
  const router = useRouter();
  const { data: person, isLoading } = useQuery({ queryKey: ["person", id], queryFn: () => getPerson(id) });

  const remove = useMutation({
    mutationFn: () => deletePerson(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["people"] });
      router.push("/people" as Route);
    },
  });

  if (isLoading) return <div className="p-6 text-gray-400">Loading…</div>;
  if (!person) return <div className="p-6 text-white">Person not found</div>;

  const contactEntries = Object.entries(person.contact ?? {});

  return (
    <main className="mx-auto max-w-2xl p-6 text-white">
      <Link href={"/people" as Route} className="text-sm text-blue-400 hover:underline">← People</Link>
      <div className="mt-3 flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-bold">{person.display_name}</h1>
          {person.relationship && <p className="text-sm text-gray-400">{person.relationship}</p>}
        </div>
        <button className="rounded-md border border-red-800 px-3 py-1.5 text-sm text-red-400 hover:bg-red-950" onClick={() => remove.mutate()} disabled={remove.isPending}>Delete</button>
      </div>

      <dl className="mt-6 space-y-3 rounded-lg border border-gray-800 bg-gray-900 p-4 text-sm">
        {person.birthday && (
          <div className="flex justify-between">
            <dt className="text-gray-500">Birthday</dt>
            <dd>🎂 {formatBirthday(person.birthday)}</dd>
          </div>
        )}
        {contactEntries.map(([k, v]) => (
          <div key={k} className="flex justify-between">
            <dt className="capitalize text-gray-500">{k}</dt>
            <dd>{String(v)}</dd>
          </div>
        ))}
        {person.note_md && (
          <div>
            <dt className="text-gray-500">Notes</dt>
            <dd className="mt-1 whitespace-pre-wrap text-gray-300">{person.note_md}</dd>
          </div>
        )}
        {!person.birthday && contactEntries.length === 0 && !person.note_md && (
          <p className="text-gray-500">No details yet.</p>
        )}
      </dl>
    </main>
  );
}
