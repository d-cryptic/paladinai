"use client";

import { useState } from "react";
import { motion } from "framer-motion";
import { Star, BookOpen } from "lucide-react";

const GITHUB_URL = "https://github.com/barundebnath/paladinai";
const DOCS_URL = "https://github.com/barundebnath/paladinai#readme";

const PIXELS = [
  { left: "8%", top: "20%", size: 14, color: "#FF3131" },
  { left: "16%", top: "70%", size: 10, color: "#00FF88" },
  { left: "85%", top: "18%", size: 18, color: "#FFE600" },
  { left: "92%", top: "75%", size: 12, color: "#A855F7" },
  { left: "50%", top: "12%", size: 8, color: "#4DAAFF" },
  { left: "30%", top: "85%", size: 12, color: "#FFE600" },
];

export function CTA() {
  const [stars, setStars] = useState(1337);

  return (
    <section
      id="cta"
      className="relative py-24 sm:py-32 px-5 sm:px-8 overflow-hidden"
      style={{ background: "#111111" }}
    >
      {/* Floating pixel chunks */}
      {PIXELS.map((p, i) => (
        <span
          key={i}
          className="absolute animate-float"
          style={{
            left: p.left,
            top: p.top,
            width: p.size,
            height: p.size,
            background: p.color,
            border: "2px solid #111",
            boxShadow: `0 0 0 2px ${p.color}`,
            animationDelay: `${i * 0.4}s`,
          }}
          aria-hidden
        />
      ))}

      {/* scanline subtle effect */}
      <div className="absolute inset-0 dot-grid opacity-10 pointer-events-none" />

      <div className="relative max-w-4xl mx-auto text-center">
        <motion.h2
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          transition={{ duration: 0.55 }}
          className="font-pixel text-2xl sm:text-4xl md:text-5xl leading-[1.3] text-white"
        >
          READY TO DEPLOY{" "}
          <span className="bg-[#FFE600] text-[#111] px-2 inline-block">
            PALADIN AI?
          </span>
        </motion.h2>

        <motion.p
          initial={{ opacity: 0 }}
          whileInView={{ opacity: 1 }}
          viewport={{ once: true }}
          transition={{ duration: 0.5, delay: 0.2 }}
          className="font-mono mt-6 text-base sm:text-lg text-white/80"
        >
          Open source. Self-hosted. Apache 2.0 + Commons Clause Licensed.
        </motion.p>

        <motion.div
          initial={{ opacity: 0, y: 14 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          transition={{ duration: 0.5, delay: 0.35 }}
          className="mt-10 flex flex-wrap items-center justify-center gap-5"
        >
          <a
            href={GITHUB_URL}
            target="_blank"
            rel="noopener noreferrer"
            onMouseEnter={() => setStars((s) => s + 1)}
            className="neu-btn neu-btn-primary"
          >
            <Star size={14} fill="#111111" />
            STAR ON GITHUB · {stars.toLocaleString()}
          </a>

          <a
            href={DOCS_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="neu-btn"
            style={{ background: "#fff", color: "#111" }}
          >
            <BookOpen size={14} />
            READ THE DOCS
          </a>
        </motion.div>

        <motion.div
          initial={{ opacity: 0 }}
          whileInView={{ opacity: 1 }}
          viewport={{ once: true }}
          transition={{ duration: 0.5, delay: 0.55 }}
          className="mt-10 font-pixel text-[10px] text-[#FFE600]/80 tracking-widest"
        >
          [ INSERT COIN TO CONTINUE ]
          <span className="animate-blink ml-1">▋</span>
        </motion.div>
      </div>
    </section>
  );
}

export default CTA;
