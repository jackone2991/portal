CalendarWidget from portal-frontend. Use via `window.PortalUI.CalendarWidget` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Month-calendar rail (âm dương). Opens on the current month per the app time
config (server clock + APP_TIMEZONE) and lets you page through other months and
years in place — prev/next month (‹ ›) and prev/next year (« ») — instead of
navigating away. Each day shows its solar + lunar date; days with a journal
entry get a dot. A footer link still opens the full /calendar page. Lunar dates
are computed for Vietnam (UTC+7) via lib/lunar, matching that page.
