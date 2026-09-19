import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

// Unit tests for the pure modules under src/lib (SPEC-12 introduced the first
// three; none of them touches the DOM). Two things the defaults do not give:
// only `*.test.ts` is a test (a `.test.tsx` would not run — and the matrix
// checker accepts only what runs), and the app's `@/` alias resolves, so a
// test may import a module the way the app does rather than by relative path
// (the three existing ones predate this and import relatively). Run by
// `pnpm test`, which the `frontend` CI job executes between typecheck and
// build (backlog #10, closed 2026-09-19).
export default defineConfig({
  resolve: { alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) } },
  test: { environment: "node", include: ["src/**/*.test.ts"] },
});
