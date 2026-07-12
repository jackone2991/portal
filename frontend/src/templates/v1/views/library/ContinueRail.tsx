"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import type { Route } from "next";
import { getContinueItems } from "../../../../lib/media-assets";
import { Icon } from "../../components/ui/Icon";

export function ContinueRail() {
  const { data, isLoading } = useQuery({
    queryKey: ["continue-items"],
    queryFn: () => getContinueItems(10),
  });

  if (isLoading) {
    return <div className="p-4">Loading continue watching...</div>;
  }

  if (!data || data.items.length === 0) {
    return null; // Don't show rail if empty
  }

  return (
    <section className="mb-8">
      <h2 className="text-xl font-semibold mb-4 text-white">Continue Watching</h2>
      <div className="flex gap-4 overflow-x-auto pb-4 snap-x">
        {data.items.map((item) => (
          <Link
            key={item.ref_id}
            href={item.href as Route}
            className="flex-shrink-0 w-64 group snap-start relative rounded-lg overflow-hidden bg-gray-900 border border-gray-800"
          >
            {item.poster_url ? (
              <img
                src={item.poster_url}
                alt={item.title}
                className="w-full h-36 object-cover"
              />
            ) : (
              <div className="w-full h-36 bg-gray-800 flex items-center justify-center">
                <Icon name="play" className="w-8 h-8 text-gray-500" />
              </div>
            )}
            
            {/* Progress bar */}
            <div className="absolute bottom-0 left-0 right-0 h-1 bg-gray-700">
              <div
                className="h-full bg-blue-500"
                style={{ width: `${item.progress_pct}%` }}
              />
            </div>

            {/* Hover overlay */}
            <div className="absolute inset-0 bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center">
              <div className="bg-white/20 p-3 rounded-full backdrop-blur-sm">
                <Icon name="play" className="w-6 h-6 text-white" />
              </div>
            </div>

            <div className="p-3">
              <h3 className="text-sm font-medium text-white truncate">{item.title}</h3>
              <p className="text-xs text-gray-400 mt-1 capitalize">{item.module}</p>
            </div>
          </Link>
        ))}
      </div>
    </section>
  );
}
