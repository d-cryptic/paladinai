import { Navbar } from "@/components/sections/Navbar";
import { Hero } from "@/components/sections/Hero";
import { TechMarquee } from "@/components/sections/TechMarquee";
import { VideoSection } from "@/components/sections/VideoSection";
import { HowItWorks } from "@/components/sections/HowItWorks";
import { Architecture } from "@/components/sections/Architecture";
import { Features } from "@/components/sections/Features";
import { TechStack } from "@/components/sections/TechStack";
import { PaladinBattle } from "@/components/sections/PaladinBattle";
import { CTA } from "@/components/sections/CTA";
import { Footer } from "@/components/sections/Footer";

const jsonLd = {
  "@context": "https://schema.org",
  "@type": "SoftwareApplication",
  name: "Paladin AI",
  url: "https://getpaladin.dev",
  description:
    "AI-powered incident response and infrastructure monitoring. Natural language queries across Prometheus, Loki, Grafana, and Alertmanager with persistent intelligent memory.",
  applicationCategory: "DeveloperApplication",
  operatingSystem: "Linux, macOS, Windows",
  offers: {
    "@type": "Offer",
    price: "0",
    priceCurrency: "USD",
    description: "Early access — join the waitlist",
  },
  featureList: [
    "Natural language Prometheus and Loki queries",
    "AI-powered alert triage and root cause analysis",
    "Persistent memory across incidents",
    "Multi-source signal correlation",
    "LangGraph multi-step reasoning",
  ],
  keywords:
    "AI monitoring, incident response, Prometheus, Loki, Grafana, Alertmanager, SRE, observability, LangGraph",
  author: {
    "@type": "Organization",
    name: "Paladin AI",
    url: "https://getpaladin.dev",
  },
};

export default function Home() {
  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
      />
      <Navbar />
      <main className="flex-1 pt-16">
        <Hero />
        <TechMarquee />
        <VideoSection />
        <HowItWorks />
        <Architecture />
        <Features />
        <TechStack />
        <CTA />
        <PaladinBattle />
      </main>
      <Footer />
    </>
  );
}
