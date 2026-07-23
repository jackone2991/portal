// Weather data — Open-Meteo (free, keyless, CORS-open) for the browser's
// geolocation. Shared by the newsfeed WeatherWidget and the /weather page.
// (AccuWeather would need an API key + a backend proxy — see the weather page
// notes; Open-Meteo is the chosen source.)

import { useEffect, useState } from "react";

/** WMO weather code → one of the four Olympus sprite icons + a Vietnamese label. */
export function wmo(code: number): { icon: string; label: string } {
  if (code === 0) return { icon: "weather-sunny-icon", label: "Trời quang" };
  if (code <= 2) return { icon: "weather-partly-sunny-icon", label: "Ít mây" };
  if (code === 3) return { icon: "weather-cloudy-icon", label: "Nhiều mây" };
  if (code <= 48) return { icon: "weather-cloudy-icon", label: "Sương mù" };
  if (code <= 57) return { icon: "weather-rain-icon", label: "Mưa phùn" };
  if (code <= 67) return { icon: "weather-rain-icon", label: "Mưa" };
  if (code <= 77) return { icon: "weather-rain-icon", label: "Tuyết" };
  if (code <= 82) return { icon: "weather-rain-icon", label: "Mưa rào" };
  if (code <= 86) return { icon: "weather-rain-icon", label: "Mưa tuyết" };
  return { icon: "weather-rain-icon", label: "Giông bão" };
}

export interface WeatherNow {
  temp: number;
  feels: number;
  code: number;
  icon: string;
  label: string;
  low: number;
  high: number;
  rain: number;
  humidity: number;
  wind: number;
}
export interface WeatherHour { time: string; temp: number; icon: string }
export interface WeatherDay { dow: string; icon: string; label: string; low: number; high: number; rain: number }
export interface WeatherData { now: WeatherNow; hourly: WeatherHour[]; daily: WeatherDay[]; place: string | null }

const DOW_VI = ["CN", "T2", "T3", "T4", "T5", "T6", "T7"];

type OM = {
  current?: {
    time?: string;
    temperature_2m?: number;
    apparent_temperature?: number;
    relative_humidity_2m?: number;
    precipitation_probability?: number;
    weather_code?: number;
    wind_speed_10m?: number;
  };
  hourly?: { time?: string[]; temperature_2m?: number[]; weather_code?: number[] };
  daily?: {
    time?: string[];
    weather_code?: number[];
    temperature_2m_max?: number[];
    temperature_2m_min?: number[];
    precipitation_probability_max?: number[];
  };
};

/** Fetch current + 24h hourly + 7-day weather for a coordinate (metric, °C).
 *  `place` is an optional human label for the location (from config). */
