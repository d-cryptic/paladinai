"use client";

import { motion } from "framer-motion";
import { PixelBadge } from "@/components/shared/PixelBadge";
import { GlitchText } from "@/components/shared/GlitchText";

type Step = {
  level: string;
  title: string;
  icon: string;
  description: string;
  color: string;
};

const STEPS: Step[] = [
  {
    level: "LEVEL 01",
    title: "ASK ANYTHING",
    icon: "💬",
    description:
      "Type any question about your systems in plain English. No PromQL. No LogQL. Just talk.",
    color: "#FF3131",
  },
  {
    level: "LEVEL 02",
    title: "SCAN THE STACK",
    icon: "🔍",
    description:
      "Paladin auto-queries Prometheus metrics, Loki logs, Grafana dashboards and Alertmanager rules simultaneously.",
    color: "#FFE600",
  },
  {
    level: "LEVEL 03",
    title: "REMEMBER EVERYTHING",
    icon: "🧠",
    description:
      "Findings are stored in Neo4j memory graph + Qdrant vectors. Context persists across all sessions.",
    color: "#00FF88",
  },
  {
    level: "LEVEL 04",
    title: "GET ANSWERS",
    icon: "⚡",
    description:
      "Receive a contextual response with root cause analysis, metrics, recommendations and next steps.",
    color: "#4DAAFF",
  },
];

export function HowItWorks() {
  return (
    <section
      id="how-it-works"
      className="relative py-20 sm:py-28 px-5 sm:px-8 overflow-hidden"
      style={{ background: "#FFE600" }}
    >
      <div className="absolute inset-0 dot-grid opacity-30 pointer-events-none" />

      <div className="relative max-w-6xl mx-auto">
        <div className="text-center">
          <PixelBadge label="LEVEL SELECT" color="black" />
          <div className="mt-6">
            <GlitchText
              as="h2"
              text="FOUR STEPS TO INCIDENT MASTERY"
              className="text-2xl sm:text-3xl lg:text-4xl"
            />
          </div>
          <p className="font-mono mt-5 max-w-2xl mx-auto text-[#111]/80">
            From question to root cause in seconds. Power up through every level
            of the incident response loop.
          </p>
        </div>

        <div className="mt-14 grid md:grid-cols-2 gap-8">
          {STEPS.map((step, idx) => (
            <motion.div
              key={step.level}
              initial={{ opacity: 0, x: -32 }}
              whileInView={{ opacity: 1, x: 0 }}
              viewport={{ once: true, margin: "-60px" }}
              transition={{ duration: 0.5, delay: idx * 0.12 }}
              className="relative"
              style={{
                background: "#ffffff",
                border: "2.5px solid #111111",
                boxShadow: "6px 6px 0 #111111",
              }}
            >
              {/* color bar */}
              <div
                className="h-3"
                style={{
                  background: step.color,
                  borderBottom: "2.5px solid #111111",
                }}
              />
              <div className="p-6 sm:p-7">
                <div className="flex items-center justify-between gap-3">
                  <span className="font-pixel text-[10px] tracking-wider text-[#111]/60 uppercase">
                    {step.level}
                  </span>
                  <span
                    className="text-3xl"
                    aria-hidden
                    style={{ filter: "drop-shadow(2px 2px 0 #111)" }}
                  >
                    {step.icon}
                  </span>
                </div>
                <h3 className="font-pixel mt-4 text-base sm:text-lg leading-tight text-[#111]">
                  {step.title}
                </h3>
                <p className="font-mono mt-4 text-sm sm:text-[15px] text-[#111]/80 leading-relaxed">
                  {step.description}
                </p>
              </div>
            </motion.div>
          ))}
        </div>
      </div>
    </section>
  );
}

export default HowItWorks;
