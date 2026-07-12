import type { Metadata } from "next";
import { activeTemplate } from "@/templates/registry";

export const metadata: Metadata = { title: "People" };

/** /people — contacts + birthdays list (SPEC-08 P0.5). */
export default function PeoplePage() {
  const View = activeTemplate().views.peopleList;
  return <View />;
}
