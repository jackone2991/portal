import type { Metadata } from "next";
import { activeTemplate } from "@/templates/registry";

export const metadata: Metadata = { title: "Playlist" };

/**
 * "/library/music/playlists". A static segment, so Next resolves it ahead of
 * the sibling "[id]" track route — no collision.
 */
export default function PlaylistsPage() {
  const View = activeTemplate().views.libraryMusicPlaylists;
  return <View />;
}
