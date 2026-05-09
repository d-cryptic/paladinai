"use client";

import { motion } from "framer-motion";
import {
  MessageSquare,
  FileText,
  Brain,
  Terminal,
  Clock,
  Activity,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { PixelBadge, type BadgeColor } from "@/components/shared/PixelBadge";
import { GlitchText } from "@/components/shared/GlitchText";

type Feature = {
  title: string;
  description: string;
  icon: LucideIcon;
  color: BadgeColor;
  bg: string;
  status: string;
};

const FEATURES: Feature[] = [
  {
    title: "NATURAL LANGUAGE",
    description:
      "Ask about your systems in plain English. No query language needed.",
    icon: MessageSquare,
    color: "blue",
    bg: "#4DAAFF",
    status: "ACTIVE",
  },
  {
    title: "RAG DOCUMENTS",
    description:
      "Upload PDFs and Markdown. Paladin learns from your runbooks automatically.",
    icon: FileText,
    color: "green",
    bg: "#00FF88",
    status: "ENABLED",
  },
  {
    title: "INTELLIGENT MEMORY",
    description:
      "Neo4j graph + Qdrant vectors remember every insight across sessions.",
    icon: Brain,
    color: "purple",
    bg: "#A855F7",
    status: "PERSISTENT",
  },
  {
    title: "MULTI-INTERFACE",
    description:
      "Web UI, Python CLI, or REST API. Access Paladin from anywhere.",
    icon: Terminal,
    color: "yellow",
    bg: "#FFE600",
    status: "3 MODES",
  },
  {
    title: "SESSION PERSISTENCE",
    description:
      "MongoDB checkpoints preserve full conversation context. Resume anytime.",
    icon: Clock,
    color: "red",
    bg: "#FF3131",
    status: "ALWAYS ON",
  },
  {
    title: "MONITORING NATIVE",
    description:
      "Direct integration with Prometheus, Loki, Grafana, and Alertmanager.",
    icon: Activity,
    color: "blue",
    bg: "#4DAAFF",
    status: "CONNECTED",
  },
];

export function Features() {
  return (
    <section
      id="features"
      className="relative py-20 sm:py-28 px-5 sm:px-8"
      style={{ background: "#ffffff" }}
    >
      <div className="max-w-6xl mx-auto">
        <div className="text-center">
          <PixelBadge label="POWER-UPS UNLOCKED" color="green" />
          <div className="mt-6">
            <GlitchText
              as="h2"
              text="EVERYTHING YOU NEED TO FIGHT INCIDENTS"
              className="text-2xl sm:text-3xl lg:text-4xl"
            />
          </div>
          <p className="font-mono mt-5 max-w-2xl mx-auto text-[#111]/70">
            Six battle-ready capabilities. Wired together into one operator
            sidekick that actually understands your stack.
          </p>
        </div>

        <div className="mt-14 grid md:grid-cols-2 lg:grid-cols-3 gap-7">
          {FEATURES.map((f, idx) => {
            const Icon = f.icon;
            return (
              <motion.div
                key={f.title}
                initial={{ opacity: 0, y: 28 }}
                whileInView={{ opacity: 1, y: 0 }}
                viewport={{ once: true, margin: "-50px" }}
                transition={{ duration: 0.45, delay: (idx % 3) * 0.1 }}
                whileHover={{ x: 2, y: 2, boxShadow: "3px 3px 0 #111" }}
                className="p-6 flex flex-col"
                style={{
                  background: "#fff",
                  border: "2.5px solid #111",
                  boxShadow: "5px 5px 0 #111",
                }}
              >
                <div
                  className="w-14 h-14 flex items-center justify-center"
                  style={{
                    background: f.bg,
                    border: "2.5px solid #111",
                    boxShadow: "3px 3px 0 #111",
                  }}
                  aria-hidden
                >
                  <Icon size={26} strokeWidth={2.5} color="#111111" />
                </div>

                <h3 className="font-pixel mt-6 text-sm sm:text-base leading-tight text-[#111]">
                  {f.title}
                </h3>
                <p className="font-mono mt-3 text-sm text-[#111]/75 leading-relaxed flex-1">
                  {f.description}
                </p>

                <div className="mt-6">
                  <PixelBadge label={f.status} color={f.color} />
                </div>
              </motion.div>
            );
          })}
        </div>
      </div>
    </section>
  );
}

export default Features;
