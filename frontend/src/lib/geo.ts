// Place search + slippy-map tile maths for the newsfeed location picker.
//
// Both services are keyless, CORS-open and called straight from the browser —
// the same arrangement `weather.ts` already uses for Open-Meteo's forecast API,
// so this adds a data source, not a new kind of dependency:
//
//   · search   → Open-Meteo geocoding (https://geocoding-api.open-meteo.com)
//   · map tile → OpenStreetMap raster tiles (https://tile.openstreetmap.org)
//
// There is deliberately NO map library: a slippy map is a grid of 256px images
// positioned by the Web-Mercator formulas below, which is ~40 lines and costs
// nothing in the bundle. Attribution is required by the OSM tile policy and is
// rendered by the picker.

export interface Place {
  name: string;
  lat: number;
  lon: number;
}

export interface PlaceResult extends Place {
  /** "Hanoi, Vietnam" style context line, already joined for display. */
  detail: string;
}

interface OMGeo {
  results?: {
    name: string;
    latitude: number;
    longitude: number;
    country?: string;
    admin1?: string;
    admin2?: string;
  }[];
}

/** Open-Meteo geocoding. Returns [] for a blank query, a miss, or a failure. */
export async function searchPlaces(query: string, signal?: AbortSignal): Promise<PlaceResult[]> {
  const q = query.trim();
  if (q.length < 2) return [];
  const url = new URL("https://geocoding-api.open-meteo.com/v1/search");
  url.searchParams.set("name", q);
  url.searchParams.set("count", "8");
  url.searchParams.set("language", "vi");
  url.searchParams.set("format", "json");
  try {
    const r = await fetch(url, { signal });
    if (!r.ok) return [];
    const data = (await r.json()) as OMGeo;
    return (data.results ?? []).map((p) => ({
      name: p.name,
      lat: p.latitude,
      lon: p.longitude,
      detail: [p.admin2, p.admin1, p.country].filter(Boolean).join(", "),
    }));
  } catch {
    return []; // abort or network error — the caller shows the empty state
  }
}

/* ── Web Mercator ─────────────────────────────────────────────────── */

export const TILE = 256;
export const MIN_ZOOM = 2;
export const MAX_ZOOM = 18;

export function lonToTileX(lon: number, z: number): number {
  return ((lon + 180) / 360) * 2 ** z;
}

export function latToTileY(lat: number, z: number): number {
  const clamped = Math.max(-85.05112878, Math.min(85.05112878, lat));
  const r = (clamped * Math.PI) / 180;
  return ((1 - Math.log(Math.tan(r) + 1 / Math.cos(r)) / Math.PI) / 2) * 2 ** z;
}

export function tileXToLon(x: number, z: number): number {
  return (x / 2 ** z) * 360 - 180;
}

export function tileYToLat(y: number, z: number): number {
  const n = Math.PI - (2 * Math.PI * y) / 2 ** z;
  return (180 / Math.PI) * Math.atan(0.5 * (Math.exp(n) - Math.exp(-n)));
}

/** OSM raster tile. `x` wraps around the antimeridian; `y` is clamped. */
export function tileURL(z: number, x: number, y: number): string {
  const n = 2 ** z;
  const wrapped = ((x % n) + n) % n;
  return `https://tile.openstreetmap.org/${z}/${wrapped}/${y}.png`;
}

/** Rounded to ~11 m — enough for a post's location, and keeps bodies short. */
export function fmtCoord(v: number): string {
  return v.toFixed(4);
}

/** Human-readable fallback name for a pin dropped straight on the map. */
export function coordName(lat: number, lon: number): string {
  return `${fmtCoord(lat)}, ${fmtCoord(lon)}`;
}

/** Deep link to the same point on openstreetmap.org. */
export function osmURL(lat: number, lon: number, zoom = 15): string {
  return `https://www.openstreetmap.org/?mlat=${lat}&mlon=${lon}#map=${zoom}/${lat}/${lon}`;
}
