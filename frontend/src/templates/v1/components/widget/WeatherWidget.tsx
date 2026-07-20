"use client";

import Link from "next/link";
import type { Route } from "next";
import { Icon } from "../ui/Icon";
import { Card } from "./WidgetCard";
import { useGeoWeather } from "@/lib/weather";

/**
 * Weather rail — current conditions + 7-day from Open-Meteo (lib/weather) for the
 * browser's geolocation; links to the full /weather page. Falls back to a compact
 * prompt when location is unavailable — never fake weather.
 */
export function WeatherWidget() {
  const { status, data } = useGeoWeather();

  if (status !== "ok" || !data) {
    return (
      <Card className="p-4">
        <div className="flex items-center gap-3">
          <span style={{ color: "var(--tpl-muted)" }}><Icon name="weather-partly-sunny-icon" size={22} /></span>
          <div>
            <p className="text-sm font-bold" style={{ color: "var(--tpl-heading)" }}>Thời tiết</p>
            <p className="text-xs" style={{ color: "var(--tpl-muted)" }}>
              {status === "loading" ? "Đang định vị…" : "Bật định vị để xem thời tiết"}
            </p>
          </div>
        </div>
      </Card>
    );
  }

  const { now, daily } = data;
  return (
    <Link href={"/weather" as Route} className="block transition hover:opacity-90" title="Mở trang thời tiết">
      <Card className="overflow-hidden">
        <div className="p-4" style={{ background: "linear-gradient(150deg, var(--tpl-accent), var(--tpl-blue))", color: "#fff" }}>
          <div className="flex items-start justify-between">
            <div>
              <div className="text-3xl font-bold leading-none">{now.temp}°</div>
              <div className="mt-1 text-xs text-white/80">{now.high}° / {now.low}°</div>
            </div>
            <Icon name={now.icon} size={40} />
          </div>
          <p className="mt-2 text-sm font-semibold">{now.label}</p>
          <p className="text-xs text-white/80">Cảm giác {now.feels}° · Mưa {now.rain}%</p>
        </div>
        <div className="grid grid-cols-7 gap-1 px-2 py-3">
          {daily.map((d, i) => (
            <div key={i} className="flex flex-col items-center gap-1">
              <span className="text-[10px] font-semibold" style={{ color: "var(--tpl-muted)" }}>{i === 0 ? "Nay" : d.dow}</span>
              <span style={{ color: "var(--tpl-heading)" }}><Icon name={d.icon} size={16} /></span>
              <span className="text-[11px]" style={{ color: "var(--tpl-heading)" }}>{d.high}°</span>
            </div>
          ))}
        </div>
      </Card>
    </Link>
  );
}
