import { ContinueRail } from "../../../templates/v1/views/library/ContinueRail";
import Link from "next/link";
import type { Route } from "next";

export default function LibraryPage() {
  return (
    <main className="p-8">
      <h1 className="text-3xl font-bold mb-8 text-white">Library</h1>
      
      <ContinueRail />

      <section className="mb-8">
        <h2 className="text-xl font-semibold mb-4 text-white">Categories</h2>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <Link
            href="/library/media"
            className="p-6 bg-gray-900 rounded-lg border border-gray-800 hover:bg-gray-800 transition-colors text-center"
          >
            <h3 className="text-lg font-medium text-white">Media</h3>
            <p className="text-sm text-gray-400 mt-2">Videos, Audio, and Images</p>
          </Link>
          {/* No novel index route yet (only /library/novel/[id]); cast until it lands. */}
          <Link
            href={"/library/novel" as Route}
            className="p-6 bg-gray-900 rounded-lg border border-gray-800 hover:bg-gray-800 transition-colors text-center"
          >
            <h3 className="text-lg font-medium text-white">Novels</h3>
            <p className="text-sm text-gray-400 mt-2">Read stories</p>
          </Link>
          <Link
            href="/library/comic"
            className="p-6 bg-gray-900 rounded-lg border border-gray-800 hover:bg-gray-800 transition-colors text-center"
          >
            <h3 className="text-lg font-medium text-white">Comics</h3>
            <p className="text-sm text-gray-400 mt-2">Read comics</p>
          </Link>
          <Link
            href={"/library/music" as Route}
            className="p-6 bg-gray-900 rounded-lg border border-gray-800 hover:bg-gray-800 transition-colors text-center"
          >
            <h3 className="text-lg font-medium text-white">Music</h3>
            <p className="text-sm text-gray-400 mt-2">Listen to tracks</p>
          </Link>
        </div>
      </section>
    </main>
  );
}
