"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { motion } from "framer-motion";
import { Star, ArrowRight } from "lucide-react";
import { PixelBadge } from "@/components/shared/PixelBadge";

const GITHUB_URL = "https://github.com/d-cryptic/paladinai";
const DOCS_URL   = "https://github.com/d-cryptic/paladinai#readme";

const TERMINAL_LINES: Array<{ text: string; color?: string; pauseBefore: number }> = [
  { text: '> paladin ask "why is API latency spiking?"', color: "#FFE600", pauseBefore: 600  },
  { text: "",                                                                 pauseBefore: 400  },
  { text: "◆ Analyzing Prometheus metrics...",          color: "#4DAAFF",   pauseBefore: 100  },
  { text: "◆ Querying Loki logs...",                    color: "#4DAAFF",   pauseBefore: 700  },
  { text: "◆ Cross-referencing memory...",              color: "#A855F7",   pauseBefore: 600  },
  { text: "",                                                                 pauseBefore: 350  },
  { text: "✓ Found root cause: DB connection pool exhausted", color: "#00FF88", pauseBefore: 900 },
  { text: "  - p99 latency: 2.3s (threshold: 500ms)",  color: "#F5F0E4",   pauseBefore: 200  },
  { text: "  - Active connections: 98/100",             color: "#F5F0E4",   pauseBefore: 150  },
  { text: "  - Recommendation: Scale connection pool",  color: "#FFE600",   pauseBefore: 150  },
];

const CHAR_SPEED = 38; // ms per character

interface DisplayLine {
  text: string;
  color?: string;
  typing: boolean;
}

