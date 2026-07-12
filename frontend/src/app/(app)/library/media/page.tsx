import type { Metadata } from "next";
import { Suspense } from "react";
import { activeTemplate } from "@/templates/registry";

export const metadata: Metadata = { title: "Media" };

/**
 * /library/media (SPEC-01 P0.4, [F006]). The view reads/writes its kind and
 * status filters through the URL (`useSearchParams`/`router.replace`), which
 * requires a Suspense boundary here or `next build` fails with
 * "missing-suspense-with-csr-bailout".
 */
export default function MediaLibraryPage() {
  const View = activeTemplate().views.libraryMedia;
  return (
    <Suspense fallback={null}>
      <View />
    </Suspense>
  );
}
