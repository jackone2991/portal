import type { Metadata } from "next";
import { activeTemplate } from "@/templates/registry";

export const metadata: Metadata = { title: "Calendar & Events" };

/** /calendar — month view of journal notes; add a note to any day. */
export default function CalendarPage() {
  const View = activeTemplate().views.calendar;
  return <View />;
}
