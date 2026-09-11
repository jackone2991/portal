import type { Metadata } from "next";
import { activeTemplate } from "@/templates/registry";

export const metadata: Metadata = { title: "Nhạc" };

export default function MusicPage() {
  const View = activeTemplate().views.libraryMusic;
  return <View />;
}
