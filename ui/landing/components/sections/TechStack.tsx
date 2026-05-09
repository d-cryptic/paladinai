"use client";

import { motion } from "framer-motion";
import { PixelBadge } from "@/components/shared/PixelBadge";
import { GlitchText } from "@/components/shared/GlitchText";

type Group = {
  title: string;
  color: string;
  textColor: string;
  items: string[];
};

const GROUPS: Group[] = [
  {
    title: "BACKEND",
    color: "#FF3131",
    textColor: "#fff",
    items: ["FastAPI", "Python 3.13", "LangGraph", "Mem0AI"],
  },
  {
    title: "FRONTEND",
    color: "#FFE600",
    textColor: "#111",
    items: ["Next.js 15", "React 19", "TypeScript", "Tailwind CSS"],
  },
  {
    title: "STORAGE",
    color: "#00FF88",
    textColor: "#111",
    items: ["MongoDB", "Neo4j", "Qdrant", "Valkey"],
  },
  {
    title: "MONITORING",
    color: "#4DAAFF",
    textColor: "#111",
    items: ["Prometheus", "Loki", "Grafana", "Alertmanager"],
  },
];

export function TechStack() {
  return (
    <section
      id="stack"
      className="relative py-20 sm:py-28 px-5 sm:px-8"
      style={{ background: "#F5F0E4" }}
    >
      <div className="max-w-6xl mx-auto">
        <div className="text-center">
          <PixelBadge label="TECH TREE" color="blue" />
          <div className="mt-6">
            <GlitchText
              as="h2"
              text="THE WEAPONS IN PALADIN'S ARSENAL"
              className="text-2xl sm:text-3xl lg:text-4xl"
            />
          </div>
          <p className="font-mono mt-5 max-w-2xl mx-auto text-[#111]/70">
            Best-in-class tools across every layer. No glue code. No surprises.
          </p>
        </div>

        <div className="mt-14 grid md:grid-cols-2 gap-7">
          {GROUPS.map((g, idx) => (
            <motion.div
              key={g.title}
              initial={{ opacity: 0, y: 24 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true, margin: "-60px" }}
              transition={{ duration: 0.45, delay: idx * 0.08 }}
              style={{
                background: "#fff",
                border: "2.5px solid #111",
                boxShadow: "6px 6px 0 #111",
              }}
            >
              <div
                className="px-5 py-3 flex items-center justify-between"
                style={{
                  background: g.color,
                  borderBottom: "2.5px solid #111",
                }}
              >
                <span
                  className="font-pixel text-sm tracking-wider"
                  style={{ color: g.textColor }}
                >
                  [ {g.title} ]
                </span>
                <span
                  className="font-pixel text-[10px]"
                  style={{ color: g.textColor }}
                >
                  ★ {g.items.length}
                </span>
              </div>

              <div className="p-5 flex flex-wrap gap-3">
                {g.items.map((item) => (
                  <PixelBadge
                    key={item}
                    label={item}
                    color="white"
                    className="!font-mono !text-[12px] !px-3 !py-1.5 !tracking-normal !normal-case"
                  />
                ))}
              </div>
            </motion.div>
          ))}
        </div>
      </div>
    </section>
  );
}

export default TechStack;
