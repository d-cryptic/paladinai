"use client";

import type { CSSProperties } from "react";
import { motion } from "framer-motion";
import { PixelBadge } from "@/components/shared/PixelBadge";
import { GlitchText } from "@/components/shared/GlitchText";

type NodeDef = {
  id: string;
  x: number;
  y: number;
  w: number;
  h: number;
  label: string;
  fill: string;
  textColor?: string;
  fontSize?: number;
};

type EdgeDef = {
  from: string;
  to: string;
  color: string;
};

const NODES: NodeDef[] = [
  // User interfaces (top)
  { id: "web", x: 200, y: 40, w: 130, h: 50, label: "WEB UI", fill: "#4DAAFF" },
  { id: "cli", x: 385, y: 40, w: 130, h: 50, label: "CLI", fill: "#A855F7", textColor: "#fff" },
  { id: "api", x: 570, y: 40, w: 130, h: 50, label: "API", fill: "#00FF88" },

  // Core (center)
  {
    id: "core",
    x: 320,
    y: 240,
    w: 260,
    h: 90,
    label: "PALADIN AI CORE",
    fill: "#FF3131",
    textColor: "#fff",
    fontSize: 16,
  },

  // Monitoring stack (left)
  { id: "prom", x: 40, y: 180, w: 170, h: 46, label: "PROMETHEUS", fill: "#FFE600" },
  { id: "loki", x: 40, y: 246, w: 170, h: 46, label: "LOKI", fill: "#FFE600" },
  { id: "graf", x: 40, y: 312, w: 170, h: 46, label: "GRAFANA", fill: "#FFE600" },
  { id: "alm", x: 40, y: 378, w: 170, h: 46, label: "ALERTMANAGER", fill: "#FFE600" },

  // Memory stack (right)
  { id: "neo", x: 690, y: 200, w: 170, h: 46, label: "NEO4J", fill: "#A855F7", textColor: "#fff" },
  { id: "qdr", x: 690, y: 266, w: 170, h: 46, label: "QDRANT", fill: "#A855F7", textColor: "#fff" },
  { id: "mng", x: 690, y: 332, w: 170, h: 46, label: "MONGODB", fill: "#A855F7", textColor: "#fff" },

  // Bottom
  {
    id: "lg",
    x: 230,
    y: 470,
    w: 180,
    h: 50,
    label: "LANGGRAPH ENGINE",
    fill: "#00FF88",
    fontSize: 11,
  },
  { id: "mem", x: 490, y: 470, w: 180, h: 50, label: "MEM0AI", fill: "#4DAAFF" },
];

const EDGES: EdgeDef[] = [
  { from: "web", to: "core", color: "#4DAAFF" },
  { from: "cli", to: "core", color: "#A855F7" },
  { from: "api", to: "core", color: "#00FF88" },
  { from: "prom", to: "core", color: "#FFE600" },
  { from: "loki", to: "core", color: "#FFE600" },
  { from: "graf", to: "core", color: "#FFE600" },
  { from: "alm", to: "core", color: "#FFE600" },
  { from: "core", to: "neo", color: "#A855F7" },
  { from: "core", to: "qdr", color: "#A855F7" },
  { from: "core", to: "mng", color: "#A855F7" },
  { from: "core", to: "lg", color: "#00FF88" },
  { from: "core", to: "mem", color: "#4DAAFF" },
];

function nodeCenter(n: NodeDef) {
  return { cx: n.x + n.w / 2, cy: n.y + n.h / 2 };
}

