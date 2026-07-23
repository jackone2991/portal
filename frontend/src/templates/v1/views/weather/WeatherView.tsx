"use client";

import { Icon } from "../../components/ui/Icon";
import { useGeoWeather } from "@/lib/weather";

/**
 * Weather page (`/weather`) — current conditions + 24h + 7-day, from Open-Meteo
 * for the browser's geolocation (see lib/weather). Free/keyless; degrades to a
 * "bật định vị" prompt when location is unavailable.
 */
export function WeatherView() {
  const { status, data } = useGeoWeather();

  if (status === "loading") {
    return <p style={{ color: "var(--tpl-muted)" }}>Đang lấy thời tiết…</p>;
  }
  if (status !== "ok" || !data) {
    return (
      <div className="max-w-md rounded-2xl border p-6 shadow-sm" style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}>
        <div className="flex items-center gap-3">
          <span style={{ color: "var(--tpl-muted)" }}><Icon name="weather-partly-sunny-icon" size={28} /></span>
          <div>
            <h1 className="text-base font-bold" style={{ color: "var(--tpl-heading)" }}>Thời tiết</h1>
            <p className="text-sm" style={{ color: "var(--tpl-muted)" }}>Hãy bật quyền định vị trong trình duyệt để xem thời tiết theo vị trí của bạn.</p>
          </div>
        </div>
      </div>
    );
  }

  const { now, hourly, daily, place } = data;

  return (
    <div className="max-w-3xl space-y-5">
      {/* Hiện tại */}
      <div className="overflow-hidden rounded-2xl border shadow-sm" style={{ borderColor: "var(--tpl-border)" }}>
        <div className="p-6 text-white" style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-blue))" }}>
          <p className="text-xs text-white/80">{place ? `Thời tiết · ${place}` : "Thời tiết tại vị trí của bạn"}</p>
          <div className="mt-1 flex items-start justify-between">
            <div>
              <div className="text-5xl font-bold leading-none">{now.temp}°</div>
              <p className="mt-2 text-lg font-semibold">{now.label}</p>
              <p className="text-sm text-white/85">Cảm giác như {now.feels}° · {now.high}° / {now.low}°</p>
            </div>
            <Icon name={now.icon} size={72} />
          </div>
        </div>
        <div className="grid grid-cols-3" style={{ background: "var(--tpl-surface)" }}>
          <Metric label="Độ ẩm" value={`${now.humidity}%`} />
          <Metric label="Gió" value={`${now.wind} km/h`} border />
          <Metric label="Khả năng mưa" value={`${now.rain}%`} border />
        </div>
      </div>

      {/* Theo giờ */}
      <div className="rounded-2xl border p-4 shadow-sm" style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}>
        <h2 className="mb-3 text-sm font-bold" style={{ color: "var(--tpl-heading)" }}>Theo giờ</h2>
        <div className="flex gap-5 overflow-x-auto pb-1">
          {hourly.map((h, i) => (
            <div key={i} className="flex min-w-[2.75rem] flex-col items-center gap-1.5">
              <span className="text-xs" style={{ color: "var(--tpl-muted)" }}>{h.time}</span>
              <span style={{ color: "var(--tpl-heading)" }}><Icon name={h.icon} size={22} /></span>
              <span className="text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>{h.temp}°</span>
            </div>
          ))}
        </div>
      </div>

      {/* 7 ngày tới */}
      <div className="rounded-2xl border p-4 shadow-sm" style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}>
        <h2 className="mb-1 text-sm font-bold" style={{ color: "var(--tpl-heading)" }}>7 ngày tới</h2>
        <ul>
          {daily.map((d, i) => (
            <li key={i} className="flex items-center gap-3 border-t py-2.5 first:border-t-0" style={{ borderColor: "var(--tpl-border)" }}>
              <span className="w-16 text-sm font-medium" style={{ color: "var(--tpl-heading)" }}>{d.dow}</span>
              <span style={{ color: "var(--tpl-heading)" }}><Icon name={d.icon} size={22} /></span>
              <span className="flex-1 truncate text-sm" style={{ color: "var(--tpl-muted)" }}>{d.label}</span>
              <span className="text-xs" style={{ color: "var(--tpl-accent)" }}>💧 {d.rain}%</span>
              <span className="w-20 text-right text-sm" style={{ color: "var(--tpl-heading)" }}>
                <b>{d.high}°</b> <span style={{ color: "var(--tpl-muted)" }}>{d.low}°</span>
              </span>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}

function Metric({ label, value, border }: { label: string; value: string; border?: boolean }) {
  return (
    <div className="px-4 py-3 text-center" style={border ? { borderLeft: "1px solid var(--tpl-border)" } : undefined}>
      <p className="text-xs" style={{ color: "var(--tpl-muted)" }}>{label}</p>
      <p className="text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>{value}</p>
    </div>
  );
}
