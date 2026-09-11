// Design-sync shim: next/navigation's hooks read the App Router context, which
// only a running Next app mounts. In the design tool every card renders
// standalone, so useRouter() throws "invariant expected app router to be
// mounted" and the whole preview blank-renders — it took out TopMenu,
// MasterBase, SidebarCenter and NotificationsMenu the first time the v1
// templates started navigating. Alias next/navigation -> this inert router
// (see tsconfig.ds.json paths).
//
// Navigation is a no-op by design: a preview card has nowhere to navigate to,
// and a thrown error is the only outcome worth preventing here.
import * as React from "react";

const noop = () => {};

export function useRouter() {
  return React.useMemo(
    () => ({
      push: noop,
      replace: noop,
      back: noop,
      forward: noop,
      refresh: noop,
      prefetch: async () => {},
    }),
    [],
  );
}

// Components branch on the current path to mark the active nav item. "/" keeps
// that logic on its default branch instead of highlighting an arbitrary entry.
export function usePathname() {
  return "/";
}

export function useSearchParams() {
  return React.useMemo(() => new URLSearchParams(), []);
}

export function useParams<T extends Record<string, string | string[]> = Record<string, string>>() {
  return {} as T;
}

export function redirect(_url: string): never {
  throw new Error("design-sync preview: redirect() is inert");
}

export function notFound(): never {
  throw new Error("design-sync preview: notFound() is inert");
}
