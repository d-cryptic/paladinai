import type { Metadata } from "next";
import { Press_Start_2P, Space_Mono } from "next/font/google";
import "./globals.css";
import { ReactScanInit } from "@/components/dev/ReactScanInit";
import { PostHogProvider } from "@/components/providers/PostHogProvider";

const pressStart = Press_Start_2P({
  variable: "--font-pixel",
  weight: "400",
  subsets: ["latin"],
  display: "swap",
});

const spaceMono = Space_Mono({
  variable: "--font-mono",
  weight: ["400", "700"],
  subsets: ["latin"],
  display: "swap",
});

const siteUrl = "https://getpaladin.dev";

export const metadata: Metadata = {
  metadataBase: new URL(siteUrl),
  title: {
    default: "Paladin AI — The AI That Watches Your Systems",
    template: "%s | Paladin AI",
  },
  description:
    "AI-powered incident response and infrastructure monitoring. Natural language queries across Prometheus, Loki, Grafana, and Alertmanager with persistent intelligent memory.",
  keywords: [
    "AI monitoring",
    "incident response",
    "Prometheus",
    "Loki",
    "Grafana",
    "LangGraph",
    "observability",
    "SRE",
    "DevOps",
    "on-call",
    "infrastructure monitoring",
    "AI ops",
  ],
  authors: [{ name: "Paladin AI" }],
  creator: "Paladin AI",
  publisher: "Paladin AI",
  robots: {
    index: true,
    follow: true,
    googleBot: {
      index: true,
      follow: true,
      "max-image-preview": "large",
      "max-snippet": -1,
    },
  },
  alternates: {
    canonical: siteUrl,
  },
  openGraph: {
    type: "website",
    url: siteUrl,
    siteName: "Paladin AI",
    title: "Paladin AI — The AI That Watches Your Systems",
    description:
      "AI-powered incident response and infrastructure monitoring. Natural language queries across Prometheus, Loki, Grafana, and Alertmanager with persistent intelligent memory.",
    images: [
      {
        url: "/og-image.svg",
        width: 1200,
        height: 630,
        alt: "Paladin AI — The AI That Watches Your Systems",
      },
    ],
    locale: "en_US",
  },
  twitter: {
    card: "summary_large_image",
    title: "Paladin AI — The AI That Watches Your Systems",
    description:
      "AI-powered incident response and infrastructure monitoring. Natural language queries across Prometheus, Loki, Grafana, and Alertmanager.",
    images: ["/og-image.svg"],
    creator: "@paladinai",
  },
  icons: {
    icon: [
      { url: "/icon.svg", type: "image/svg+xml" },
    ],
    shortcut: "/icon.svg",
    apple: "/icon.svg",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang="en"
      className={`${pressStart.variable} ${spaceMono.variable} h-full antialiased`}
    >
      <body className="min-h-full flex flex-col bg-[#F5F0E4] text-[#111111]" style={{ fontFamily: "var(--font-mono), 'Space Mono', monospace" }}>
        <ReactScanInit />
        <PostHogProvider>
          {children}
        </PostHogProvider>
      </body>
    </html>
  );
}
