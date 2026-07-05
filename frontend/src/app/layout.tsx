import type { Metadata } from "next";
import "./globals.css";
// Active template theme tokens. Swap this import when changing
// NEXT_PUBLIC_TEMPLATE_VERSION (see src/templates/registry.ts).
import "@/templates/v1/theme/theme.css";
import { Providers } from "./providers";

export const metadata: Metadata = {
  title: {
    default: "Portal",
    template: "%s — Portal",
  },
  description: "Self-hosted media platform: movies, music, stories.",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
