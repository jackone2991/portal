// Compile Tailwind v4 utilities used across templates/v1 + the theme tokens into
// one static stylesheet the converter appends to _ds_bundle.css (cfg.cssEntry).
// Run from frontend/: `node .ds-compile-css.mjs`. Regenerable build input.
import { readFileSync, writeFileSync } from "node:fs";
import postcss from "postcss";
import twMod from "@tailwindcss/postcss";

const tailwind = twMod.default ?? twMod;

const base = readFileSync(".ds-tw-input.css", "utf8");
const tokens = readFileSync("src/templates/v1/theme/theme.css", "utf8");
// Inline the theme tokens so the :root custom properties land in the output
// regardless of @import resolution order.
const input = `${base}\n/* ---- inlined theme.css tokens ---- */\n${tokens}\n`;

const res = await postcss([tailwind()]).process(input, {
  from: ".ds-tw-input.css",
  to: ".ds-compiled.css",
});
writeFileSync(".ds-compiled.css", res.css);
console.error(`compiled .ds-compiled.css — ${res.css.length} bytes`);
