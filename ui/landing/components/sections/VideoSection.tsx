"use client";

import { motion } from "framer-motion";
import { PixelBadge } from "@/components/shared/PixelBadge";
import { GlitchText } from "@/components/shared/GlitchText";

const CORNER_PIXELS = [
  { pos: { top: -10, left: -10 }, size: 20, color: "#FFE600" },
  { pos: { top: -10, right: -10 }, size: 14, color: "#FF3131" },
  { pos: { bottom: -10, left: -10 }, size: 14, color: "#A855F7" },
  { pos: { bottom: -10, right: -10 }, size: 20, color: "#00FF88" },
];

export function VideoSection() {
  return (
    <section
      id="demo"
      className="relative py-20 sm:py-28 px-5 sm:px-8"
      style={{ background: "#F5F0E4" }}
    >
      {/* subtle dot grid */}
      <div className="absolute inset-0 dot-grid opacity-20 pointer-events-none" />

      <div className="relative max-w-6xl mx-auto text-center">
        <motion.div
          initial={{ opacity: 0, y: 10 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          transition={{ duration: 0.4 }}
        >
          <PixelBadge label="LIVE DEMO" color="red" />
        </motion.div>

        <motion.div
          initial={{ opacity: 0, y: 14 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          transition={{ duration: 0.45, delay: 0.1 }}
          className="mt-6"
        >
          <GlitchText
            as="h2"
            text="SEE IT IN ACTION"
            className="text-2xl sm:text-4xl md:text-5xl"
          />
        </motion.div>

        <motion.p
          initial={{ opacity: 0 }}
          whileInView={{ opacity: 1 }}
          viewport={{ once: true }}
          transition={{ duration: 0.4, delay: 0.2 }}
          className="font-mono mt-5 max-w-2xl mx-auto text-[#111]/70"
        >
          Watch Paladin investigate a real incident — natural language in,
          root cause out.
        </motion.p>

        {/* Video frame */}
        <motion.div
          initial={{ opacity: 0, y: 36 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true, margin: "-80px" }}
          transition={{ duration: 0.6, delay: 0.15 }}
          className="relative mt-12 mx-auto"
          style={{ maxWidth: 1050 }}
        >
          {/* Corner pixel accents */}
          {CORNER_PIXELS.map((p, i) => (
            <span
              key={i}
              className="absolute z-10 hidden sm:block"
              style={{
                ...p.pos,
                width: p.size,
                height: p.size,
                background: p.color,
                border: "2.5px solid #111111",
              }}
              aria-hidden
            />
          ))}

          {/* Outer glow/shadow layer */}
          <div
            style={{
              position: "absolute",
              inset: 0,
              transform: "translate(10px, 10px)",
              background: "#111111",
              zIndex: 0,
            }}
            aria-hidden
          />

          {/* Main frame */}
          <div
            className="relative z-[1]"
            style={{
              border: "3px solid #111111",
              background: "#111111",
            }}
          >
            {/* Title bar */}
            <div
              className="flex items-center gap-2 px-4 py-2.5"
              style={{
                background: "#111111",
                borderBottom: "3px solid #FFE600",
              }}
            >
              {/* Traffic lights */}
              <span
                className="w-3.5 h-3.5 inline-block"
                style={{ background: "#FF3131", border: "2px solid #FF6060" }}
              />
              <span
                className="w-3.5 h-3.5 inline-block"
                style={{ background: "#FFE600", border: "2px solid #FFD000" }}
              />
              <span
                className="w-3.5 h-3.5 inline-block"
                style={{ background: "#00FF88", border: "2px solid #00CC6A" }}
              />

              <span className="flex-1 text-center font-pixel text-[9px] text-[#FFE600] uppercase tracking-widest">
                paladin.ai — demo
              </span>

              <PixelBadge label="v0.1.0" color="yellow" className="!text-[8px] !py-1 !px-2" />
            </div>

            {/* Loom iframe — 16:9 */}
            <div style={{ position: "relative", paddingBottom: "56.25%", height: 0 }}>
              <iframe
                src="https://www.loom.com/embed/8bdf923be32a429e83afcd0e19d842e8"
                frameBorder="0"
                allowFullScreen
                title="Paladin AI demo"
                style={{
                  position: "absolute",
                  top: 0,
                  left: 0,
                  width: "100%",
                  height: "100%",
                  display: "block",
                }}
              />
            </div>

            {/* Bottom bar */}
            <div
              className="flex items-center justify-between px-4 py-2"
              style={{ borderTop: "3px solid #FFE600", background: "#111111" }}
            >
              <span className="font-pixel text-[9px] text-[#FFE600]/70 tracking-wider">
                ◆ INCIDENT RESPONSE DEMO
              </span>
              <span className="font-pixel text-[9px] text-[#00FF88] tracking-wider">
                ● REC
              </span>
            </div>
          </div>
        </motion.div>
      </div>
    </section>
  );
}

export default VideoSection;
