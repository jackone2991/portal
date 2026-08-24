WeatherWidget from portal-frontend. Use via `window.PortalUI.WeatherWidget` (bundle loaded from the root `_ds_bundle.js`). Wrap the tree in `<DSProvider>` (full provider chain in README.md — components read theme/i18n from that context).

Weather rail — current conditions + 7-day from Open-Meteo (lib/weather) for the
browser's geolocation; links to the full /weather page. Falls back to a compact
prompt when location is unavailable — never fake weather.
