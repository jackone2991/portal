import type { Metadata } from "next";
import { Suspense } from "react";
import { activeTemplate } from "@/templates/registry";

export const metadata: Metadata = { title: "People" };

/**
 * /people — the registry, and the management page behind each section of the
 * right rail (SPEC-08 P0.5). The view reads its section from the URL
 * (`?circle=…` via `useSearchParams`), which requires a Suspense boundary here
 * or `next build` fails with "missing-suspense-with-csr-bailout" — the same
 * reason /library/media has one.
 */
export default function PeoplePage() {
  const View = activeTemplate().views.peopleList;
  return (
    <Suspense fallback={null}>
      <View />
    </Suspense>
  );
}
