import type { ComponentType } from "react";
import { FinanceWidget } from "./FinanceWidget";
import { ContinueWidget } from "./ContinueWidget";
import { MusicWidget } from "./MusicWidget";
import { WeatherWidget } from "./WeatherWidget";
import { CalendarWidget } from "./CalendarWidget";
import { PagesWidget } from "./PagesWidget";
import { BirthdayCard } from "./BirthdayCard";
import { FriendSuggestions } from "./FriendSuggestions";
import { ActivityFeed } from "./ActivityFeed";

/**
 * Widget key → component.
 *
 * This is the half of the widget system that CANNOT live in the database: a row
 * in `layout_widgets` stores where a card goes and whether it shows, but the
 * card itself is code. The keys here and the seeded `key` column are one
 * contract — adding a widget means adding it in both places, and the API refuses
 * a key it has never seen rather than letting the dashboard render a hole.
 *
 * A key the bundle does not know is skipped silently at render time. That is the
 * right failure for a rolling deploy: an old bundle meeting a newly seeded
 * widget shows one card less, not a crashed dashboard.
 */
export const WIDGET_REGISTRY: Record<string, ComponentType> = {
  finance: FinanceWidget,
  continue: ContinueWidget,
  music: MusicWidget,
  weather: WeatherWidget,
  calendar: CalendarWidget,
  pages: PagesWidget,
  birthdays: BirthdayCard,
  "friend-suggestions": FriendSuggestions,
  "activity-feed": ActivityFeed,
};

export function widgetComponent(key: string): ComponentType | undefined {
  return WIDGET_REGISTRY[key];
}