export function Architecture() {
  const map = new Map(NODES.map((n) => [n.id, n]));

  return (
    <section
      id="architecture"
      className="relative py-20 sm:py-28 px-5 sm:px-8"
      style={{ background: "#F5F0E4" }}
    >
      <div className="max-w-6xl mx-auto">
        <div className="text-center">
          <PixelBadge label="BOSS STAGE" color="purple" />
          <div className="mt-6">
            <GlitchText
              as="h2"
              text="BUILT ON A BATTLE-TESTED STACK"
              className="text-2xl sm:text-3xl md:text-4xl"
            />
          </div>
          <p className="font-mono mt-5 max-w-2xl mx-auto text-[#111]/70">
            Every layer chosen for resilience. From ingestion to memory to
            reasoning — wired through a single graph-based engine.
          </p>
        </div>

        {/* Mobile scroll hint */}
        <p className="sm:hidden font-pixel text-[9px] text-[#111]/50 text-center mt-8 mb-2 tracking-wider">
          ← SCROLL TO EXPLORE →
        </p>

        <motion.div
          initial={{ opacity: 0, y: 28 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true, margin: "-80px" }}
          transition={{ duration: 0.6 }}
          className="mt-6 sm:mt-14 mx-auto"
          style={{
            border: "3px solid #111111",
            boxShadow: "10px 10px 0 #111111",
          }}
        >
          {/* Scroll container separated from the styled border box */}
          <div
            style={{
              overflowX: "auto",
              WebkitOverflowScrolling: "touch" as CSSProperties["WebkitOverflowScrolling"],
              background: "#ffffff",
              padding: "20px",
            }}
          >
          <svg
            width={900}
            height={560}
            viewBox="0 0 900 560"
            style={{ display: "block", minWidth: 900 }}
            role="img"
            aria-label="Paladin AI architecture diagram"
          >
            {/* Edges first */}
            {EDGES.map((e, i) => {
              const from = map.get(e.from);
              const to = map.get(e.to);
              if (!from || !to) return null;
              const a = nodeCenter(from);
              const b = nodeCenter(to);
              return (
                <g key={`edge-${i}`}>
                  <line
                    x1={a.cx}
                    y1={a.cy}
                    x2={b.cx}
                    y2={b.cy}
                    stroke="#111111"
                    strokeWidth={2.5}
                  />
                  <line
                    x1={a.cx}
                    y1={a.cy}
                    x2={b.cx}
                    y2={b.cy}
                    stroke={e.color}
                    strokeWidth={2.5}
                    strokeDasharray="8 8"
                    className="flow-path"
                    style={{ animationDelay: `${i * 0.1}s` }}
                  />
                  {/* Pulse dot */}
                  <circle r="5" fill={e.color} stroke="#111" strokeWidth="2">
                    <animateMotion
                      dur={`${2 + (i % 3)}s`}
                      repeatCount="indefinite"
                      begin={`${i * 0.15}s`}
                      path={`M ${a.cx} ${a.cy} L ${b.cx} ${b.cy}`}
                    />
                  </circle>
                </g>
              );
            })}

            {/* Nodes */}
            {NODES.map((n) => {
              const isCore = n.id === "core";
              return (
                <g key={n.id}>
                  {/* shadow */}
                  <rect
                    x={n.x + 4}
                    y={n.y + 4}
                    width={n.w}
                    height={n.h}
                    fill="#111111"
                  />
                  <rect
                    x={n.x}
                    y={n.y}
                    width={n.w}
                    height={n.h}
                    fill={n.fill}
                    stroke="#111111"
                    strokeWidth={2.5}
                  />
                  <text
                    x={n.x + n.w / 2}
                    y={n.y + n.h / 2 + (n.fontSize ? n.fontSize / 3 : 4)}
                    textAnchor="middle"
                    fontFamily='"Press Start 2P", monospace'
                    fontSize={n.fontSize ?? (isCore ? 14 : 10)}
                    fill={n.textColor ?? "#111111"}
                  >
                    {n.label}
                  </text>
                </g>
              );
            })}

            {/* Group labels */}
            <text
              x={125}
              y={160}
              textAnchor="middle"
              fontFamily='"Press Start 2P", monospace'
              fontSize="9"
              fill="#111"
            >
              [ MONITORING ]
            </text>
            <text
              x={775}
              y={180}
              textAnchor="middle"
              fontFamily='"Press Start 2P", monospace'
              fontSize="9"
              fill="#111"
            >
              [ MEMORY ]
            </text>
            <text
              x={450}
              y={20}
              textAnchor="middle"
              fontFamily='"Press Start 2P", monospace'
              fontSize="9"
              fill="#111"
            >
              [ INTERFACES ]
            </text>
            <text
              x={450}
              y={550}
              textAnchor="middle"
              fontFamily='"Press Start 2P", monospace'
              fontSize="9"
              fill="#111"
            >
              [ ENGINE ]
            </text>
          </svg>
          </div>
        </motion.div>
      </div>
    </section>
  );
}

export default Architecture;
