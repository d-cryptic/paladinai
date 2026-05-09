"use client";

import { motion } from "framer-motion";
import { PixelBadge } from "@/components/shared/PixelBadge";
import { GlitchText } from "@/components/shared/GlitchText";

export function VideoSection() {
  return (
    <section
      id="demo"
      className="relative py-20 sm:py-28 px-5 sm:px-8"
      style={{ background: "#F5F0E4" }}
    >
      <div className="max-w-6xl mx-auto text-center">
        <PixelBadge label="LIVE DEMO" color="red" />
        <div className="mt-6">
          <GlitchText
            as="h2"
            text="SEE IT IN ACTION"
            className="text-2xl sm:text-4xl md:text-5xl"
          />
        </div>
        <p className="font-mono mt-5 max-w-2xl mx-auto text-[#111]/70">
          Watch how Paladin investigates incidents, surfaces root causes, and
          remembers context — all in plain English.
        </p>

        {/* TODO: Replace this div with <iframe src="YOUR_LOOM_URL" allow="autoplay; fullscreen" allowFullScreen /> */}
        <motion.div
          initial={{ opacity: 0, y: 30 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true, margin: "-100px" }}
          transition={{ duration: 0.55 }}
          className="relative mt-10 mx-auto"
          style={{
            border: "3px solid #111111",
            boxShadow: "12px 12px 0 #111111",
            background: "#0A0A0A",
            aspectRatio: "16 / 9",
            maxWidth: "1100px",
          }}
        >
          {/* Title bar */}
          <div
            className="flex items-center gap-2 px-3 py-2"
            style={{ background: "#FF3131", borderBottom: "3px solid #111111" }}
          >
            <span className="w-3 h-3 bg-[#FFE600] border-2 border-[#111111]" />
            <span className="w-3 h-3 bg-[#00FF88] border-2 border-[#111111]" />
            <span className="w-3 h-3 bg-[#4DAAFF] border-2 border-[#111111]" />
            <span className="ml-3 font-pixel text-[9px] text-white uppercase tracking-wider">
              paladin.demo.mp4
            </span>
          </div>

          {/* Body */}
          <div
            className="absolute flex flex-col items-center justify-center gap-6"
            style={{ inset: 0, top: 40 }}
          >
            {/* Play button */}
            <button
              type="button"
              aria-label="Play demo video"
              style={{
                width: 88,
                height: 88,
                background: "#FFE600",
                border: "3px solid #111111",
                boxShadow: "6px 6px 0 #111111",
                position: "relative",
                cursor: "pointer",
              }}
            >
              <span
                style={{
                  position: "absolute",
                  top: "50%",
                  left: "56%",
                  transform: "translate(-50%, -50%)",
                  width: 0,
                  height: 0,
                  borderLeft: "26px solid #111111",
                  borderTop: "18px solid transparent",
                  borderBottom: "18px solid transparent",
                }}
                aria-hidden
              />
            </button>

            <div style={{ textAlign: "center" }}>
              <div className="font-pixel text-white text-sm uppercase tracking-wider">
                DEMO VIDEO
              </div>
              <div className="font-mono mt-2 text-xs" style={{ color: "#FFE600" }}>
                [ YOUR LOOM LINK GOES HERE ]
              </div>
            </div>

            {/* Fake progress bar */}
            <div
              style={{
                width: "min(340px, 80%)",
                border: "2px solid #333",
                background: "#1a1a1a",
                height: 12,
                marginTop: 8,
              }}
            >
              <div
                style={{
                  width: "38%",
                  height: "100%",
                  background: "#FF3131",
                  borderRight: "2px solid #FF6060",
                }}
              />
            </div>
            <div className="font-pixel text-[9px] flex gap-6" style={{ color: "#555" }}>
              <span>00:42</span>
              <span style={{ color: "#333" }}>────────────────</span>
              <span>01:52</span>
            </div>
          </div>

          {/* Corner pixels */}
          <span
            className="absolute -bottom-3 -right-3 w-7 h-7"
            style={{ background: "#00FF88", border: "3px solid #111111" }}
            aria-hidden
          />
          <span
            className="absolute -top-3 -left-3 w-5 h-5"
            style={{ background: "#FFE600", border: "3px solid #111111" }}
            aria-hidden
          />
        </motion.div>
      </div>
    </section>
  );
}

export default VideoSection;
