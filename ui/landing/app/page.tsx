import { Navbar } from "@/components/sections/Navbar";
import { Hero } from "@/components/sections/Hero";
import { TechMarquee } from "@/components/sections/TechMarquee";
import { VideoSection } from "@/components/sections/VideoSection";
import { HowItWorks } from "@/components/sections/HowItWorks";
import { Architecture } from "@/components/sections/Architecture";
import { Features } from "@/components/sections/Features";
import { TechStack } from "@/components/sections/TechStack";
import { CTA } from "@/components/sections/CTA";
import { Footer } from "@/components/sections/Footer";

export default function Home() {
  return (
    <>
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
      </main>
      <Footer />
    </>
  );
}