export async function fetchWeather(lat: number, lon: number, place: string | null = null): Promise<WeatherData> {
  const url =
    `https://api.open-meteo.com/v1/forecast?latitude=${lat.toFixed(3)}&longitude=${lon.toFixed(3)}` +
    `&current=temperature_2m,apparent_temperature,relative_humidity_2m,precipitation_probability,weather_code,wind_speed_10m` +
    `&hourly=temperature_2m,weather_code` +
    `&daily=weather_code,temperature_2m_max,temperature_2m_min,precipitation_probability_max` +
    `&timezone=auto&forecast_days=7`;
  const res = await fetch(url);
  if (!res.ok) throw new Error(String(res.status));
  const j = (await res.json()) as OM;
  const c = j.current ?? {};
  const d = j.daily ?? {};
  const h = j.hourly ?? {};
  const cw = wmo(Math.round(c.weather_code ?? 0));

  const now: WeatherNow = {
    temp: Math.round(c.temperature_2m ?? 0),
    feels: Math.round(c.apparent_temperature ?? c.temperature_2m ?? 0),
    code: Math.round(c.weather_code ?? 0),
    icon: cw.icon,
    label: cw.label,
    low: Math.round(d.temperature_2m_min?.[0] ?? 0),
    high: Math.round(d.temperature_2m_max?.[0] ?? 0),
    rain: Math.round(c.precipitation_probability ?? 0),
    humidity: Math.round(c.relative_humidity_2m ?? 0),
    wind: Math.round(c.wind_speed_10m ?? 0),
  };

  const times = h.time ?? [];
  let start = 0;
  if (c.time) {
    const ct = c.time;
    const idx = times.findIndex((t) => t >= ct);
    if (idx > 0) start = idx;
  }
  const hourly: WeatherHour[] = times.slice(start, start + 24).map((t, i) => ({
    time: t.slice(11, 16),
    temp: Math.round(h.temperature_2m?.[start + i] ?? 0),
    icon: wmo(Math.round(h.weather_code?.[start + i] ?? 0)).icon,
  }));

  const daily: WeatherDay[] = (d.time ?? []).slice(0, 7).map((iso, i) => {
    const w = wmo(Math.round(d.weather_code?.[i] ?? 0));
    return {
      dow: i === 0 ? "Hôm nay" : DOW_VI[new Date(`${iso}T12:00:00Z`).getUTCDay()] ?? "",
      icon: w.icon,
      label: w.label,
      low: Math.round(d.temperature_2m_min?.[i] ?? 0),
      high: Math.round(d.temperature_2m_max?.[i] ?? 0),
      rain: Math.round(d.precipitation_probability_max?.[i] ?? 0),
    };
  });

  return { now, hourly, daily, place };
}

export type WeatherState = { status: "loading" | "ok" | "unavailable"; data: WeatherData | null };

/**
 * A fixed location from build-time config (`NEXT_PUBLIC_WEATHER_LAT` / `_LON` /
 * `_PLACE`), or `null` if not configured. When set, weather uses it directly
 * instead of prompting the browser for geolocation — reliable on desktop and when
 * the geolocation permission is denied. An empty/unset env yields `0` → treated as
 * not configured.
 */
export function configuredLocation(): { lat: number; lon: number; place: string | null } | null {
  const lat = Number(process.env.NEXT_PUBLIC_WEATHER_LAT);
  const lon = Number(process.env.NEXT_PUBLIC_WEATHER_LON);
  if (!Number.isFinite(lat) || !Number.isFinite(lon) || (lat === 0 && lon === 0)) return null;
  const place = (process.env.NEXT_PUBLIC_WEATHER_PLACE ?? "").trim();
  return { lat, lon, place: place || null };
}

/**
 * Weather source for the UI. Prefers the configured location (above); if none is
 * configured, falls back to the browser's geolocation. Degrades to "unavailable"
 * when neither is available or the fetch fails (never fabricates weather).
 */
export function useGeoWeather(): WeatherState {
  const [state, setState] = useState<WeatherState>({ status: "loading", data: null });
  useEffect(() => {
    let alive = true;

    // 1) Configured location wins — no geolocation prompt needed.
    const cfg = configuredLocation();
    if (cfg) {
      fetchWeather(cfg.lat, cfg.lon, cfg.place)
        .then((data) => { if (alive) setState({ status: "ok", data }); })
        .catch(() => { if (alive) setState({ status: "unavailable", data: null }); });
      return () => { alive = false; };
    }

    // 2) Otherwise fall back to the browser's geolocation.
    if (typeof navigator === "undefined" || !navigator.geolocation) {
      setState({ status: "unavailable", data: null });
      return;
    }
    navigator.geolocation.getCurrentPosition(
      async (pos) => {
        try {
          const data = await fetchWeather(pos.coords.latitude, pos.coords.longitude);
          if (alive) setState({ status: "ok", data });
        } catch {
          if (alive) setState({ status: "unavailable", data: null });
        }
      },
      () => {
        if (alive) setState({ status: "unavailable", data: null });
      },
      { timeout: 8000, maximumAge: 30 * 60 * 1000 },
    );
    return () => {
      alive = false;
    };
  }, []);
  return state;
}
