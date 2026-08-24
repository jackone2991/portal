// Design-sync render provider. Icon (and everything composing it: TopMenu,
// the sidebars, views) renders via <use href="#olymp-*"> against symbols that
// <SvgSprite/> injects. Each design-tool card renders standalone, so every
// preview must carry the sprite — cfg.provider mounts this around every card.
import * as React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { SvgSprite } from "../src/templates/v1/components/footers/SvgSprite";

// Several Olympus components read `process.env.NEXT_PUBLIC_API_BASE_URL` at
// module top level; in a browser IIFE `process` is undefined and the whole
// bundle throws before assigning window.PortalUI. This provider entry is
// bundled ahead of the component modules (converter emits extraEntries first),
// so defining `process` here runs before any component evaluates. Idempotent.
(globalThis as unknown as { process?: { env: Record<string, string | undefined> } }).process ??= {
  env: {},
};

// Server state is TanStack Query's (D-32), so every view that loads data calls
// useQuery/useMutation — which throw "No QueryClient set" without a provider and
// blank-render the card. One client per module is fine: cards are independent
// renders and nothing here is shared across them.
//
// retry:false is what keeps a card from hanging: previews render with no API
// origin, so every query fails, and the default exponential retry would leave
// the card in `isLoading` well past the screenshot. Failing immediately renders
// the component's real empty/error state, which is the honest preview anyway.
const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: false, refetchOnWindowFocus: false, staleTime: Infinity },
    mutations: { retry: false },
  },
});

// Preview-only escape hatch for the views that fetch their own data and take no
// data props (ContinueRail, MediaDetailView, PersonDetailView). Those render
// "not found" — or literally `null` — against no API, so the only way to card
// them is to seed the cache they read.
//
// It MUST live here rather than in the preview .tsx files: previews externalize
// only react and the DS package, so a preview importing @tanstack/react-query
// directly would bundle a SECOND copy, and its provider's context would not be
// the one the bundled components read — the card would fail "No QueryClient
// set" exactly as if there were no provider at all. Exported from the bundle,
// there is one instance. extraEntries exports aren't matched to src files, so
// this does not become a component card.
export function DSQuerySeed({
  seed,
  children,
}: {
  seed?: ReadonlyArray<readonly [readonly unknown[], unknown]>;
  children?: React.ReactNode;
}) {
  const client = React.useMemo(() => {
    const qc = new QueryClient({
      defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false, staleTime: Infinity } },
    });
    for (const [key, value] of seed ?? []) qc.setQueryData(key as unknown[], value);
    return qc;
  }, [seed]);
  return React.createElement(QueryClientProvider, { client }, children as React.ReactNode);
}

export function DSProvider({ children }: { children?: React.ReactNode }) {
  return React.createElement(
    QueryClientProvider,
    { client: queryClient },
    React.createElement(SvgSprite, null),
    children as React.ReactNode,
  );
}
