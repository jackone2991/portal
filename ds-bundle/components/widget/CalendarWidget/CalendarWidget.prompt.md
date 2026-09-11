CalendarWidget from portal-frontend. Use via `window.PortalUI.CalendarWidget` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Month-calendar rail (âm dương). Opens on the current month per the app time
config (server clock + APP_TIMEZONE) and lets you page through other months and
years in place — prev/next month (‹ ›) and prev/next year (« ») — instead of
navigating away. Each day shows its solar + lunar date; days with a journal
entry get a dot. A footer link still opens the full /calendar page. Lunar dates
are computed for Vietnam (UTC+7) via lib/lunar, matching that page.

## Examples

### WithEntries

```jsx
() => frame({ items: entries, next_cursor: null }, <CalendarWidget />);

// A month nobody wrote in: the grid still renders, just undotted. That is an
// ordinary state for a new journal, not an empty-data failure.
```

### EmptyMonth

```jsx
() => frame({ items: [], next_cursor: null }, <CalendarWidget />)
```
