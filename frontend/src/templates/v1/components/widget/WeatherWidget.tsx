import { Icon } from "../ui/Icon";

/**
 * Weather rail card — extracted from the inline `WeatherWidget` in
 * `views/home/HomeView.tsx` (Olympus "Weather Widget"). A gradient card showing
 * the current temperature + condition, a 7-day icon strip, and location/date.
 * All props are optional and default to the original Olympus sample data.
 */

export type WeatherDay = {
  /** Short day label, e.g. "SUN". */
  day: string;
  /** Olympus sprite icon name for the day's condition. */
  icon: string;
  /** Temperature (degrees), rendered with a trailing `°`. */
  temp: number;
};

const DEFAULT_WEEK: WeatherDay[] = [
  { day: "SUN", icon: "weather-sunny-icon", temp: 60 },
  { day: "MON", icon: "weather-sunny-icon", temp: 58 },
  { day: "TUE", icon: "weather-cloudy-icon", temp: 67 },
  { day: "WED", icon: "weather-rain-icon", temp: 70 },
  { day: "THU", icon: "weather-rain-icon", temp: 58 },
  { day: "FRI", icon: "weather-rain-icon", temp: 68 },
  { day: "SAT", icon: "weather-partly-sunny-icon", temp: 65 },
];

export function WeatherWidget({
  temp = 64,
  low = 58,
  high = 76,
  condition = "Partly Sunny",
  realFeel = 67,
  chanceOfRain = 49,
  icon = "weather-partly-sunny-icon",
  week = DEFAULT_WEEK,
  date = "Saturday, March 26th",
  location = "San Francisco, CA",
}: {
  temp?: number;
  low?: number;
  high?: number;
  condition?: string;
  realFeel?: number;
  chanceOfRain?: number;
  icon?: string;
  week?: WeatherDay[];
  date?: string;
  location?: string;
} = {}) {
  return (
    <div
      className="overflow-hidden rounded-xl text-white shadow-sm"
      style={{ background: "linear-gradient(160deg, var(--tpl-weather-1), var(--tpl-weather-2))" }}
    >
      <div className="p-5">
        <div className="flex items-start justify-between">
          <div className="flex items-start gap-1">
            <span className="text-5xl font-light leading-none">{temp}°</span>
            <span className="mt-1 text-xs leading-tight text-white/80">
              {low}°
              <br />
              {high}°
            </span>
          </div>
          <Icon name={icon} size={40} />
        </div>
        <p className="mt-3 text-lg font-semibold">{condition}</p>
        <p className="text-xs text-white/80">
          Real Feel: {realFeel}°&nbsp;&nbsp;&nbsp;Chance of Rain: {chanceOfRain}%
        </p>
      </div>

      <div
        className="grid grid-cols-7 gap-1 px-3 py-4"
        style={{ background: "rgba(255,255,255,0.08)" }}
      >
        {week.map((d) => (
          <div
            key={d.day}
            className="flex flex-col items-center gap-1.5 text-[10px] text-white/85"
          >
            <span className="font-semibold">{d.day}</span>
            <Icon name={d.icon} size={18} />
            <span>{d.temp}°</span>
          </div>
        ))}
      </div>

      <div className="px-5 py-3 text-center text-xs">
        <p className="font-semibold">{date}</p>
        <p className="text-white/75">{location}</p>
      </div>
    </div>
  );
}