function useTypewriter(lines: typeof TERMINAL_LINES) {
  const [displayed, setDisplayed] = useState<DisplayLine[]>([]);
  const cancelRef = useRef(false);

  useEffect(() => {
    cancelRef.current = false;

    // Simple sleep — just setTimeout. Cancellation is checked at each await point.
    const sleep = (ms: number) => new Promise<void>((res) => setTimeout(res, ms));

    async function run() {
      setDisplayed([]);
      await sleep(400);

      for (let i = 0; i < lines.length; i++) {
        if (cancelRef.current) return;
        const line = lines[i];

        await sleep(line.pauseBefore);
        if (cancelRef.current) return;

        setDisplayed((prev) => [...prev, { text: "", color: line.color, typing: true }]);

        for (let c = 1; c <= line.text.length; c++) {
          if (cancelRef.current) return;
          const partial = line.text.slice(0, c);
          setDisplayed((prev) => {
            const next = [...prev];
            next[i] = { text: partial, color: line.color, typing: true };
            return next;
          });
          await sleep(line.text[c - 1] === " " ? CHAR_SPEED * 0.6 : CHAR_SPEED);
        }

        if (cancelRef.current) return;
        setDisplayed((prev) => {
          const next = [...prev];
          if (next[i]) next[i] = { ...next[i], typing: false };
          return next;
        });
      }

      await sleep(3200);
      if (!cancelRef.current) run();
    }

    run();
    return () => { cancelRef.current = true; };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  return displayed;
}

const FLOATERS = [
  { left: "6%",  top: "18%", size: 18, color: "#FF3131", delay: 0   },
  { left: "12%", top: "70%", size: 14, color: "#FFE600", delay: 0.5 },
  { left: "84%", top: "12%", size: 16, color: "#00FF88", delay: 1   },
  { left: "92%", top: "62%", size: 22, color: "#A855F7", delay: 1.5 },
  { left: "47%", top: "8%",  size: 12, color: "#4DAAFF", delay: 0.8 },
];

export function Hero() {
  const displayed = useTypewriter(TERMINAL_LINES);
  const typingIdx = useMemo(() => displayed.findIndex((l) => l.typing), [displayed]);
  const allDone   = useMemo(() => displayed.length > 0 && displayed.every((l) => !l.typing), [displayed]);

  return (
    <section
      id="hero"
      className="relative pt-28 pb-20 sm:pt-32 sm:pb-24 px-5 sm:px-8 overflow-hidden"
      style={{ background: "#F5F0E4" }}
    >
      {/* dotted bg */}
      <div className="absolute inset-0 dot-grid opacity-25 pointer-events-none" />

      {/* floating pixels */}
      {FLOATERS.map((f, i) => (
        <motion.div
          key={i}
          className="absolute animate-float pointer-events-none hidden lg:block"
          style={{
            left: f.left, top: f.top,
            width: f.size, height: f.size,
            background: f.color,
            border: "2.5px solid #111111",
            animationDelay: `${f.delay}s`,
          }}
          aria-hidden
        />
      ))}

      <div className="relative max-w-7xl mx-auto grid lg:grid-cols-2 gap-12 items-center">
        {/* ── Left col ── */}
        <div>
          <motion.div initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.4 }}>
            <PixelBadge label="v0.1.0 — OPEN SOURCE" color="green" />
          </motion.div>

          <motion.h1
            initial={{ opacity: 0, y: 18 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.55, delay: 0.1 }}
            className="font-pixel mt-6 text-[24px] sm:text-[34px] lg:text-[42px] leading-[1.25] text-[#111111]"
          >
            <span className="glitch-text inline-block">THE AI THAT</span>
            <br />
            <span className="text-[#FF3131] inline-block">WATCHES YOUR</span>
            <br />
            <span className="bg-[#FFE600] inline-block px-2 -mx-2">SYSTEMS</span>
          </motion.h1>

          <motion.p
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ duration: 0.5, delay: 0.4 }}
            className="font-mono mt-6 text-base lg:text-lg text-[#111111]/80 max-w-xl"
          >
            Natural language incident response. Intelligent memory. Real-time
            monitoring analysis — all in one open-source platform.
          </motion.p>

          <motion.div
            initial={{ opacity: 0, y: 12 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.5, delay: 0.55 }}
            className="mt-8 flex flex-wrap gap-4"
          >
            <a href={GITHUB_URL} target="_blank" rel="noopener noreferrer" className="neu-btn neu-btn-primary">
              <Star size={14} fill="#111111" />
              STAR ON GITHUB
            </a>
            <a href={DOCS_URL} target="_blank" rel="noopener noreferrer" className="neu-btn neu-btn-secondary">
              EXPLORE DOCS
              <ArrowRight size={14} />
            </a>
          </motion.div>

          {/* Stats row */}
          <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.5, delay: 0.75 }}
            className="mt-8 flex flex-wrap gap-3"
          >
            {[
              { value: "3",    label: "INTERFACES",  color: "#FF3131" },
              { value: "4+",   label: "INTEGRATIONS",color: "#FFE600" },
              { value: "∞",    label: "MEMORY",      color: "#00FF88" },
              { value: "100%", label: "OPEN SOURCE", color: "#4DAAFF" },
            ].map((s) => (
              <div
                key={s.label}
                className="flex flex-col items-center px-4 py-3 min-w-[80px]"
                style={{ background: s.color, border: "2.5px solid #111", boxShadow: "3px 3px 0 #111" }}
              >
                <span className="font-pixel text-lg text-[#111]">{s.value}</span>
                <span className="font-pixel text-[8px] text-[#111]/70 mt-1 tracking-wider">{s.label}</span>
              </div>
            ))}
          </motion.div>

          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ duration: 0.5, delay: 0.95 }}
            className="mt-5 flex flex-wrap items-center gap-3 font-pixel text-[10px] uppercase tracking-wider"
          >
            <span className="text-[#111111]/50">[ BUILT WITH ]</span>
            <span className="text-[#FF3131]">LangGraph</span>
            <span className="text-[#111111]/30">/</span>
            <span className="text-[#4DAAFF]">FastAPI</span>
            <span className="text-[#111111]/30">/</span>
            <span className="text-[#A855F7]">Neo4j</span>
            <span className="text-[#111111]/30">/</span>
            <span className="text-[#FF7A00]">Qdrant</span>
          </motion.div>
        </div>

        {/* ── Right col — Typewriter Terminal ── */}
        <motion.div
          initial={{ opacity: 0, scale: 0.95, y: 18 }}
          animate={{ opacity: 1, scale: 1, y: 0 }}
          transition={{ duration: 0.6, delay: 0.3 }}
          className="relative"
        >
          <div
            className="scanline"
            style={{
              background: "#0A0A0A",
              border: "2.5px solid #111111",
              boxShadow: "8px 8px 0 #111111",
            }}
          >
            {/* Title bar */}
            <div
              className="flex items-center gap-2 px-3 py-2"
              style={{ background: "#FFE600", borderBottom: "2.5px solid #111111" }}
            >
              <span className="w-3 h-3 bg-[#FF3131] border-2 border-[#111111]" />
              <span className="w-3 h-3 bg-[#FFE600] border-2 border-[#111111]" />
              <span className="w-3 h-3 bg-[#00FF88] border-2 border-[#111111]" />
              <span className="ml-3 font-pixel text-[9px] text-[#111111] uppercase tracking-wider">
                paladin@core ~ %
              </span>
            </div>

            {/* Terminal body */}
            <div className="px-4 py-5 font-mono text-[12px] sm:text-[13px] leading-relaxed min-h-[330px]">
              {displayed.map((line, i) => {
                const isCurrentlyTyping = line.typing;
                return (
                  <div
                    key={i}
                    style={{ color: line.color ?? "#F5F0E4", minHeight: "1.4em" }}
                  >
                    {line.text || " "}
                    {isCurrentlyTyping && (
                      <span className="animate-blink" style={{ color: "#FFE600" }}>▋</span>
                    )}
                  </div>
                );
              })}

              {/* terminal cursor after all lines done */}
              {(allDone || displayed.length === 0) && (
                <span
                  className="inline-block animate-blink mt-1"
                  style={{ color: "#00FF88" }}
                >
                  ▋
                </span>
              )}
            </div>
          </div>

          {/* decorative pixel chunks */}
          <div
            className="absolute -bottom-4 -right-4 w-10 h-10 hidden sm:block"
            style={{ background: "#FF3131", border: "2.5px solid #111111" }}
            aria-hidden
          />
          <div
            className="absolute -top-3 -left-3 w-6 h-6 hidden sm:block"
            style={{ background: "#00FF88", border: "2.5px solid #111111" }}
            aria-hidden
          />
        </motion.div>
      </div>
    </section>
  );
}

export default Hero;
