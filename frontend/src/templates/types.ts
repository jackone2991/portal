import type { ComponentType, ReactNode } from "react";

/** Props every template "shell" (master layout) receives. */
export interface ShellProps {
  children: ReactNode;
}

/**
 * A versioned frontend template = a set of layout shells + page views.
 *
 * Mirrors the Blade reference at
 * `template-main/portal/resources/views/v1/**`, where the whole presentation
 * layer is namespaced by version. Adding a redesign ("v2") means implementing
 * this same contract under `templates/v2/` — no `app/` route changes, no URL
 * changes. The active version is chosen in `registry.ts`.
 */
export interface TemplateManifest {
  /** Folder/version id, e.g. "v1". Must match a key in the registry. */
  version: string;
  /** Human-readable label, shown in dev tooling. */
  label: string;

  /** Layout shells — port of `master/master-*.blade.php`. */
  shells: {
    /** Guest / marketing shell (≈ `master-public.blade.php`). */
    public: ComponentType<ShellProps>;
    /** Authenticated app shell with sidebars (≈ `master-base.blade.php`). */
    app: ComponentType<ShellProps>;
  };

  /** Page views — port of `resources/views/v1/**`. */
  views: {
    /** `views/home/home.blade.php` */
    home: ComponentType;
    /** `public/login.blade.php` */
    login: ComponentType;
    /** `public/register.blade.php` */
    register: ComponentType;
    /** `views/library/commic/index.blade.php` (typo "commic" fixed) */
    libraryComic: ComponentType;
    /** SPEC-02 P0.5 — comic detail (chapters, continue, creator controls). */
    libraryComicDetail: ComponentType<{ id: string }>;
    /** SPEC-02 P0.3 — comic reader (vertical scroll + resume). */
    libraryComicReader: ComponentType<{ id: string; chapterId: string }>;
    /** `views/library/novel/detail.blade.php` */
    libraryNovelDetail: ComponentType<{ id: string }>;
    /** SPEC-01 P0.4 ([F006]) — no Blade equivalent; media library grid. */
    libraryMedia: ComponentType;
    /** SPEC-07 P0.4 — media player with resume. */
    libraryMediaDetail: ComponentType<{ id: string }>;
    /** SPEC-03 §8 — personal ledger (bank): dashboard, transactions, accounts, budgets. */
    bankDashboard: ComponentType;
    bankTransactions: ComponentType;
    bankAccounts: ComponentType;
    bankBudgets: ComponentType;
    /** SPEC-08 P0.5 — people registry (contacts + birthdays). */
    peopleList: ComponentType;
    peopleDetail: ComponentType<{ id: string }>;
    /** Calendar & Events — month view of journal notes; add a note to any day. */
    calendar: ComponentType;
    /** Weather — current + hourly + 7-day (Open-Meteo, geolocation). */
    weather: ComponentType;
  };
}
