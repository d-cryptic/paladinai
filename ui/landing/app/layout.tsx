import type { Metadata } from "next";
import { Press_Start_2P, Space_Mono } from "next/font/google";
import "./globals.css";

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

export const metadata: Metadata = {
  title: "Paladin AI — The AI That Watches Your Systems",
  description:
    "AI-powered incident response and monitoring. Natural language queries across Prometheus, Loki, Grafana, and Alertmanager with persistent intelligent memory.",
  keywords: ["AI monitoring", "incident response", "Prometheus", "LangGraph", "observability"],
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
        {children}
      </body>
    </html>
  );
}
