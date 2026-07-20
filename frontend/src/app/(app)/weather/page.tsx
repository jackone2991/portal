import type { Metadata } from "next";
import { activeTemplate } from "@/templates/registry";

export const metadata: Metadata = { title: "Thời tiết" };

/** /weather — current + hourly + 7-day forecast (Open-Meteo, geolocation). */
export default function WeatherPage() {
  const View = activeTemplate().views.weather;
  return <View />;
}
