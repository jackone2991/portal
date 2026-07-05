# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this directory is

This is **third-party reference material, not active code.** It is the CRUMINA **"Olympus"** social-network HTML template (vendor: `crumina.net`), delivered as a set of static, pre-rendered HTML pages. The repo root [`../../CLAUDE.md`](../../CLAUDE.md) is authoritative and says of this tree: *"`template-main/` is reference material, not active code … Don't edit, don't import."* The real implementation is the Next.js app under `../../frontend/`.

Treat everything here as **read-only design reference** — a source of markup/CSS patterns to reimplement in the real frontend, never a dependency to import or ship.

## No build system

There is nothing to build, lint, or test — these are plain saved HTML pages. To view one, open the `.html` file directly in a browser, or serve the directory statically (e.g. `python -m http.server` from this folder) so the relative `./css`, `./js`, and `_files` asset paths resolve. There is no package manager, task runner, or test suite in this tree.

## Layout

- Each page is a browser **"complete webpage" save**: a `<Page Name>.html` file paired with a `<Page Name>_files/` folder holding that page's copied CSS/JS/images. The `_files` folders are largely duplicated per page — the same `jquery-3.5.1.min.js`, `webp-hero.bundle.js`, `ajax-pagination.min.js`, etc. recur across them. Don't treat a `_files` copy as canonical.
- Page collections:
  - `newsfeed.html` — top-level sample page.
  - `social/` — ~39 social pages (Newsfeed, Profile, Groups, Events, Music, Blog, popups).
  - `Olympus Company/` — ~21 company/commerce pages (About, Careers, Checkout, Merchandise, Blog, error pages).
  - `Olympus Components/` — ~22 component showcase pages (Forms, Widgets, Posts & Comments, Headers, Forums).
- Shared top-level assets (used by `newsfeed.html`): `css/` (Bootstrap + `main.min.css` + `light.css` theme + `theme-font.min.css`), `js/`, `font/` (Montserrat woff2), `img/`, `ico/`.

## Frontend architecture (for reimplementation reference)

- **jQuery-era, no framework.** Runtime stack is jQuery 3.5.1 + Bootstrap bundle, plus plugins: Isotope + imagesLoaded (masonry grids), Magnific Popup (lightbox), Selectize (tag/select inputs), Perfect Scrollbar, mousewheel, `ajax-pagination.min.js` (infinite-scroll via `$.ajax`), `webp-hero` + `polyfills.js` (WebP fallback).
- **Init pattern:** behavior is organized as `CRUMINA.<Component>` functions in `js/libs-init.js` (e.g. `CRUMINA.Bootstrap`), invoked on DOM ready. `js/main.js` holds core UI behavior (fixed header, sidebars, scroll-to-top, quantity spinners, custom scrollbars).
- **Styling:** Bootstrap base + `main.min.css` for the theme; `light.css` is the light color scheme (an RTL sheet is referenced but commented out). Icon font via `theme-font.min.css`.

## Important caveats

- **`js/main.js` is obfuscated** (string-array `_0x462c` pattern) and contains the vendor's domain/attribution check (the literal `"crumina.net"`). This is the template author's licensing/attribution code. Do **not** edit, deobfuscate, or strip it — this template is a licensed third-party asset, and removing that code is a license-circumvention concern, not a normal edit. If the design is wanted, either use a properly-licensed copy or reimplement the markup/CSS cleanly in `../../frontend/`.
- **Do not import from this tree into the real app.** These assets carry the third-party license and the tracking code above; the real frontend is a separate Next.js codebase. Use these pages only to read design intent, then rebuild in `../../frontend/`.
- The sibling `../portal/` tree is a separate Laravel/PHP reference scaffold and is likewise reference-only.
