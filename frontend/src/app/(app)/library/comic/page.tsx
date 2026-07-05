import type { Metadata } from "next";
import { activeTemplate } from "@/templates/registry";

export const metadata: Metadata = { title: "Comics" };

export default function ComicPage() {
  const View = activeTemplate().views.libraryComic;
  return <View />;
}
