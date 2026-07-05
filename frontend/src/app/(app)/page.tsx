import { activeTemplate } from "@/templates/registry";

/** Home "/" — authenticated newsfeed (≈ Blade route('home')). */
export default function HomePage() {
  const View = activeTemplate().views.home;
  return <View />;
}
